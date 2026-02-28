import sys
import json
import urllib.request

# verify version
assert sys.version_info >= (3, 11), f"need python 3.11+, got {sys.version}"
print(f"python {sys.version_info.major}.{sys.version_info.minor}.{sys.version_info.micro}")

# stdlib json round-trip
data = {"status": "ok", "sandbox": True}
assert json.loads(json.dumps(data)) == data
print("json: ok")

# stdlib http server sanity — just import, don't bind a port
import http.server
print("http.server: ok")

print("all checks passed")
