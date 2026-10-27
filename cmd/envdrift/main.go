package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/mmunki/envdrift/internal/compare"
	"github.com/mmunki/envdrift/internal/fingerprint"
	"github.com/mmunki/envdrift/internal/report"
)

var version = "dev"

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "capture":
		return cmdCapture(args[1:], stdout, stderr)
	case "diff":
		return cmdDiff(args[1:], stdout, stderr)
	case "snippet":
		return cmdSnippet(args[1:], stdout, stderr)
	case "version", "-v", "--version":
		fmt.Fprintf(stdout, "envdrift %s\n", version)
		return 0
	case "help", "-h", "--help":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "envdrift: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

type stringList []string

func (s *stringList) String() string { return strings.Join(*s, ",") }

func (s *stringList) Set(value string) error {
	*s = append(*s, value)
	return nil
}

func cmdCapture(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("capture", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var out string
	fs.StringVar(&out, "out", "", "write the fingerprint to this file instead of stdout")
	fs.StringVar(&out, "o", "", "shorthand for --out")
	allEnv := fs.Bool("all-env", false, "include every environment variable (still redacted)")
	quiet := fs.Bool("quiet", false, "do not print the output path")
	var labels stringList
	fs.Var(&labels, "label", "add a label as key=value (repeatable)")
	if err := fs.Parse(reorderFlags(args, map[string]bool{"out": true, "o": true, "label": true})); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "envdrift capture: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	labelMap, err := parseLabels(labels)
	if err != nil {
		fmt.Fprintf(stderr, "envdrift capture: %v\n", err)
		return 2
	}
	fp, err := fingerprint.Capture(fingerprint.Options{
		Version: version,
		AllEnv:  *allEnv,
		Labels:  labelMap,
	})
	if err != nil {
		fmt.Fprintf(stderr, "envdrift capture: %v\n", err)
		return 2
	}
	data, err := json.MarshalIndent(fp, "", "  ")
	if err != nil {
		fmt.Fprintf(stderr, "envdrift capture: %v\n", err)
		return 2
	}
	data = append(data, '\n')
	if out == "" || out == "-" {
		if _, err := stdout.Write(data); err != nil {
			fmt.Fprintf(stderr, "envdrift capture: %v\n", err)
			return 2
		}
		return 0
	}
	if err := os.WriteFile(out, data, 0o644); err != nil {
		fmt.Fprintf(stderr, "envdrift capture: %v\n", err)
		return 2
	}
	if !*quiet {
		fmt.Fprintf(stderr, "wrote %s\n", out)
	}
	return 0
}

func cmdDiff(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("diff", flag.ContinueOnError)
	fs.SetOutput(stderr)
	format := fs.String("format", "text", "output format: text, markdown or json")
	failOn := fs.String("fail-on", "warning", "exit 1 at this severity or above: info, warning, critical or never")
	noColor := fs.Bool("no-color", false, "disable colour")
	if err := fs.Parse(reorderFlags(args, map[string]bool{"format": true, "fail-on": true})); err != nil {
		return 2
	}
	if fs.NArg() != 2 {
		fmt.Fprintln(stderr, "usage: envdrift diff <left.json> <right.json>")
		return 2
	}
	left, err := loadFingerprint(fs.Arg(0))
	if err != nil {
		fmt.Fprintf(stderr, "envdrift diff: %v\n", err)
		return 2
	}
	right, err := loadFingerprint(fs.Arg(1))
	if err != nil {
		fmt.Fprintf(stderr, "envdrift diff: %v\n", err)
		return 2
	}
	result := compare.Compare(left, right)

	switch strings.ToLower(*format) {
	case "text":
		report.Text(stdout, result, !*noColor && os.Getenv("NO_COLOR") == "" && isTerminal(stdout))
	case "markdown", "md":
		report.Markdown(stdout, result)
	case "json":
		if err := report.JSON(stdout, result); err != nil {
			fmt.Fprintf(stderr, "envdrift diff: %v\n", err)
			return 2
		}
	default:
		fmt.Fprintf(stderr, "envdrift diff: unknown format %q (use text, markdown or json)\n", *format)
		return 2
	}

	if strings.EqualFold(*failOn, "never") {
		return 0
	}
	threshold, err := compare.ParseSeverity(*failOn)
	if err != nil {
		fmt.Fprintf(stderr, "envdrift diff: %v\n", err)
		return 2
	}
	for _, difference := range result.Differences {
		if difference.Severity >= threshold {
			return 1
		}
	}
	return 0
}

func cmdSnippet(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("snippet", flag.ContinueOnError)
	fs.SetOutput(stderr)
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() > 0 {
		fmt.Fprintf(stderr, "envdrift snippet: unexpected argument %q\n", fs.Arg(0))
		return 2
	}
	fmt.Fprint(stdout, snippetText)
	return 0
}

func loadFingerprint(path string) (fingerprint.Fingerprint, error) {
	var (
		data []byte
		err  error
	)
	if path == "-" {
		data, err = io.ReadAll(os.Stdin)
	} else {
		data, err = os.ReadFile(path)
	}
	if err != nil {
		return fingerprint.Fingerprint{}, fmt.Errorf("read %s: %w", path, err)
	}
	var fp fingerprint.Fingerprint
	if err := json.Unmarshal(data, &fp); err != nil {
		return fingerprint.Fingerprint{}, fmt.Errorf("parse %s: %w", path, err)
	}
	if fp.SchemaVersion == 0 {
		return fingerprint.Fingerprint{}, fmt.Errorf("%s is not an envdrift fingerprint (missing schema_version)", path)
	}
	if fp.SchemaVersion > fingerprint.SchemaVersion {
		return fingerprint.Fingerprint{}, fmt.Errorf("%s uses schema version %d, this build supports %d", path, fp.SchemaVersion, fingerprint.SchemaVersion)
	}
	return fp, nil
}

func parseLabels(values []string) (map[string]string, error) {
	if len(values) == 0 {
		return nil, nil
	}
	labels := make(map[string]string, len(values))
	for _, value := range values {
		key, val, ok := strings.Cut(value, "=")
		if !ok || strings.TrimSpace(key) == "" {
			return nil, fmt.Errorf("invalid label %q (expected key=value)", value)
		}
		labels[strings.TrimSpace(key)] = strings.TrimSpace(val)
	}
	return labels, nil
}

func reorderFlags(args []string, valueFlags map[string]bool) []string {
	var flags, positional []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			positional = append(positional, arg)
			continue
		}
		flags = append(flags, arg)
		name := strings.TrimLeft(arg, "-")
		if eq := strings.IndexByte(name, '='); eq >= 0 {
			name = name[:eq]
		}
		if valueFlags[name] && !strings.Contains(arg, "=") && i+1 < len(args) {
			i++
			flags = append(flags, args[i])
		}
	}
	return append(flags, positional...)
}

