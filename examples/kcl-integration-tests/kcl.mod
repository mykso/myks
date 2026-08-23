[package]
name = "config"
version = "0.1.0"

# In-repo example: the myks schema package is consumed by path so the example
# always tracks the engine at HEAD. User repos pin the published package instead:
#   kcl mod add oci://ghcr.io/mykso/myks
[dependencies]
myks = { path = "../../kcl/myks" }
