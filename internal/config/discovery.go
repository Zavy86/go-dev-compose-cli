package config

import (
	"fmt"
	"os"
	"path/filepath"
)

var defaultComposeFiles = []string{
	"compose.yaml",
	"compose.yml",
	"docker-compose.yaml",
	"docker-compose.yml",
}

func FindComposeFile(targetDir string) (string, error) {
	if targetDir == "" {
		pwd, err := os.Getwd()
		if err != nil {
			return "", err
		}
		targetDir = pwd
	}

	for _, filename := range defaultComposeFiles {
		fullPath := filepath.Join(targetDir, filename)
		if _, err := os.Stat(fullPath); err == nil {
			return fullPath, nil
		}
	}

	return "", fmt.Errorf("no compose file found in %s", targetDir)
}
