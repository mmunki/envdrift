package compare

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/mmunki/envdrift/internal/fingerprint"
)

func base() fingerprint.Fingerprint {
	return fingerprint.Fingerprint{
		SchemaVersion: fingerprint.SchemaVersion,
		Host: fingerprint.Host{
			OS:     "linux",
			Arch:   "amd64",
			Distro: "Ubuntu 26.04",
			CPUs:   8,
		},
		Tools: map[string]fingerprint.Tool{
			"go":   {Found: true, Path: "/usr/local/go/bin/go", Version: "go version go1.23.0 linux/amd64"},
			"node": {Found: true, Path: "/usr/bin/node", Version: "v22.0.0"},
		},
		Env:  map[string]string{"CC": "gcc", "FOO": "a"},
		Path: []string{"/usr/local/bin", "/usr/bin", "/bin"},
	}
}

func find(result Result, category, field string) *Difference {
	for i := range result.Differences {
		if result.Differences[i].Category == category && result.Differences[i].Field == field {
			return &result.Differences[i]
		}
	}
	return nil
}

func TestCompareIdentical(t *testing.T) {
	result := Compare(base(), base())
	if len(result.Differences) != 0 {
		t.Fatalf("got %d differences, want 0", len(result.Differences))
	}
	if result.Summary.Total != 0 {
		t.Fatalf("summary total = %d, want 0", result.Summary.Total)
	}
}

func TestComparePlatformIsCritical(t *testing.T) {
	right := base()
	right.Host.OS = "darwin"
	right.Host.Arch = "arm64"

	result := Compare(base(), right)
	osDiff := find(result, "host", "os")
	if osDiff == nil || osDiff.Severity != Critical {
		t.Fatalf("host.os diff = %+v, want critical", osDiff)
	}
	archDiff := find(result, "host", "arch")
	if archDiff == nil || archDiff.Severity != Critical {
		t.Fatalf("host.arch diff = %+v, want critical", archDiff)
	}
	if result.Differences[0].Severity != Critical {
		t.Error("critical differences are not sorted first")
	}
}

func TestCompareMissingToolIsCritical(t *testing.T) {
	right := base()
	delete(right.Tools, "go")

	result := Compare(base(), right)
	diff := find(result, "tool", "go")
	if diff == nil {
		t.Fatal("missing go tool was not reported")
	}
	if diff.Severity != Critical {
		t.Errorf("severity = %v, want critical", diff.Severity)
	}
	if diff.Right != "not found" {
		t.Errorf("right = %q, want not found", diff.Right)
	}
}

func TestCompareMajorVersionIsCritical(t *testing.T) {
	right := base()
	right.Tools["node"] = fingerprint.Tool{Found: true, Version: "v24.0.0"}

	result := Compare(base(), right)
	diff := find(result, "tool", "node")
	if diff == nil || diff.Severity != Critical {
		t.Fatalf("node diff = %+v, want critical", diff)
	}
}

func TestCompareMinorVersionIsWarning(t *testing.T) {
	right := base()
	right.Tools["go"] = fingerprint.Tool{Found: true, Version: "go version go1.24.0 linux/amd64"}

	result := Compare(base(), right)
	diff := find(result, "tool", "go")
	if diff == nil || diff.Severity != Warning {
		t.Fatalf("go diff = %+v, want warning", diff)
	}
}

func TestCompareEnvSeverity(t *testing.T) {
	right := base()
	right.Env["CC"] = "clang"
	right.Env["FOO"] = "b"

	result := Compare(base(), right)
	cc := find(result, "env", "CC")
	if cc == nil || cc.Severity != Warning {
		t.Fatalf("CC diff = %+v, want warning", cc)
	}
	foo := find(result, "env", "FOO")
	if foo == nil || foo.Severity != Info {
		t.Fatalf("FOO diff = %+v, want info", foo)
	}
}

func TestComparePath(t *testing.T) {
	right := base()
	right.Path = []string{"/usr/local/bin", "/usr/bin", "/opt/bin"}

	result := Compare(base(), right)
	diff := find(result, "path", "PATH")
	if diff == nil || diff.Severity != Warning {
		t.Fatalf("PATH diff = %+v, want warning", diff)
	}
	if !strings.Contains(diff.Impact, "/opt/bin") || !strings.Contains(diff.Impact, "/bin") {
		t.Errorf("impact does not name the differing entries: %q", diff.Impact)
	}
}

func TestCompareUnsetEnv(t *testing.T) {
	right := base()
	delete(right.Env, "CC")

	result := Compare(base(), right)
	diff := find(result, "env", "CC")
	if diff == nil || diff.Right != "<unset>" {
		t.Fatalf("CC diff = %+v, want right <unset>", diff)
	}
}

func TestSeverityJSON(t *testing.T) {
	data, err := json.Marshal(Difference{Severity: Warning})
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !strings.Contains(string(data), `"severity":"warning"`) {
		t.Errorf("severity did not marshal as a name: %s", data)
	}
	var diff Difference
	if err := json.Unmarshal([]byte(`{"severity":"critical"}`), &diff); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if diff.Severity != Critical {
		t.Errorf("severity = %v, want critical", diff.Severity)
	}
}

func TestParseSeverity(t *testing.T) {
	if got, err := ParseSeverity("warn"); err != nil || got != Warning {
		t.Errorf("ParseSeverity(warn) = %v, %v", got, err)
	}
	if _, err := ParseSeverity("loud"); err == nil {
		t.Error("ParseSeverity accepted an unknown name")
	}
}
