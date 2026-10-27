package fingerprint

import (
	"strings"
	"testing"
)

func TestRedactSecretNames(t *testing.T) {
	names := []string{
		"GITHUB_TOKEN",
		"AWS_SECRET_ACCESS_KEY",
		"DB_PASSWORD",
		"SSH_AUTH_SOCK",
		"NPM_CONFIG__AUTH",
		"API_KEY",
		"PRIVATE_KEY",
		"SESSION_ID",
		"GITHUB_PAT",
		"MY_CREDENTIALS",
	}
	for _, name := range names {
		got, changed := Redact(name, "ordinary-value")
		if !changed {
			t.Errorf("Redact(%q) did not redact the name", name)
		}
		if got != Redacted {
			t.Errorf("Redact(%q) = %q, want %q", name, got, Redacted)
		}
	}
}

func TestRedactKeepsNormalNames(t *testing.T) {
	names := []string{"PATH", "GOPATH", "JAVA_HOME", "MONKEY", "COMPASS", "TERM"}
	for _, name := range names {
		got, changed := Redact(name, "/usr/local/bin")
		if changed {
			t.Errorf("Redact(%q) unexpectedly redacted a normal name", name)
		}
		if got != "/usr/local/bin" {
			t.Errorf("Redact(%q) = %q, want the value unchanged", name, got)
		}
	}
}

func TestRedactValues(t *testing.T) {
	tests := []struct {
		name  string
		value string
		want  string
	}{
		{"gh-token", "ghp_abcdefghijklmnopqrstuvwxyz0123456789", Redacted},
		{"github-pat", "github_pat_abcdefghijklmnopqrstuvwxyz0123456789", Redacted},
		{"openai", "sk-abcdefghijklmnopqrstuvwxyz0123456789", Redacted},
		{"aws", "AKIAABCDEFGHIJKLMNOP", Redacted},
		{"slack", "xoxb-1234567890-abcdefghij", Redacted},
		{"url-creds", "https://user:pass@example.com/repo", "https://" + Redacted + "@example.com/repo"},
		{"query", "https://example.com/cb?access_token=abc123&x=1", "https://example.com/cb?access_token=" + Redacted + "&x=1"},
		{"bearer", "Authorization: Bearer abcdefghijklmnop", "Authorization: Bearer " + Redacted},
		{"plain", "clang -O2", "clang -O2"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, _ := Redact("SOME_VAR", test.value)
			if got != test.want {
				t.Errorf("Redact(%q) = %q, want %q", test.value, got, test.want)
			}
		})
	}
}

func TestRedactPrivateKey(t *testing.T) {
	value := "-----BEGIN RSA PRIVATE KEY-----\nMIIEsecret\n-----END RSA PRIVATE KEY-----"
	got, changed := Redact("DEPLOY_VALUE", value)
	if !changed {
		t.Fatal("private key was not redacted")
	}
	if strings.Contains(got, "MIIEsecret") {
		t.Fatalf("private key body leaked: %q", got)
	}
}
