// Package compare turns two environment fingerprints into an ordered list of
// differences, each with a severity and a plain-language explanation.
package compare

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/mmunki/envdrift/internal/fingerprint"
)

// Severity is how likely a difference is to change the outcome of a command.
type Severity int

const (
	// Info differences are useful context but rarely explain a failure.
	Info Severity = iota
	// Warning differences commonly change behaviour.
	Warning
	// Critical differences make the two environments fundamentally different.
	Critical
)

func (s Severity) String() string {
	switch s {
	case Critical:
		return "critical"
	case Warning:
		return "warning"
	default:
		return "info"
	}
}

// MarshalJSON writes the severity as its name.
func (s Severity) MarshalJSON() ([]byte, error) {
	return json.Marshal(s.String())
}

// UnmarshalJSON reads a severity name.
func (s *Severity) UnmarshalJSON(data []byte) error {
	var name string
	if err := json.Unmarshal(data, &name); err != nil {
		return err
	}
	value, err := ParseSeverity(name)
	if err != nil {
		return err
	}
	*s = value
	return nil
}

// ParseSeverity parses a severity name.
func ParseSeverity(name string) (Severity, error) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "info":
		return Info, nil
	case "warning", "warn":
		return Warning, nil
	case "critical", "crit", "error":
		return Critical, nil
	default:
		return Info, fmt.Errorf("unknown severity %q (use info, warning or critical)", name)
	}
}

// Difference is one field that does not match between the two environments.
type Difference struct {
	Category string   `json:"category"`
	Field    string   `json:"field"`
	Left     string   `json:"left"`
	Right    string   `json:"right"`
	Severity Severity `json:"severity"`
	Impact   string   `json:"impact"`
}

// Summary counts the differences by severity.
type Summary struct {
	Critical int `json:"critical"`
	Warning  int `json:"warning"`
	Info     int `json:"info"`
	Total    int `json:"total"`
}

// Result is a complete comparison.
type Result struct {
	Left        fingerprint.Fingerprint `json:"left"`
	Right       fingerprint.Fingerprint `json:"right"`
	Differences []Difference            `json:"differences"`
	Summary     Summary                 `json:"summary"`
}

var digits = regexp.MustCompile(`\d+`)

var importantEnv = map[string]bool{
	"CC": true, "CXX": true, "CPP": true,
	"CFLAGS": true, "CXXFLAGS": true, "CPPFLAGS": true,
	"LDFLAGS": true, "LDLIBS": true,
	"LD_LIBRARY_PATH": true, "DYLD_LIBRARY_PATH": true,
	"DYLD_FALLBACK_LIBRARY_PATH": true,
	"PKG_CONFIG_PATH":            true, "PKG_CONFIG_LIBDIR": true,
	"JAVA_HOME": true, "GOROOT": true, "GOPATH": true,
	"GOFLAGS": true, "GOTOOLCHAIN": true, "GOMODCACHE": true,
	"GOCACHE": true, "CGO_ENABLED": true, "GOOS": true, "GOARCH": true,
	"NODE_OPTIONS": true, "NODE_PATH": true,
	"PYTHONPATH": true, "VIRTUAL_ENV": true, "CONDA_PREFIX": true,
	"RUSTUP_TOOLCHAIN": true, "RUSTFLAGS": true, "CARGO_HOME": true,
	"SDKROOT": true, "ANDROID_HOME": true, "ANDROID_SDK_ROOT": true,
	"ANDROID_NDK_HOME": true, "NDK_HOME": true,
	"MAVEN_OPTS": true, "GRADLE_OPTS": true,
	"DOTNET_ROOT": true, "KUBECONFIG": true, "DOCKER_HOST": true,
}

