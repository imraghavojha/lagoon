package config

import (
	"os"

	"github.com/BurntSushi/toml"
)

const (
	Filename = "lagoon.toml"

	// nixpkgs-unstable pin — fetched 2026-02-27, verified sha256 in vm
	// to update: nix-prefetch-url --unpack https://github.com/NixOS/nixpkgs/archive/<new-commit>.tar.gz
	DefaultCommit = "26eaeac4e409d7b5a6bf6f90a2a2dc223c78d915"
	DefaultSHA256 = "1knl8dcr5ip70a2vbky3q844212crwrvybyw2nhfmgm1mvqry963"
)

// Config holds everything from lagoon.toml
type Config struct {
	Packages      []string `toml:"packages"`
	NixpkgsCommit string   `toml:"nixpkgs_commit"`
	NixpkgsSHA256 string   `toml:"nixpkgs_sha256"`
	Profile       string   `toml:"profile"`   // "minimal" or "network"
	OnEnter       string   `toml:"on_enter,omitempty"` // command to run on sandbox entry
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
