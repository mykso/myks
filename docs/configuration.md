# Configuration Reference

Myks uses a `.myks.yaml` configuration file to control global behavior and
project settings. This file is automatically searched for in the current
directory and all parent directories.

## Configuration File Location

Myks searches for configuration files in the following order:

1. File specified with `--config` flag
2. `.myks.yaml` in the current working directory
3. `.myks.yaml` in any parent directory up to the filesystem root

## Configuration Options

### `async`

- **Type**: `integer`
- **Default**: `0` (unlimited)
- **Description**: Sets the number of applications to be processed in parallel.
  A value of `0` means no limit.
- **Environment Variable**: `MYKS_ASYNC`
- **Command Line Flag**: `--async`, `-a`

```yaml
async: 4
```

### `config-in-root`

- **Type**: `boolean`
- **Default**: `false`
- **Description**: When set to `true`, automatically sets the root directory to
  the location of the configuration file. This allows myks to be run from any
  subdirectory within the project.
- **Environment Variable**: `MYKS_CONFIG_IN_ROOT`

```yaml
config-in-root: true
```

### `log-level`

- **Type**: `string`
- **Default**: `info`
- **Valid Values**: `trace`, `debug`, `info`, `warn`, `error`, `fatal`, `panic`
- **Description**: Sets the logging level. See
  [zerolog documentation](https://github.com/rs/zerolog#leveled-logging) for
  details.
- **Environment Variable**: `MYKS_LOG_LEVEL`
- **Command Line Flag**: `--log-level`, `-l`

```yaml
log-level: debug
```

### `min-version`

- **Type**: `string`
- **Default**: none
- **Description**: Specifies the minimum version of myks required to run this
  configuration. Myks will print an error message if the current version is
  lower than this requirement.
- **Environment Variable**: `MYKS_MIN_VERSION`

```yaml
min-version: 'v4.0.0'
```

### `plugin-sources`

- **Type**: `array` of `string`
- **Default**: `["./plugins"]`
- **Description**: List of directories to search for myks plugins. All binaries
  in these directories will be loaded as plugins.
- **Environment Variable**: `MYKS_PLUGIN_SOURCES` (comma-separated)

```yaml
plugin-sources:
  - ./plugins
  - ./custom-tools
  - /opt/myks-plugins
```

### `root-dir`

- **Type**: `string`
- **Default**: `"."` (current directory)
- **Description**: Sets the root directory for the myks project. Usually set
  automatically when `config-in-root` is enabled.
- **Environment Variable**: `MYKS_ROOT_DIR`

```yaml
root-dir: '/path/to/project'
```

## Environment Variables

All configuration options can be overridden using environment variables. The
variable names follow the pattern `MYKS_<OPTION_NAME>` where option names are
converted to uppercase and hyphens are replaced with underscores.

Examples:

- `MYKS_ASYNC=4`
- `MYKS_LOG_LEVEL=debug`
- `MYKS_CONFIG_IN_ROOT=true`
- `MYKS_MIN_VERSION=v4.0.0`

## Smart Mode Configuration

Smart Mode behavior can be controlled through command-line flags:

### `--smart-mode.base-revision`

- **Type**: `string`
- **Default**: none (uses local changes only)
- **Description**: Base revision to compare against for change detection.
- **Environment Variable**: `MYKS_SMART_MODE_BASE_REVISION`

```shell
myks render --smart-mode.base-revision=main
myks render --smart-mode.base-revision=HEAD~1
```

### `--smart-mode.only-print`

- **Type**: `boolean`
- **Default**: `false`
- **Description**: Only print the list of environments and applications that
  would be rendered without actually processing them.
- **Environment Variable**: `MYKS_SMART_MODE_ONLY_PRINT`

```shell
myks render --smart-mode.only-print
```

## Example Configuration

Here's a complete example of a `.myks.yaml` configuration file:

```yaml
# Process 4 applications in parallel
async: 4

# Allow running myks from subdirectories
config-in-root: true

# Enable debug logging
log-level: debug

# Require minimum myks version
min-version: 'v4.0.0'

# Additional plugin directories
plugin-sources:
  - ./plugins
  - ./scripts
```

## Legacy Configuration

Previous versions of myks may have used different configuration formats or
locations. The current version maintains backward compatibility where possible,
but it's recommended to migrate to the current format for the best experience.
