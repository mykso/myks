# Running embedded tools in-process

Investigation of whether myks can call its embedded ytt, kbld and vendir as Go
libraries instead of re-executing its own binary (`myks ytt ...`) for every
call. Recorded so the topic can be picked up again without repeating the
research. Code references are for myks 5.14.0 (`842aa94`), the `mykso/ytt` fork
at `a4e2475e5969`, kbld v0.49.1 and vendir v0.46.2.

## Outcome

Not implemented. The cost that motivated the question came from UPX: the
Linux and Windows release binaries were UPX-packed (LZMA), so every
self-invocation decompressed the whole binary in user space. UPX was removed
from the release build instead. In-process calls remain feasible for all three
tools, but the remaining gain is small compared to the work and the behavior
changes listed below.

## Measurements

Per-exec user CPU on a 4 vCPU Linux VM, release 5.14.0 (54.6 MB unpacked,
14.6 MB packed):

| Command              | UPX-packed | Unpacked |
| -------------------- | ---------- | -------- |
| `myks --help`        | 986 ms     | 24 ms    |
| `myks ytt` (render)  | 1135 ms    | 166 ms   |

A full render of a large project (680 applications, about 11,100 tool calls
summed over all steps of the `Tool Resource Metrics Summary`) used 181 min user
CPU, about 1 s per call. Except for a handful of `git` and the external `helm`
calls, every call was a self-invocation, so most of that time was UPX
decompression.

Estimate (not measured on a full render): without UPX, a self-invocation costs
about 24 ms of startup over the tool's own work. In-process calls would save
that floor, roughly 4 min user CPU on the render above, against about 165 min
saved by dropping UPX.

Dropping UPX grows the Linux and Windows archives by about 7 MB each (estimate:
5.14.0 `linux_amd64.tar.gz` is 13 MB packed, `darwin_amd64.tar.gz` is 20 MB
unpacked).

## Call sites

All self-invocations go through two functions in `internal/myks/process.go`:

- `runYttWithFilesAndStdin` builds `myks ytt --file=... [args]` for every ytt
  call (`Application.ytt`/`yttS`, `Environment.yttS`, `renderDataYaml`,
  `mergeValuesYaml`, `render_ytt_pkg.go`, `vendir_secrets.go`, `globe.go`,
  `migrate.go`).
- `runCmd` with `myksFullPath()` runs kbld (`render_kbld.go`, render and
  `--unresolved-inspect`) and vendir (`sync_vendir.go`, `vendir sync`).

`cmd/embedded` dispatches `os.Args[1]` to `yttMain`, `kbldMain` and
`vendirMain`, which run the upstream cobra commands.

## Why not just run the cobra commands in-process

Each tool writes to `os.Stdout`/`os.Stderr` directly and reads `os.Stdin` for
`-`. These are process globals, so concurrent calls under `--async` cannot be
redirected per call. Each tool needs an adapter on its library API instead.

## Per-tool feasibility

| Tool   | Verdict              | Library entry point |
| ------ | -------------------- | ------------------- |
| ytt    | feasible             | `template.NewOptions()` + `Options.RunWithFiles(Input, ui.UI)` |
| kbld   | feasible             | `cmd.ResolveOptions` + `ResolveResources(logger, prefixWriter)` |
| vendir | feasible, workaround | `cmd.SyncOptions.Run()` |

### ytt

- `ui.NewCustomWriterTTY(debug, stdout, stderr)` captures output;
  `NewRegularFilesSource(opts, ui).Output(out)` produces the same stdout as the
  CLI. Flags can be parsed into `Options` with `BindFlags` on a fresh
  `pflag.FlagSet`.
- The stdin callers (`plugin_argocd.go`, `vendir_secrets.go`) pass embedded
  templates. They can be passed as
  `files.NewFileFromSource(files.NewBytesSource("stdin.yml", bs))`, which
  keeps the name `StdinSource` uses and sidesteps the `hasStdinBeenRead`
  package global in `pkg/files/stdin.go`.
- Blocker: `template.NewCompiledTemplate` assigns the starlark
  `resolve.Allow*` package globals on every compile. Concurrent renders race on
  them, and `go test -race` would flag it. Fix in the `mykso/ytt` fork by
  setting them once (`init` or `sync.Once`).

### kbld

- `ResolveOptions` fields are exported. `ResolveOptions.Run` hardcodes
  `ctllog.NewLogger(os.Stderr)`, so call `ResolveResources` with a logger on a
  buffer and print the result the way `Run` does.
- `ui.NewWriterUI(stdout, stderr, logger)` from go-cli-ui captures the
  `--unresolved-inspect` output.
- No mutable package globals and no stdin use found.

### vendir

- myks passes rendered secrets with `--file=-`; vendir reads `-` from
  `os.Stdin` in `config/resources.go`. In-process, the secrets need a 0600
  temporary file or a fork patch that accepts bytes.
- `SyncOptions.Run` calls `os.Chdir` only with `--chdir`, which myks does not
  pass. Other settings come from `VENDIR_*` environment variables.

## Behavior changes of in-process calls

- `TrackCmdMetric` reads user/system CPU and max RSS from `cmd.ProcessState`.
  In-process calls would report only count and wall time.
- Errors become Go errors instead of `exit status 1`; the stderr text that
  callers log (`ytt: Error: ...`, built with `uierrs.NewMultiLineError`) has
  to be rebuilt.
- A panic inside a tool kills myks unless the adapter recovers it.
- `kbldMain` and `vendirMain` call `log.SetOutput(io.Discard)`; in-process that
  would apply to all of myks.
- Tests would no longer need a `myks` binary on `PATH` for embedded tools.

## When to revisit

If profiling an unpacked build still shows process startup as a significant
share of render time, start with ytt (the bulk of the calls), then kbld.
Acceptance: identical `rendered/` output against the self-invoking build,
`go test -race` and golangci-lint pass, CPU time of a full render measured
before and after.