// Compare diffs two fingerprints. Differences are sorted from most to least
// severe, then by category and field so the output is stable.
func Compare(left, right fingerprint.Fingerprint) Result {
	diffs := make([]Difference, 0)
	add := func(category, field, leftValue, rightValue string, severity Severity, impact string) {
		if leftValue == rightValue {
			return
		}
		diffs = append(diffs, Difference{
			Category: category,
			Field:    field,
			Left:     leftValue,
			Right:    rightValue,
			Severity: severity,
			Impact:   impact,
		})
	}

	lh, rh := left.Host, right.Host
	add("host", "os", lh.OS, rh.OS, Critical,
		"The operating system differs. Native binaries, path separators, shell syntax and syscalls are not interchangeable.")
	add("host", "arch", lh.Arch, rh.Arch, Critical,
		"The CPU architecture differs. A binary built for one architecture cannot run on the other.")
	add("host", "distro", lh.Distro, rh.Distro, Warning,
		"The distribution differs. System libraries, package versions and default compilers can differ with it.")
	add("host", "kernel", lh.Kernel, rh.Kernel, Info,
		"The kernel version differs. This matters for container features, sandboxes and low-level networking.")
	add("host", "shell", lh.Shell, rh.Shell, Info,
		"The login shell differs. Shell startup files and quoting behaviour can change what a script actually runs.")
	add("host", "locale", lh.Locale, rh.Locale, Info,
		"The locale differs. Sorting, number formatting and compiler diagnostics can change with it.")
	add("host", "timezone", lh.Timezone, rh.Timezone, Info,
		"The timezone differs. Tests that depend on the current date or time can fail around a boundary.")
	add("host", "cpus", strconv.Itoa(lh.CPUs), strconv.Itoa(rh.CPUs), Info,
		"The CPU count differs. Parallel builds and tests can hit timing or resource assumptions.")
	add("host", "memory_bytes", humanBytes(lh.MemoryBytes), humanBytes(rh.MemoryBytes), Info,
		"The available memory differs. Linkers and test runners can be killed for using too much memory.")
	add("host", "term", lh.Term, rh.Term, Info,
		"The terminal differs. Colour detection and interactive prompts can behave differently.")

	for _, name := range unionKeys(left.Tools, right.Tools) {
		lt, leftKnown := left.Tools[name]
		rt, rightKnown := right.Tools[name]
		leftFound := leftKnown && lt.Found
		rightFound := rightKnown && rt.Found
		if !leftFound && !rightFound {
			continue
		}
		leftValue, rightValue := toolValue(lt, leftFound), toolValue(rt, rightFound)
		if leftValue == rightValue {
			continue
		}
		severity := Warning
		impact := fmt.Sprintf("%s is installed at a different version. Build output can change even when the source does not.", name)
		if !leftFound || !rightFound {
			severity = Critical
			impact = fmt.Sprintf("%s is installed on one side but missing on the other. A command that depends on it cannot run there.", name)
		} else if a, b := majorVersion(lt.Version), majorVersion(rt.Version); a != "" && b != "" && a != b {
			severity = Critical
			impact = fmt.Sprintf("%s has a different major version. APIs and command-line behaviour often change across major releases.", name)
		}
		add("tool", name, leftValue, rightValue, severity, impact)
	}

	for _, name := range unionKeys(left.Env, right.Env) {
		leftValue, leftSet := left.Env[name]
		rightValue, rightSet := right.Env[name]
		if !leftSet {
			leftValue = "<unset>"
		}
		if !rightSet {
			rightValue = "<unset>"
		}
		severity := Info
		impact := fmt.Sprintf("%s changes what a command sees, but is rarely the first thing to check.", name)
		if importantEnv[strings.ToUpper(name)] {
			severity = Warning
			impact = fmt.Sprintf("%s directly changes how the toolchain resolves compilers, libraries or runtimes.", name)
		}
		add("env", name, leftValue, rightValue, severity, impact)
	}

	leftPath := strings.Join(left.Path, string(os.PathListSeparator))
	rightPath := strings.Join(right.Path, string(os.PathListSeparator))
	if leftPath != rightPath {
		onlyRight := setDiff(right.Path, left.Path)
		onlyLeft := setDiff(left.Path, right.Path)
		severity := Info
		impact := "PATH contains the same directories in a different order."
		if len(onlyLeft)+len(onlyRight) > 0 {
			severity = Warning
			impact = "PATH resolves to different directories. The same command name can run a different binary."
			if len(onlyLeft) > 0 {
				impact += " Only on the left: " + strings.Join(limit(onlyLeft, 5), ", ") + "."
			}
			if len(onlyRight) > 0 {
				impact += " Only on the right: " + strings.Join(limit(onlyRight, 5), ", ") + "."
			}
		}
		diffs = append(diffs, Difference{
			Category: "path",
			Field:    "PATH",
			Left:     leftPath,
			Right:    rightPath,
			Severity: severity,
			Impact:   impact,
		})
	}

	for _, name := range unionKeys(left.Limits, right.Limits) {
		ll, leftSet := left.Limits[name]
		rl, rightSet := right.Limits[name]
		leftValue, rightValue := "<unset>", "<unset>"
		if leftSet {
			leftValue = fmt.Sprintf("soft=%d hard=%d", ll.Soft, ll.Hard)
		}
		if rightSet {
			rightValue = fmt.Sprintf("soft=%d hard=%d", rl.Soft, rl.Hard)
		}
		add("limit", name, leftValue, rightValue, Warning,
			"Resource limits can make a process fail with 'too many open files' or an allocation error even though the machine has capacity.")
	}

	switch {
	case left.Git == nil && right.Git == nil:
	case left.Git == nil || right.Git == nil:
		add("git", "repository", gitValue(left.Git), gitValue(right.Git), Info,
			"Git state was captured on only one side, so the checkout cannot be compared.")
	default:
		lg, rg := left.Git, right.Git
		add("git", "project", lg.Project, rg.Project, Info,
			"The checkouts have different project directory names. This is usually harmless.")
		add("git", "branch", lg.Branch, rg.Branch, Info,
			"The checkouts are on different branches. The source itself may differ.")
		add("git", "commit", lg.Commit, rg.Commit, Info,
			"The checkouts are at different commits. The source itself may differ.")
		add("git", "dirty", strconv.FormatBool(lg.Dirty), strconv.FormatBool(rg.Dirty), Info,
			"Uncommitted changes exist on one side, so the build is not from a clean tree.")
	}

	for _, name := range unionKeys(left.Labels, right.Labels) {
		leftValue, leftSet := left.Labels[name]
		rightValue, rightSet := right.Labels[name]
		if !leftSet {
			leftValue = "<unset>"
		}
		if !rightSet {
			rightValue = "<unset>"
		}
		add("label", name, leftValue, rightValue, Info,
			"The custom label differs. Labels are free-form metadata added by the caller.")
	}

	sort.SliceStable(diffs, func(i, j int) bool {
		if diffs[i].Severity != diffs[j].Severity {
			return diffs[i].Severity > diffs[j].Severity
		}
		if diffs[i].Category != diffs[j].Category {
			return diffs[i].Category < diffs[j].Category
		}
		return diffs[i].Field < diffs[j].Field
	})

	summary := Summary{Total: len(diffs)}
	for _, d := range diffs {
		switch d.Severity {
		case Critical:
			summary.Critical++
		case Warning:
			summary.Warning++
		default:
			summary.Info++
		}
	}

	return Result{Left: left, Right: right, Differences: diffs, Summary: summary}
}

