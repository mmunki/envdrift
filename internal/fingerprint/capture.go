package fingerprint

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type toolSpec struct {
	name string
	args []string
}

var toolSpecs = []toolSpec{
	{"go", []string{"version"}},
	{"node", []string{"--version"}},
	{"npm", []string{"--version"}},
	{"python3", []string{"--version"}},
	{"python", []string{"--version"}},
	{"java", []string{"-version"}},
	{"javac", []string{"-version"}},
	{"rustc", []string{"--version"}},
	{"cargo", []string{"--version"}},
	{"gcc", []string{"--version"}},
	{"clang", []string{"--version"}},
	{"make", []string{"--version"}},
	{"cmake", []string{"--version"}},
	{"ninja", []string{"--version"}},
	{"docker", []string{"--version"}},
	{"git", []string{"--version"}},
	{"bash", []string{"--version"}},
	{"zsh", []string{"--version"}},
	{"deno", []string{"--version"}},
	{"bun", []string{"--version"}},
	{"ruby", []string{"--version"}},
	{"php", []string{"--version"}},
	{"dotnet", []string{"--version"}},
}

var envExact = map[string]bool{
	"CC": true, "CXX": true, "CPP": true,
	"CFLAGS": true, "CXXFLAGS": true, "CPPFLAGS": true,
	"LDFLAGS": true, "LDLIBS": true,
	"LD_LIBRARY_PATH": true, "DYLD_LIBRARY_PATH": true,
	"DYLD_FALLBACK_LIBRARY_PATH": true,
	"PKG_CONFIG_PATH":            true, "PKG_CONFIG_LIBDIR": true,
	"TMPDIR": true, "TEMP": true, "TMP": true,
	"VIRTUAL_ENV": true, "CONDA_PREFIX": true, "CONDA_DEFAULT_ENV": true,
	"SDKROOT": true, "SHELL": true, "LANG": true, "TZ": true,
	"TERM": true, "TERM_PROGRAM": true, "CI": true,
	"GITHUB_ACTIONS": true, "RUNNER_OS": true, "RUNNER_ARCH": true,
	"IMAGEOS": true, "IMAGEVERSION": true,
	"KUBECONFIG": true, "DOCKER_HOST": true, "DOCKER_CONFIG": true,
	"NIX_PATH": true, "NIX_PROFILES": true,
}

var envPrefixes = []string{
	"LC_", "GITHUB_", "RUNNER_", "GO", "JAVA_", "NODE_", "NPM_",
	"PYTHON", "PIP_", "RUST", "CARGO_", "ANDROID_", "GRADLE_",
	"MAVEN_", "DOTNET_", "RUBY", "GEM_", "PHP_", "DENO_", "BUN_",
	"KUBE", "DOCKER_", "AWS_", "AZURE_", "GCLOUD", "GOOGLE_",
}

// Capture takes a snapshot of the current process environment and the
// checkout below WorkDir.
func Capture(opts Options) (Fingerprint, error) {
	workDir := opts.WorkDir
	if workDir == "" {
		wd, err := os.Getwd()
		if err != nil {
			return Fingerprint{}, fmt.Errorf("get working directory: %w", err)
		}
		workDir = wd
	}

	env, path, redacted := captureEnv(opts.AllEnv)
	fp := Fingerprint{
		SchemaVersion:   SchemaVersion,
		GeneratedAt:     time.Now().UTC(),
		EnvdriftVersion: opts.Version,
		Host:            captureHost(),
		Tools:           captureTools(),
		Env:             env,
		Path:            path,
		Limits:          captureLimits(),
		Git:             captureGit(workDir),
		Labels:          opts.Labels,
		Redacted:        redacted,
		AllEnv:          opts.AllEnv,
	}
	if len(fp.Labels) == 0 {
		fp.Labels = nil
	}
	return fp, nil
}

func captureHost() Host {
	h := Host{
		OS:     runtime.GOOS,
		Arch:   runtime.GOARCH,
		CPUs:   runtime.NumCPU(),
		Kernel: commandOutput("uname", "-r"),
		Distro: distro(),
		Shell:  firstNonEmpty(os.Getenv("SHELL"), os.Getenv("ComSpec")),
		Locale: firstNonEmpty(os.Getenv("LC_ALL"), os.Getenv("LC_CTYPE"), os.Getenv("LANG")),
		Term:   firstNonEmpty(os.Getenv("TERM"), os.Getenv("TERM_PROGRAM")),
	}
	h.Timezone = timezone()
	h.MemoryBytes = memoryBytes()
	return h
}

