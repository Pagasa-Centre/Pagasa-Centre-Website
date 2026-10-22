package testhelper

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

// AssertGoldenJSON compares v to testdata/<name>.json using json.NewEncoder (HTML escape, trailing newline),
// matching pkg/commonlibrary/render.Json.
func AssertGoldenJSON(t *testing.T, name string, v any) {
	t.Helper()
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	if err := enc.Encode(v); err != nil {
		t.Fatalf("encode golden %s: %v", name, err)
	}
	assertGoldenBytes(t, name, buf.Bytes())
}

// AssertGoldenMarshal compares v to testdata/<name>.json using json.Marshal (no trailing newline).
func AssertGoldenMarshal(t *testing.T, name string, v any) {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal golden %s: %v", name, err)
	}
	assertGoldenBytes(t, name, b)
}

// AssertGolden compares raw bytes to testdata/<name> (e.g. CSV exports).
func AssertGolden(t *testing.T, name string, got []byte) {
	t.Helper()
	assertGoldenBytes(t, name, got)
}

func assertGoldenBytes(t *testing.T, name string, got []byte) {
	t.Helper()
	path := goldenPath(t, name)
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("mkdir testdata: %v", err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatalf("write golden %s: %v", path, err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden %s: %v (run with UPDATE_GOLDEN=1 to create)", path, err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("golden %s mismatch\n--- got ---\n%s--- want ---\n%s", name, got, want)
	}
}

func goldenPath(t *testing.T, name string) string {
	t.Helper()
	for skip := 2; skip < 8; skip++ {
		_, file, _, ok := runtime.Caller(skip)
		if !ok {
			break
		}
		if filepath.Base(file) == "golden.go" {
			continue
		}
		return filepath.Join(filepath.Dir(file), "testdata", name)
	}
	t.Fatal("goldenPath: could not find calling test file")
	return ""
}
