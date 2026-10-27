package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/mmunki/envdrift/internal/fingerprint"
)

func TestVersion(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"version"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "envdrift") {
		t.Errorf("version output = %q", out.String())
	}
}

func TestNoArgs(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run(nil, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "Usage:") {
		t.Errorf("usage was not printed: %q", errOut.String())
	}
}

func TestUnknownCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"nope"}, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "unknown command") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestCaptureAndDiff(t *testing.T) {
	dir := t.TempDir()
	leftPath := filepath.Join(dir, "left.json")
	rightPath := filepath.Join(dir, "right.json")

	var out, errOut bytes.Buffer
	if code := run([]string{"capture", "--out", leftPath, "--quiet"}, &out, &errOut); code != 0 {
		t.Fatalf("capture exit code = %d, stderr = %s", code, errOut.String())
	}
	data, err := os.ReadFile(leftPath)
	if err != nil {
		t.Fatalf("read capture: %v", err)
	}
	var fp fingerprint.Fingerprint
	if err := json.Unmarshal(data, &fp); err != nil {
		t.Fatalf("parse capture: %v", err)
	}
	fp.Host.OS = "plan9"
	fp.Host.Arch = "mips"
	data, err = json.Marshal(fp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if err := os.WriteFile(rightPath, data, 0o644); err != nil {
		t.Fatalf("write right: %v", err)
	}

	out.Reset()
	errOut.Reset()
	code := run([]string{"diff", leftPath, rightPath, "--format", "json", "--fail-on", "critical"}, &out, &errOut)
	if code != 1 {
		t.Fatalf("diff exit code = %d, want 1, stderr = %s", code, errOut.String())
	}
	var result struct {
		Summary struct {
			Critical int `json:"critical"`
		} `json:"summary"`
	}
	if err := json.Unmarshal(out.Bytes(), &result); err != nil {
		t.Fatalf("parse diff: %v", err)
	}
	if result.Summary.Critical == 0 {
		t.Error("diff reported no critical differences")
	}
}

func TestDiffFailOnNever(t *testing.T) {
	dir := t.TempDir()
	left := fingerprint.Fingerprint{SchemaVersion: fingerprint.SchemaVersion, Host: fingerprint.Host{OS: "linux", Arch: "amd64"}}
	right := fingerprint.Fingerprint{SchemaVersion: fingerprint.SchemaVersion, Host: fingerprint.Host{OS: "darwin", Arch: "arm64"}}
	leftPath := filepath.Join(dir, "left.json")
	rightPath := filepath.Join(dir, "right.json")
	writeJSON(t, leftPath, left)
	writeJSON(t, rightPath, right)

	var out, errOut bytes.Buffer
	if code := run([]string{"diff", leftPath, rightPath, "--fail-on", "never"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0, stderr = %s", code, errOut.String())
	}
	if !strings.Contains(out.String(), "CRITICAL") {
		t.Errorf("text report has no critical section:\n%s", out.String())
	}
}

func TestDiffRejectsBadSchema(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"host":{"os":"linux"}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	var out, errOut bytes.Buffer
	if code := run([]string{"diff", path, path}, &out, &errOut); code != 2 {
		t.Fatalf("exit code = %d, want 2", code)
	}
	if !strings.Contains(errOut.String(), "schema_version") {
		t.Errorf("stderr = %q", errOut.String())
	}
}

func TestSnippet(t *testing.T) {
	var out, errOut bytes.Buffer
	if code := run([]string{"snippet"}, &out, &errOut); code != 0 {
		t.Fatalf("exit code = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "upload-artifact") {
		t.Errorf("snippet = %q", out.String())
	}
}

func writeJSON(t *testing.T, path string, value any) {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
}
