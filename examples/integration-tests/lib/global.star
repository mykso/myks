globalVar = "defined in global"

# Covers the derivation converter: a single-expression library function becomes a KCL lambda,
# and the data values calling it keep calling it.
def prefixed(name):
    return "myks-{}".format(name)
end
