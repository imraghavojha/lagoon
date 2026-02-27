package nix

// shellNixTemplate is the nix expression we write to the cache dir.
// bash and coreutils are always included — without them the sandbox has no shell or /usr/bin/env.
// {{COMMIT}} and {{SHA256}} are the pinned nixpkgs values from lagoon.toml.
// {{PACKAGES}} is the user's package list, one name per line, indented.
const shellNixTemplate = `{ pkgs ? import (fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/{{COMMIT}}.tar.gz";
    sha256 = "{{SHA256}}";
  }) {}
}:

pkgs.mkShell {
  buildInputs = with pkgs; [
    bash
    coreutils
    {{PACKAGES}}
  ];
}
`