func toolValue(tool fingerprint.Tool, found bool) string {
	if !found {
		return "not found"
	}
	if tool.Version != "" {
		return tool.Version
	}
	if tool.Path != "" {
		return tool.Path
	}
	return "found"
}

func majorVersion(version string) string {
	return digits.FindString(version)
}

func gitValue(g *fingerprint.Git) string {
	if g == nil {
		return "<no repository>"
	}
	if g.Commit == "" {
		return g.Branch
	}
	return g.Branch + "@" + g.Commit
}

func unionKeys[V any](maps ...map[string]V) []string {
	seen := map[string]struct{}{}
	for _, m := range maps {
		for key := range m {
			seen[key] = struct{}{}
		}
	}
	keys := make([]string, 0, len(seen))
	for key := range seen {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func setDiff(a, b []string) []string {
	set := make(map[string]struct{}, len(b))
	for _, value := range b {
		set[value] = struct{}{}
	}
	var out []string
	for _, value := range a {
		if _, ok := set[value]; !ok {
			out = append(out, value)
		}
	}
	return out
}

func limit(values []string, n int) []string {
	if len(values) <= n {
		return values
	}
	out := append([]string{}, values[:n]...)
	return append(out, fmt.Sprintf("and %d more", len(values)-n))
}

func humanBytes(n uint64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := uint64(unit), 0
	for v := n / unit; v >= unit; v /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
