// Package filehash computes content identities for file-based evidence.
package filehash

import (
	"crypto/sha256"
	"fmt"
	"os"
)

// SHA256 returns the lowercase hexadecimal SHA-256 identity of a file.
func SHA256(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum), nil
}
