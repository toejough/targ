// Package internal provides file operation utilities.
package internal

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
)

// Exported variables.
var (
	ErrEmptyDest       = errors.New("dest cannot be empty")
	ErrNoInputPatterns = errors.New("no input patterns provided")
	MkdirAll           = os.MkdirAll
	//nolint:gosec // G304: Opening user-specified files is the function's purpose.
	OpenFile  = func(name string) (io.ReadCloser, error) { return os.Open(name) }
	ReadFile  = os.ReadFile
	WriteFile = os.WriteFile
)

// Checksum reports whether the content hash of inputs differs from the stored hash at dest.
// When the hash changes, the new hash is written to dest.
func Checksum(
	inputs []string,
	dest string,
	matchFn func([]string) ([]string, error),
) (bool, error) {
	if len(inputs) == 0 {
		return false, ErrNoInputPatterns
	}

	if dest == "" {
		return false, ErrEmptyDest
	}

	matches, err := matchFn(inputs)
	if err != nil {
		return false, err
	}

	nextHash, err := computeChecksum(matches)
	if err != nil {
		return false, err
	}

	prevHash, err := readChecksum(dest)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
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
		// hash.Hash.Write never returns an error per Go documentation
		_, _ = io.WriteString(hasher, path)
		_, _ = io.WriteString(hasher, "\x00")

		file, err := OpenFile(path)
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
	data, err := ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("reading checksum file: %w", err)
	}

	return string(data), nil
}

func writeChecksum(path, sum string) error {
	dir := filepath.Dir(path)
	if dir != "." {
		//nolint:mnd // standard cache directory permissions
		err := MkdirAll(dir, 0o755)
		if err != nil {
			return fmt.Errorf("creating checksum directory: %w", err)
		}
	}

	//nolint:mnd // standard cache file permissions
	err := WriteFile(path, []byte(sum), 0o644)
	if err != nil {
		return fmt.Errorf("writing checksum file: %w", err)
	}

	return nil
}
