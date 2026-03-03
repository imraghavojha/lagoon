package nix

// shellNixTemplate is the nix expression we write to the cache dir.
// bash and coreutils are always included — without them the sandbox has no shell or /usr/bin/env.
// {{COMMIT}} and {{SHA256}} are the pinned nixpkgs values from lagoon.toml.
// {{PACKAGES}} is the user's package list, one name per line, indented.
// dockerNixTemplate builds a layered Docker image from the environment's packages.
// {{NAME}} is the image name (e.g. "lagoon-myapp"), {{PACKAGES}} are 4-space-indented.
const dockerNixTemplate = `{ pkgs ? import (fetchTarball {
    url = "https://github.com/NixOS/nixpkgs/archive/{{COMMIT}}.tar.gz";
    sha256 = "{{SHA256}}";
  }) {} }:

pkgs.dockerTools.buildLayeredImage {
  name = "{{NAME}}";
  tag = "latest";
  contents = with pkgs; [
    bash
    coreutils
{{PACKAGES}}
  ];
  config.Cmd = [ "${pkgs.bash}/bin/bash" ];
}
`

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
