package main

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
)

// The widget presents the client key at upgrade. It checks the separate server
// key before displaying data, even if another account takes over a vacant port.
type sessionKeys struct {
	client string
	server string
}

// Call only after binding the port. A failed second instance must never
// replace the keys of the instance that actually owns the listener.
func publishSessionKeys(port int) (sessionKeys, error) {
	dir := os.Getenv("XDG_RUNTIME_DIR")
	if !filepath.IsAbs(dir) {
		return sessionKeys{}, fmt.Errorf("XDG_RUNTIME_DIR must be an absolute path")
	}
	info, err := os.Lstat(dir)
	if err != nil {
		return sessionKeys{}, err
	}
	stat, ok := info.Sys().(*syscall.Stat_t)
	if !info.IsDir() || info.Mode().Perm() != 0o700 || !ok || stat.Uid != uint32(os.Geteuid()) {
		return sessionKeys{}, fmt.Errorf("XDG_RUNTIME_DIR must be owned by the current user with mode 0700")
	}

	keys := sessionKeys{client: rand.Text(), server: rand.Text()}
	file, err := os.CreateTemp(dir, ".voicestatus-*.ini")
	if err != nil {
		return sessionKeys{}, err
	}
	defer os.Remove(file.Name())
	_, err = fmt.Fprintf(file, "[General]\nkeys=%s.%s\n", keys.client, keys.server)
	closeErr := file.Close()
	if err != nil {
		return sessionKeys{}, err
	}
	if closeErr != nil {
		return sessionKeys{}, closeErr
	}
	path := filepath.Join(dir, fmt.Sprintf("voicestatus-%d.ini", port))
	if err := os.Rename(file.Name(), path); err != nil {
		return sessionKeys{}, err
	}
	return keys, nil
}