func captureTools() map[string]Tool {
	tools := make(map[string]Tool, len(toolSpecs))
	var mu sync.Mutex
	var wg sync.WaitGroup
	for _, spec := range toolSpecs {
		spec := spec
		wg.Add(1)
		go func() {
			defer wg.Done()
			tool := Tool{}
			if path, err := exec.LookPath(spec.name); err == nil {
				tool.Found = true
				tool.Path = path
				tool.Version = toolVersion(spec.name, spec.args)
			}
			mu.Lock()
			tools[spec.name] = tool
			mu.Unlock()
		}()
	}
	wg.Wait()
	return tools
}

func toolVersion(name string, args []string) string {
	return strings.Join(strings.Fields(commandOutput(name, args...)), " ")
}

func captureEnv(all bool) (map[string]string, []string, []string) {
	env := map[string]string{}
	var redacted []string
	var path []string
	for _, entry := range os.Environ() {
		name, value, ok := strings.Cut(entry, "=")
		if !ok {
			continue
		}
		if strings.EqualFold(name, "PATH") {
			path = filepath.SplitList(value)
			continue
		}
		if !all && !envSelected(name) {
			continue
		}
		clean, wasRedacted := Redact(name, value)
		if wasRedacted {
			redacted = append(redacted, name)
		}
		env[name] = clean
	}
	sort.Strings(redacted)
	return env, path, redacted
}

func envSelected(name string) bool {
	upper := strings.ToUpper(name)
	if envExact[upper] {
		return true
	}
	for _, prefix := range envPrefixes {
		if strings.HasPrefix(upper, prefix) {
			return true
		}
	}
	return false
}

func captureGit(dir string) *Git {
	root := gitOutput(dir, "rev-parse", "--show-toplevel")
	if root == "" {
		return nil
	}
	g := &Git{
		Project: filepath.Base(root),
		Branch:  gitOutput(root, "rev-parse", "--abbrev-ref", "HEAD"),
		Commit:  gitOutput(root, "rev-parse", "--short", "HEAD"),
	}
	if gitOutput(root, "status", "--porcelain") != "" {
		g.Dirty = true
	}
	return g
}

func gitOutput(dir string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", append([]string{"-C", dir}, args...)...)
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func commandOutput(name string, args ...string) string {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	out, err := exec.CommandContext(ctx, name, args...).CombinedOutput()
	if err != nil && len(out) == 0 {
		return ""
	}
	line := strings.TrimSpace(string(out))
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = strings.TrimSpace(line[:i])
	}
	return line
}

func distro() string {
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/etc/os-release")
		if err == nil {
			defer f.Close()
			values := map[string]string{}
			scanner := bufio.NewScanner(f)
			for scanner.Scan() {
				line := strings.TrimSpace(scanner.Text())
				if line == "" || strings.HasPrefix(line, "#") {
					continue
				}
				key, value, ok := strings.Cut(line, "=")
				if !ok {
					continue
				}
				values[key] = strings.Trim(strings.TrimSpace(value), `"'`)
			}
			if pretty := values["PRETTY_NAME"]; pretty != "" {
				return pretty
			}
			if name := values["NAME"]; name != "" {
				if version := values["VERSION_ID"]; version != "" {
					return name + " " + version
				}
				return name
			}
		}
	case "darwin":
		if version := commandOutput("sw_vers", "-productVersion"); version != "" {
			return "macOS " + version
		}
	case "windows":
		if version := commandOutput("cmd.exe", "/c", "ver"); version != "" {
			return version
		}
	}
	return runtime.GOOS
}

func memoryBytes() uint64 {
	switch runtime.GOOS {
	case "linux":
		f, err := os.Open("/proc/meminfo")
		if err != nil {
			return 0
		}
		defer f.Close()
		scanner := bufio.NewScanner(f)
		for scanner.Scan() {
			fields := strings.Fields(scanner.Text())
			if len(fields) >= 2 && fields[0] == "MemTotal:" {
				kb, err := strconv.ParseUint(fields[1], 10, 64)
				if err == nil {
					return kb * 1024
				}
			}
		}
	case "darwin":
		if out := commandOutput("sysctl", "-n", "hw.memsize"); out != "" {
			if n, err := strconv.ParseUint(strings.TrimSpace(out), 10, 64); err == nil {
				return n
			}
		}
	}
	return 0
}

func timezone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}
	if loc := time.Now().Location(); loc != nil && loc.String() != "Local" {
		return loc.String()
	}
	name, _ := time.Now().Zone()
	return name
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
