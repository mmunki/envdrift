//go:build linux || darwin

package fingerprint

import (
	"runtime"
	"syscall"
)

func captureLimits() map[string]Limit {
	limits := map[string]Limit{}
	add := func(name string, resource int) {
		var r syscall.Rlimit
		if err := syscall.Getrlimit(resource, &r); err == nil {
			limits[name] = Limit{Soft: uint64(r.Cur), Hard: uint64(r.Max)}
		}
	}
	add("nofile", syscall.RLIMIT_NOFILE)
	nproc := 6
	if runtime.GOOS == "darwin" {
		nproc = 7
	}
	add("nproc", nproc)
	return limits
}