func isTerminal(w io.Writer) bool {
	file, ok := w.(*os.File)
	if !ok {
		return false
	}
	info, err := file.Stat()
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeCharDevice != 0
}

const snippetText = `# Add this to a workflow to keep an environment fingerprint with every CI run.
- name: Capture environment fingerprint
  run: envdrift capture --out envdrift-ci.json

- name: Upload environment fingerprint
  uses: actions/upload-artifact@v4
  with:
    name: envdrift-${{ runner.os }}-${{ runner.arch }}
    path: envdrift-ci.json
    if-no-files-found: error

# When a CI run fails, download the artifact and compare it with your machine:
#   envdrift diff envdrift-ci.json envdrift-local.json
`

func usage(w io.Writer) {
	fmt.Fprint(w, `envdrift - explain why a command works on one machine and fails on another.

Usage:
  envdrift capture [--out FILE] [--all-env] [--label key=value]
  envdrift diff LEFT.json RIGHT.json [--format text|markdown|json] [--fail-on LEVEL]
  envdrift snippet
  envdrift version

capture records the OS, architecture, tool versions, selected environment
variables, resource limits and git state. Secret-looking variables and values
are replaced with <redacted> before anything is written.

diff compares two captures and explains each difference. It exits 0 when the
environments match, 1 when a difference at or above --fail-on exists (default:
warning) and 2 on error. Use --fail-on never to always exit 0.

Supported release targets: linux/amd64, linux/arm64, darwin/amd64,
darwin/arm64, windows/amd64, windows/arm64.
`)
}
