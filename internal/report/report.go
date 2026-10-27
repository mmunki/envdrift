// Package report renders a comparison as text, Markdown or JSON.
package report

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/mmunki/envdrift/internal/compare"
	"github.com/mmunki/envdrift/internal/fingerprint"
)

const maxValue = 140

var severityColor = map[compare.Severity]string{
	compare.Critical: "\x1b[31m",
	compare.Warning:  "\x1b[33m",
	compare.Info:     "\x1b[36m",
}

// Text writes a human readable report.
func Text(w io.Writer, result compare.Result, color bool) {
	fmt.Fprintln(w, "envdrift diff")
	fmt.Fprintf(w, "  left:  %s\n", describe(result.Left))
	fmt.Fprintf(w, "  right: %s\n", describe(result.Right))
	fmt.Fprintln(w)

	if len(result.Differences) == 0 {
		fmt.Fprintln(w, "No differences found. The two environments match.")
		return
	}

	current := compare.Severity(-1)
	for _, d := range result.Differences {
		if d.Severity != current {
			current = d.Severity
			header := strings.ToUpper(current.String())
			if color {
				header = severityColor[current] + header + "\x1b[0m"
			}
			fmt.Fprintf(w, "%s (%d)\n", header, count(result, current))
		}
		fmt.Fprintf(w, "  %-30s %s -> %s\n", d.Category+"."+d.Field, short(d.Left), short(d.Right))
		fmt.Fprintf(w, "      %s\n", d.Impact)
	}

	fmt.Fprintln(w)
	fmt.Fprintf(w, "Summary: %d critical, %d warning, %d info\n",
		result.Summary.Critical, result.Summary.Warning, result.Summary.Info)
}

// Markdown writes a report suitable for an issue, pull request or CI summary.
func Markdown(w io.Writer, result compare.Result) {
	fmt.Fprintln(w, "# envdrift report")
	fmt.Fprintln(w)
	fmt.Fprintln(w, "| | Environment |")
	fmt.Fprintln(w, "| --- | --- |")
	fmt.Fprintf(w, "| Left | %s |\n", md(describe(result.Left)))
	fmt.Fprintf(w, "| Right | %s |\n", md(describe(result.Right)))
	fmt.Fprintln(w)

	if len(result.Differences) == 0 {
		fmt.Fprintln(w, "No differences found. The two environments match.")
		return
	}

	for _, severity := range []compare.Severity{compare.Critical, compare.Warning, compare.Info} {
		diffs := bySeverity(result, severity)
		if len(diffs) == 0 {
			continue
		}
		fmt.Fprintf(w, "## %s (%d)\n\n", title(severity), len(diffs))
		fmt.Fprintln(w, "| Field | Left | Right | Why it matters |")
		fmt.Fprintln(w, "| --- | --- | --- | --- |")
		for _, d := range diffs {
			fmt.Fprintf(w, "| `%s.%s` | %s | %s | %s |\n",
				d.Category, d.Field, md(short(d.Left)), md(short(d.Right)), md(d.Impact))
		}
		fmt.Fprintln(w)
	}

	fmt.Fprintf(w, "**Summary:** %d critical, %d warning, %d info\n",
		result.Summary.Critical, result.Summary.Warning, result.Summary.Info)
}

// JSON writes the full comparison as indented JSON.
func JSON(w io.Writer, result compare.Result) error {
	encoder := json.NewEncoder(w)
	encoder.SetIndent("", "  ")
	return encoder.Encode(result)
}

func describe(fp fingerprint.Fingerprint) string {
	parts := []string{fp.Host.OS + "/" + fp.Host.Arch}
	if fp.Host.Distro != "" && fp.Host.Distro != fp.Host.OS {
		parts = append(parts, fp.Host.Distro)
	}
	if fp.Git != nil && fp.Git.Commit != "" {
		parts = append(parts, fp.Git.Branch+"@"+fp.Git.Commit)
	}
	return strings.Join(parts, ", ")
}

func count(result compare.Result, severity compare.Severity) int {
	switch severity {
	case compare.Critical:
		return result.Summary.Critical
	case compare.Warning:
		return result.Summary.Warning
	default:
		return result.Summary.Info
	}
}

func bySeverity(result compare.Result, severity compare.Severity) []compare.Difference {
	var out []compare.Difference
	for _, d := range result.Differences {
		if d.Severity == severity {
			out = append(out, d)
		}
	}
	return out
}

func title(severity compare.Severity) string {
	name := severity.String()
	return strings.ToUpper(name[:1]) + name[1:]
}

func short(value string) string {
	value = strings.ReplaceAll(value, "\n", " ")
	value = strings.ReplaceAll(value, "\r", "")
	if len(value) <= maxValue {
		return value
	}
	return value[:maxValue-3] + "..."
}

func md(value string) string {
	value = strings.ReplaceAll(value, "|", "\\|")
	value = strings.ReplaceAll(value, "\n", "<br>")
	return value
}
