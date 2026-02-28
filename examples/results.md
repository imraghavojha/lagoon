# real-repo test results

tested on: ubuntu 22.04 aarch64 (lima vm, qemu)
lagoon commit: test-real-repos branch

## python (python311)

```
python 3.11.14
json: ok
http.server: ok
all checks passed
```

result: PASS
packages resolve from binary cache (~10 deps, <30s on arm64)

## node (nodejs_20)

```
node v20.20.0
os.platform: linux
path.join: /workspace/hello.js
crypto: ok
all checks passed
```

result: PASS
packages resolve from binary cache (~30 deps incl icu4c, openssl)
note: nodejs_20 resolves nodejs-20.20.0 — good binary cache coverage on aarch64

## ruby (ruby)

```
ruby 3.4.8
json: ok
net/http: ok
all checks passed
```

result: PASS
packages resolve quickly (~2 deps, fastest of the three)

## summary

| stack     | package     | version  | deps | result |
|-----------|-------------|----------|------|--------|
| python    | python311   | 3.11.14  | ~10  | PASS   |
| node      | nodejs_20   | 20.20.0  | ~30  | PASS   |
| ruby      | ruby        | 3.4.8    | ~2   | PASS   |

all stdlib modules work inside the sandbox.
no internet access needed inside — all packages come from nix binary cache.
network is off by default (profile = "minimal").
