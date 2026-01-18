package polecat

import (
	"encoding/base64"
	"fmt"
	"io/fs"
	"os"
	"strings"

	"github.com/steveyegge/gastown/internal/runner"
)

// Filesystem abstracts filesystem operations for testability.
// LocalFilesystem uses Go stdlib (cross-platform).
// RemoteFilesystem uses Runner to execute POSIX commands via SSH.
type Filesystem interface {
	// MkdirAll creates a directory and all parent directories.
	MkdirAll(path string, perm os.FileMode) error

	// RemoveAll removes a path and all its children.
	RemoveAll(path string) error

	// Exists checks if a path exists.
	Exists(path string) bool

	// IsDir checks if a path is a directory.
	IsDir(path string) bool

	// ReadDir returns the contents of a directory.
	ReadDir(path string) ([]fs.DirEntry, error)

	// ReadFile reads an entire file.
	ReadFile(path string) ([]byte, error)

	// WriteFile writes data to a file.
	WriteFile(path string, data []byte, perm os.FileMode) error
}

// LocalFilesystem implements Filesystem using Go stdlib.
// This is cross-platform (works on Windows, Linux, macOS).
type LocalFilesystem struct{}

// NewLocalFilesystem creates a new LocalFilesystem.
func NewLocalFilesystem() *LocalFilesystem {
	return &LocalFilesystem{}
}

func (f *LocalFilesystem) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}

func (f *LocalFilesystem) RemoveAll(path string) error {
	return os.RemoveAll(path)
}

func (f *LocalFilesystem) Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func (f *LocalFilesystem) IsDir(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func (f *LocalFilesystem) ReadDir(path string) ([]fs.DirEntry, error) {
	return os.ReadDir(path)
}

func (f *LocalFilesystem) ReadFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func (f *LocalFilesystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	return os.WriteFile(path, data, perm)
}

// RemoteFilesystem implements Filesystem using Runner for SSH commands.
// Assumes POSIX remote (Linux/macOS servers).
type RemoteFilesystem struct {
	runner runner.Runner
}

// NewRemoteFilesystem creates a new RemoteFilesystem using the given runner.
func NewRemoteFilesystem(r runner.Runner) *RemoteFilesystem {
	return &RemoteFilesystem{runner: r}
}

func (f *RemoteFilesystem) MkdirAll(path string, perm os.FileMode) error {
	// mkdir -p handles creating parent directories
	// chmod afterward to set permissions (mkdir -p doesn't respect -m on all systems)
	if err := f.runner.Run("", "mkdir", "-p", path); err != nil {
		return err
	}
	return f.runner.Run("", "chmod", permToOctal(perm), path)
}

func (f *RemoteFilesystem) RemoveAll(path string) error {
	return f.runner.Run("", "rm", "-rf", path)
}

func (f *RemoteFilesystem) Exists(path string) bool {
	return f.runner.Run("", "test", "-e", path) == nil
}

func (f *RemoteFilesystem) IsDir(path string) bool {
	return f.runner.Run("", "test", "-d", path) == nil
}

func (f *RemoteFilesystem) ReadDir(path string) ([]fs.DirEntry, error) {
	out, err := f.runner.Output("", "ls", "-1", path)
	if err != nil {
		return nil, err
	}

	var entries []fs.DirEntry
	lines := splitLines(string(out))
	for _, name := range lines {
		if name == "" {
			continue
		}
		entries = append(entries, &remoteDirEntry{
			name:   name,
			runner: f.runner,
			parent: path,
		})
	}
	return entries, nil
}

func (f *RemoteFilesystem) ReadFile(path string) ([]byte, error) {
	return f.runner.Output("", "cat", path)
}

func (f *RemoteFilesystem) WriteFile(path string, data []byte, perm os.FileMode) error {
	// Use base64 encoding to safely transfer binary data
	// This avoids shell escaping issues with binary content
	encoded := base64Encode(data)
	script := "echo " + encoded + " | base64 -d > " + shellEscapeForScript(path)
	if err := f.runner.Run("", "sh", "-c", script); err != nil {
		return err
	}
	return f.runner.Run("", "chmod", permToOctal(perm), path)
}

// remoteDirEntry implements fs.DirEntry for remote directory listings.
type remoteDirEntry struct {
	name   string
	runner runner.Runner
	parent string
}

func (e *remoteDirEntry) Name() string { return e.name }

func (e *remoteDirEntry) IsDir() bool {
	path := e.parent + "/" + e.name
	return e.runner.Run("", "test", "-d", path) == nil
}

func (e *remoteDirEntry) Type() fs.FileMode {
	if e.IsDir() {
		return fs.ModeDir
	}
	return 0
}

func (e *remoteDirEntry) Info() (fs.FileInfo, error) {
	return nil, fs.ErrNotExist // Not implemented
}

// Helper functions

func permToOctal(perm os.FileMode) string {
	return fmt.Sprintf("%o", perm&os.ModePerm)
}

func splitLines(s string) []string {
	var lines []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func shellEscapeForScript(s string) string {
	// For use inside a script passed to sh -c
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}
