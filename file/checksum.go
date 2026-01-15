package file

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

// Checksum reports whether the content hash of inputs differs from the stored hash at dest.
// When the hash changes, the new hash is written to dest.
func Checksum(inputs []string, dest string) (bool, error) {
	if len(inputs) == 0 {
		return false, fmt.Errorf("no input patterns provided")
	}
	if dest == "" {
		return false, fmt.Errorf("dest cannot be empty")
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

	if err := writeChecksum(dest, nextHash); err != nil {
		return false, err
	}
	return true, nil
}

func computeChecksum(paths []string) (string, error) {
	hasher := sha256.New()
	for _, path := range paths {
		if _, err := io.WriteString(hasher, path); err != nil {
			return "", err
		}
		if _, err := io.WriteString(hasher, "\x00"); err != nil {
			return "", err
		}
		file, err := os.Open(path)
		if err != nil {
			return "", err
		}
		if _, err := io.Copy(hasher, file); err != nil {
			_ = file.Close()
			return "", err
		}
		if err := file.Close(); err != nil {
			return "", err
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
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}
	return os.WriteFile(path, []byte(sum), 0o644)
}
