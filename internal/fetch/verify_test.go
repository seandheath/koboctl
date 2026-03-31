package fetch_test

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/seandheath/koboctl/internal/fetch"
)

func TestParseSHA256SumsFile(t *testing.T) {
	input := `# comment line
a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2  file1.zip
DEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEFDEADBEEF  *file2.tgz

`
	got := fetch.ParseSHA256SumsFile(input)

	if got["file1.zip"] != "a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2" {
		t.Errorf("file1.zip hash mismatch: %q", got["file1.zip"])
	}
	// Binary mode indicator (*) should be stripped from filename.
	if _, ok := got["file2.tgz"]; !ok {
		t.Error("file2.tgz (with * prefix) should be in result")
	}
	// Hash should be lowercased.
	if got["file2.tgz"] != "deadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeefdeadbeef" {
		t.Errorf("file2.tgz hash not lowercased: %q", got["file2.tgz"])
	}
	// Comment lines should be ignored.
	if len(got) != 2 {
		t.Errorf("expected 2 entries, got %d: %v", len(got), got)
	}
}

func TestVerifySHA256_Correct(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	content := []byte("koboctl test content")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	// Compute expected hash in the test itself.
	sum := sha256.Sum256(content)
	expected := hex.EncodeToString(sum[:])

	if err := fetch.VerifySHA256(path, expected); err != nil {
		t.Errorf("VerifySHA256 with correct hash: %v", err)
	}
}

func TestVerifySHA256_Mismatch(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}
	wrongHash := "0000000000000000000000000000000000000000000000000000000000000000"
	if err := fetch.VerifySHA256(path, wrongHash); err == nil {
		t.Error("expected mismatch error for wrong hash")
	}
}

func TestVerifySHA256_CaseInsensitive(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.bin")
	content := []byte("case test")
	if err := os.WriteFile(path, content, 0o644); err != nil {
		t.Fatal(err)
	}
	sum := sha256.Sum256(content)
	upper := hex.EncodeToString(sum[:])
	// Convert to uppercase to test case-insensitive comparison.
	for i, c := range upper {
		if c >= 'a' && c <= 'f' {
			upper = upper[:i] + string(rune(c-32)) + upper[i+1:]
		}
	}
	// VerifySHA256 uses strings.EqualFold so uppercase should work.
	if err := fetch.VerifySHA256(path, upper); err != nil {
		t.Errorf("VerifySHA256 failed with uppercase hash: %v", err)
	}
}
