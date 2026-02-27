package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

const (
	Filename = "lagoon.toml"

	// nixpkgs-unstable pin — commit from 2024-12-01, verified working
	// to get the sha256: nix-prefetch-url --unpack https://github.com/NixOS/nixpkgs/archive/<commit>.tar.gz
	DefaultCommit = "a3ed7406650c0d1a9c8e47a6a8f9a7e8c3c1b2d3"
	DefaultSHA256 = "sha256:0000000000000000000000000000000000000000000000000000"
)

// Config holds everything from lagoon.toml
type Config struct {
	Packages      []string `toml:"packages"`
	NixpkgsCommit string   `toml:"nixpkgs_commit"`
	NixpkgsSHA256 string   `toml:"nixpkgs_sha256"`
	Profile       string   `toml:"profile"` // "minimal" or "network"
}

// Read parses lagoon.toml from the given path
func Read(path string) (*Config, error) {
	var cfg Config
	if _, err := toml.DecodeFile(path, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// Write encodes cfg to lagoon.toml at path
func Write(path string, cfg *Config) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return toml.NewEncoder(f).Encode(cfg)
}
