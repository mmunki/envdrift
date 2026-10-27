package fingerprint

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestCaptureFiltersAndRedacts(t *testing.T) {
	t.Setenv("ENVDRIFT_UNRELATED", "keep-out")
	t.Setenv("CC", "clang")
	t.Setenv("GITHUB_TOKEN", "super-secret-value")
	t.Setenv("CFLAGS", "https://example.com/?token=abc123")

	fp, err := Capture(Options{Version: "test", WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if fp.SchemaVersion != SchemaVersion {
		t.Errorf("SchemaVersion = %d, want %d", fp.SchemaVersion, SchemaVersion)
	}
	if fp.Host.OS == "" || fp.Host.Arch == "" {
		t.Error("host os/arch is empty")
	}
	if _, ok := fp.Env["ENVDRIFT_UNRELATED"]; ok {
		t.Error("an unrelated variable was captured without --all-env")
	}
	if fp.Env["CC"] != "clang" {
		t.Errorf("CC = %q, want clang", fp.Env["CC"])
	}
	if fp.Env["GITHUB_TOKEN"] != Redacted {
		t.Errorf("GITHUB_TOKEN = %q, want %q", fp.Env["GITHUB_TOKEN"], Redacted)
	}
	if !strings.Contains(fp.Env["CFLAGS"], Redacted) {
		t.Errorf("CFLAGS was not redacted: %q", fp.Env["CFLAGS"])
	}
	if !contains(fp.Redacted, "GITHUB_TOKEN") {
		t.Errorf("GITHUB_TOKEN missing from Redacted: %v", fp.Redacted)
	}
	if len(fp.Tools) == 0 {
		t.Error("no tools were captured")
	}
	if _, ok := fp.Tools["go"]; !ok {
		t.Error("go is missing from the tool set")
	}

	data, err := json.Marshal(fp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	var back Fingerprint
	if err := json.Unmarshal(data, &back); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if back.Host.OS != fp.Host.OS || back.Host.Arch != fp.Host.Arch {
		t.Error("host did not survive a JSON round trip")
	}
}

func TestCaptureAllEnv(t *testing.T) {
	t.Setenv("ENVDRIFT_UNRELATED", "hello")
	fp, err := Capture(Options{Version: "test", AllEnv: true, WorkDir: t.TempDir()})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if fp.Env["ENVDRIFT_UNRELATED"] != "hello" {
		t.Errorf("ENVDRIFT_UNRELATED = %q, want hello", fp.Env["ENVDRIFT_UNRELATED"])
	}
	if !fp.AllEnv {
		t.Error("AllEnv was not recorded")
	}
}

func TestCaptureLabels(t *testing.T) {
	fp, err := Capture(Options{
		Version: "test",
		WorkDir: t.TempDir(),
		Labels:  map[string]string{"pipeline": "ci"},
	})
	if err != nil {
		t.Fatalf("Capture: %v", err)
	}
	if fp.Labels["pipeline"] != "ci" {
		t.Errorf("label = %q, want ci", fp.Labels["pipeline"])
	}
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
