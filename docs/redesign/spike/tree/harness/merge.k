"""
Engine harness — the ~part written ONCE, reused for every Application.

KEY FINDING (revises spike README finding #1): KCL's `|` does NOT last-wins-merge
two yaml.decode'd dicts. `{a=1} | {a=2}` from decoded files raises
"conflicting values on attribute 'a'" — the `=` override only works inside dict
LITERALS, not on runtime-decoded data. So composing real env-data FILES needs a
hand-written deep-merge. That merge is here (engine), not on the user surface.

Semantics reproduced from ytt --data-values-inspect (verified against baseline.sh):
  - dict + dict  -> deep merge, last (deeper/leaf) wins per key
  - list + list  -> APPEND (ytt data-values array default), de-dup handled by caller
  - scalar       -> last wins
"""

import file
import yaml

# last-wins recursive deep merge; lists append (ytt data-values semantics).
deepMerge = lambda a: any, b: any -> any {
    _isd = lambda x -> bool { typeof(x) == "dict" }
    _isl = lambda x -> bool { typeof(x) == "list" }
    result = b
    if _isd(a) and _isd(b):
        result = {
            k: (deepMerge(a[k], b[k]) if k in a else b[k]) for k in b
        } | {k: a[k] for k in a if k not in b}
    elif _isl(a) and _isl(b):
        result = a + b
    result
}

# read + decode + fold an ordered list of file paths (host-enumerated, see discover.sh).
fold = lambda paths: [str] -> {str:any} {
    _go = lambda ps: [str], acc: {str:any} -> {str:any} {
        r = acc
        if len(ps) > 0:
            r = _go(ps[1:], deepMerge(acc, yaml.decode(file.read(ps[0]))))
        r
    }
    _go(paths, {})
}

# remove app entries whose proto is in `protos` — KCL's translation of the
# ytt `#@overlay/match by="proto"` + `#@overlay/remove` that runs DURING composition.
# Unlike ytt, KCL self-references the accumulated list, so this is a plain filter.
removeApps = lambda apps: [{str:any}], protos: [str] -> [{str:any}] {
    [a for a in apps if a.proto not in protos]
}
