package util

import (
	"os"
	"path/filepath"
	"syscall"
)

// FileLock provides cross-process file locking using flock.
type FileLock struct {
	path string
	file *os.File
}

// NewFileLock creates a new file lock at the given path.
// The lock file is created if it doesn't exist.
func NewFileLock(path string) (*FileLock, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, err
	}

	return &FileLock{path: path, file: file}, nil
}

// Lock acquires an exclusive lock, blocking until available.
func (l *FileLock) Lock() error {
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_EX)
}

// Unlock releases the lock.
func (l *FileLock) Unlock() error {
	return syscall.Flock(int(l.file.Fd()), syscall.LOCK_UN)
}

// Close releases the lock and closes the file.
func (l *FileLock) Close() error {
	l.Unlock()
	return l.file.Close()
}
