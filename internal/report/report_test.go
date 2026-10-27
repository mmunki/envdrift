package report

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mmunki/envdrift/internal/compare"
	"github.com/mmunki/envdrift/internal/fingerprint"
)

func sample() compare.Result {
	left := fingerprint.Fingerprint{
		SchemaVersion: fingerprint.SchemaVersion,
		Host:          fingerprint.Host{OS: "linux", Arch: "amd64", Distro: "Ubuntu 26.04"},
		Tools:         map[string]fingerprint.Tool{"go": {Found: true, Version: "go1.23"}},
	}
	right := fingerprint.Fingerprint{
		SchemaVersion: fingerprint.SchemaVersion,
		Host:          fingerprint.Host{OS: "darwin", Arch: "arm64", Distro: "macOS 15.1"},
		Tools:         map[string]fingerprint.Tool{"go": {Found: true, Version: "go1.24"}},
	}
	return compare.Compare(left, right)
}

func TestText(t *testing.T) {
	var out strings.Builder
	Text(&out, sample(), false)
	text := out.String()
	for _, want := range []string{"envdrift diff", "CRITICAL", "linux/amd64", "darwin/arm64", "Summary:"} {
		if !strings.Contains(text, want) {
			t.Errorf("text report is missing %q:\n%s", want, text)
		}
	}
	if strings.Contains(text, "\x1b[") {
		t.Error("text report contains colour while colour is disabled")
	}
}

func TestTextColour(t *testing.T) {
	var out strings.Builder
	Text(&out, sample(), true)
	if !strings.Contains(out.String(), "\x1b[31m") {
		t.Error("coloured report has no critical colour")
	}
}

func TestMarkdown(t *testing.T) {
	var out strings.Builder
	Markdown(&out, sample())
	text := out.String()
	for _, want := range []string{"# envdrift report", "Why it matters", "| Field |", "**Summary:**"} {
		if !strings.Contains(text, want) {
			t.Errorf("markdown report is missing %q:\n%s", want, text)
		}
	}
}

func TestJSON(t *testing.T) {
	var out strings.Builder
	if err := JSON(&out, sample()); err != nil {
		t.Fatalf("JSON: %v", err)
	}
	var decoded struct {
		Differences []struct {
			Severity string `json:"severity"`
		} `json:"differences"`
		Summary struct {
			Critical int `json:"critical"`
		} `json:"summary"`
	}
	if err := json.Unmarshal([]byte(out.String()), &decoded); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if len(decoded.Differences) == 0 {
		t.Fatal("no differences in JSON")
	}
	if decoded.Differences[0].Severity != "critical" {
		t.Errorf("first severity = %q, want critical", decoded.Differences[0].Severity)
	}
	if decoded.Summary.Critical == 0 {
		t.Error("critical summary is zero")
	}
}
