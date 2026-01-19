// Package util provides common utilities for Gas Town.
package util

import (
	"encoding/json"
	"os"
)

// AtomicWriteJSON writes JSON data to a file atomically.
// It first writes to a temporary file, then renames it to the target path.
// This prevents data corruption if the process crashes during write.
// The rename operation is atomic on POSIX systems.
func AtomicWriteJSON(path string, v interface{}) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return AtomicWriteFile(path, data, 0644)
}

// AtomicWriteFile writes data to a file atomically.
// It first writes to a temporary file, then renames it to the target path.
// This prevents data corruption if the process crashes during write.
// The rename operation is atomic on POSIX systems.
//
// Error handling pattern: When the primary operation (rename) fails, cleanup
// errors from os.Remove are intentionally suppressed using blank identifier
// assignment. This follows the principle that the original error is more
// informative to the caller than a secondary cleanup failure. The caller
// needs to know why the write failed, not that cleanup also failed.
func AtomicWriteFile(path string, data []byte, perm os.FileMode) error {
	tmpFile := path + ".tmp"

	// Write to temp file
	if err := os.WriteFile(tmpFile, data, perm); err != nil {
		return err
	}

	// Atomic rename (on POSIX systems)
	if err := os.Rename(tmpFile, path); err != nil {
		// Clean up temp file on failure
		_ = os.Remove(tmpFile)
		return err
	}

	return nil
}
