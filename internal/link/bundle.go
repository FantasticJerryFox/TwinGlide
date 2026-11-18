package link

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

func verifyBundle(root string, manifest []byte) error {
	var files map[string]string
	if err := json.Unmarshal(manifest, &files); err != nil {
		return err
	}
	if len(files) == 0 {
		return errors.New("empty engine manifest")
	}
	for name, want := range files {
		if filepath.IsAbs(name) || strings.Contains(name, "..") || strings.Contains(name, ":") {
			return errors.New("invalid engine manifest path")
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			return err
		}
		h := sha256.Sum256(raw)
		if hex.EncodeToString(h[:]) != want {
			return errors.New("engine checksum mismatch: " + name)
		}
	}
	return nil
}
