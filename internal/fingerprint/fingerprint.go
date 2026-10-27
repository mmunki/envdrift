// Package fingerprint captures a portable, secret-redacted description of the
// environment a command runs in.
package fingerprint

import "time"

// SchemaVersion is the version of the on-disk fingerprint format.
const SchemaVersion = 1

// Fingerprint is the complete environment snapshot.
type Fingerprint struct {
	SchemaVersion   int               `json:"schema_version"`
	GeneratedAt     time.Time         `json:"generated_at"`
	EnvdriftVersion string            `json:"envdrift_version"`
	Host            Host              `json:"host"`
	Tools           map[string]Tool   `json:"tools"`
	Env             map[string]string `json:"env"`
	Path            []string          `json:"path"`
	Limits          map[string]Limit  `json:"limits,omitempty"`
	Git             *Git              `json:"git,omitempty"`
	Labels          map[string]string `json:"labels,omitempty"`
	Redacted        []string          `json:"redacted,omitempty"`
	AllEnv          bool              `json:"all_env,omitempty"`
}

// Host describes the machine itself.
type Host struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	Kernel      string `json:"kernel,omitempty"`
	Distro      string `json:"distro,omitempty"`
	Shell       string `json:"shell,omitempty"`
	Locale      string `json:"locale,omitempty"`
	Timezone    string `json:"timezone,omitempty"`
	CPUs        int    `json:"cpus"`
	MemoryBytes uint64 `json:"memory_bytes,omitempty"`
	Term        string `json:"term,omitempty"`
}

// Tool is one compiler, runtime or build tool found on PATH.
type Tool struct {
	Found   bool   `json:"found"`
	Path    string `json:"path,omitempty"`
	Version string `json:"version,omitempty"`
}

// Limit is one resource limit.
type Limit struct {
	Soft uint64 `json:"soft"`
	Hard uint64 `json:"hard"`
}

// Git is the state of the checkout the fingerprint was taken in.
type Git struct {
	Project string `json:"project,omitempty"`
	Branch  string `json:"branch,omitempty"`
	Commit  string `json:"commit,omitempty"`
	Dirty   bool   `json:"dirty"`
}

// Options controls a capture.
type Options struct {
	Version string
	AllEnv  bool
	Labels  map[string]string
	WorkDir string
}
