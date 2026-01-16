// Package file provides utilities for file operations in build scripts.
package file

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Exported variables.
var (
	ErrEmptyDest       = errors.New("dest cannot be empty")
	ErrNoInputPatterns = errors.New("no input patterns provided")
)

// Checksum reports whether the content hash of inputs differs from the stored hash at dest.
// When the hash changes, the new hash is written to dest.
func Checksum(inputs []string, dest string) (bool, error) {
	if len(inputs) == 0 {
		return false, ErrNoInputPatterns
	}

	if dest == "" {
		return false, ErrEmptyDest
	}

	matches, err := Match(inputs...)
	if err != nil {
		return false, err
	}

	nextHash, err := computeChecksum(matches)
	if err != nil {
		return false, err
	}

	prevHash, err := readChecksum(dest)
	if err != nil && !os.IsNotExist(err) {
		return false, err
	}

	if prevHash == nextHash {
		return false, nil
	}

	err = writeChecksum(dest, nextHash)
	if err != nil {
		return false, err
	}

	return true, nil
}

func computeChecksum(paths []string) (string, error) {
	hasher := sha256.New()
	for _, path := range paths {
		_, err := io.WriteString(hasher, path)
		if err != nil {
			return "", fmt.Errorf("writing path to hasher: %w", err)
		}

		_, err = io.WriteString(hasher, "\x00")
		if err != nil {
			return "", fmt.Errorf("writing separator to hasher: %w", err)
		}

		file, err := os.Open(
			path,
		)
		if err != nil {
			return "", fmt.Errorf("opening %s: %w", path, err)
		}

		_, err = io.Copy(hasher, file)
		if err != nil {
			_ = file.Close()

			return "", fmt.Errorf("reading %s: %w", path, err)
		}

		err = file.Close()
		if err != nil {
			return "", fmt.Errorf("closing %s: %w", path, err)
		}
	}

	return hex.EncodeToString(hasher.Sum(nil)), nil
}

func readChecksum(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	return string(data), nil
}

func writeChecksum(path, sum string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		err := os.MkdirAll(dir, 0o755)
		if err != nil {
			return err
		}
	}

	return os.WriteFile(path, []byte(sum), 0o644)
}
