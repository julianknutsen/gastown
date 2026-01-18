// Package town provides types for identifying towns in the Gas Town system.
package town

// ID represents a unique identifier for a town (the town root path).
type ID string

// IDFrom creates a town ID from a path string.
func IDFrom(path string) ID {
	return ID(path)
}

// String returns the path as a string.
func (id ID) String() string {
	return string(id)
}
