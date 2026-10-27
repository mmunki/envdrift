package fingerprint

import "regexp"

// Redacted is the placeholder that replaces a secret.
const Redacted = "<redacted>"

var secretName = regexp.MustCompile(`(?i)(^|_)(token|secret|password|passwd|pass|pwd|key|keys|credential|credentials|auth|private|session|cookie|pat|salt|signature|sig)($|_)`)

var secretValues = []*regexp.Regexp{
	regexp.MustCompile(`gh[pousr]_[A-Za-z0-9]{20,}`),
	regexp.MustCompile(`github_pat_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`sk-[A-Za-z0-9_-]{20,}`),
	regexp.MustCompile(`AKIA[0-9A-Z]{16}`),
	regexp.MustCompile(`xox[baprs]-[A-Za-z0-9-]{10,}`),
	regexp.MustCompile(`AIza[0-9A-Za-z_-]{30,}`),
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
}

var (
	bearerValue = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9._~+/-]{10,}`)
	urlCreds    = regexp.MustCompile(`://[^/\s:@]+:[^/\s@]+@`)
	querySecret = regexp.MustCompile(`(?i)([?&](?:token|key|secret|password|access_token)=)[^&\s]+`)
)

// Redact removes secrets from a single environment entry. It reports whether
// anything was replaced. A variable whose name looks secret is always replaced
// as a whole, regardless of its value.
func Redact(name, value string) (string, bool) {
	if secretName.MatchString(name) {
		return Redacted, true
	}
	out := value
	for _, re := range secretValues {
		out = re.ReplaceAllString(out, Redacted)
	}
	out = bearerValue.ReplaceAllString(out, "${1}"+Redacted)
	out = urlCreds.ReplaceAllString(out, "://"+Redacted+"@")
	out = querySecret.ReplaceAllString(out, "${1}"+Redacted)
	return out, out != value
}
