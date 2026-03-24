// Package config provides configuration loading for FoxRay.
// Copyright 2025 Tomohiro Owada
// SPDX-License-Identifier: Apache-2.0
package config

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

const (
	foxrayDirName    = ".foxray"
	legacyGMNDirName = ".gmn"
)

// FoxRayDir returns the path to ~/.foxray.
func FoxRayDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, foxrayDirName), nil
}

func legacyGMNDir() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, legacyGMNDirName), nil
}

// LoadDotEnv loads .env files in priority order (lowest first):
//  1. ~/.foxray/.env (user-level defaults)
//  2. ~/.gmn/.env    (legacy compatibility)
//  3. ./.env         (project-level overrides)
//
// Existing environment variables are never overwritten.
func LoadDotEnv() {
	// 1. User-level: ~/.foxray/.env
	if gmnDir, err := FoxRayDir(); err == nil {
		loadEnvFile(filepath.Join(gmnDir, ".env"))
	}

	// 2. Legacy compatibility: ~/.gmn/.env
	if gmnDir, err := legacyGMNDir(); err == nil {
		loadEnvFile(filepath.Join(gmnDir, ".env"))
	}

	// 3. Project-level: ./.env
	if cwd, err := os.Getwd(); err == nil {
		loadEnvFile(filepath.Join(cwd, ".env"))
	}
}

func loadEnvFile(path string) {
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		key, val, ok := strings.Cut(line, "=")
		if !ok {
			continue
		}

		key = strings.TrimSpace(key)
		val = strings.TrimSpace(val)

		// Strip surrounding quotes
		if len(val) >= 2 {
			if (val[0] == '"' && val[len(val)-1] == '"') ||
				(val[0] == '\'' && val[len(val)-1] == '\'') {
				val = val[1 : len(val)-1]
			}
		}

		// Never overwrite existing env vars
		if os.Getenv(key) == "" {
			os.Setenv(key, val)
		}
	}
}
