package fetch

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"strings"
)

// VerifySHA256 checks that the file at filePath has the expected SHA-256 digest.
// expectedHex must be a lowercase hex-encoded digest (64 characters).
func VerifySHA256(filePath, expectedHex string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("opening file for verification: %w", err)
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("hashing %q: %w", filePath, err)
	}

	got := hex.EncodeToString(h.Sum(nil))
	if !strings.EqualFold(got, expectedHex) {
		return fmt.Errorf("SHA256 mismatch for %q: got %s, want %s", filePath, got, expectedHex)
	}
	return nil
}

// ParseSHA256SumsFile parses a standard SHA256SUMS file (as produced by sha256sum).
// Each line has the format:
//
//	<hexdigest>  <filename>
//
// (two spaces between hash and filename — strings.Fields handles this correctly)
//
// Returns a map of filename → lowercase hex digest.
func ParseSHA256SumsFile(content string) map[string]string {
	result := make(map[string]string)
	for _, line := range strings.Split(content, "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		// Use strings.Fields to handle one or two spaces between hash and filename.
		fields := strings.Fields(line)
		if len(fields) < 2 {
			continue
		}
		hash := strings.ToLower(fields[0])
		// Some SHA256SUMS files prefix the filename with "*" (binary mode indicator).
		filename := strings.TrimPrefix(fields[1], "*")
		result[filename] = hash
	}
	return result
}
