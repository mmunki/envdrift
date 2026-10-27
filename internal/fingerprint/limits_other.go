//go:build !linux && !darwin

package fingerprint

func captureLimits() map[string]Limit {
	return nil
}
