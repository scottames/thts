package fs

import (
	"os"
	"path/filepath"
)

// CreateSymlink creates a symbolic link at linkPath pointing to target.
// If linkPath already exists, it returns an error.
func CreateSymlink(target, linkPath string) error {
	return os.Symlink(target, linkPath)
}

// IsSymlink returns true if the path is a symbolic link.
func IsSymlink(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.Mode()&os.ModeSymlink != 0
}

// SymlinkTarget returns the target of a symbolic link.
func SymlinkTarget(path string) (string, error) {
	return os.Readlink(path)
}

// ResolveSymlink resolves a symbolic link to its final target (like realpath).
func ResolveSymlink(path string) (string, error) {
	return filepath.EvalSymlinks(path)
}

// RemoveAll removes path and any children it contains.
// It handles permission issues with special directories like searchable/.
func RemoveAll(path string) error {
	// First try to fix permissions if it's a directory
	info, err := os.Lstat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Already doesn't exist
		}
		return err
	}

	if info.IsDir() {
		// Walk the directory and fix permissions
		_ = filepath.WalkDir(path, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				// Try to fix permission and retry
				if os.IsPermission(err) {
					_ = os.Chmod(p, 0755)
				}
				return nil // Continue walking
			}
			if d.IsDir() {
				_ = os.Chmod(p, 0755)
			}
			return nil
		})
		// Ignore walk errors, try to remove anyway
	}

	return os.RemoveAll(path)
}

// EnsureDir creates a directory and all parent directories if they don't exist.
func EnsureDir(path string) error {
	return os.MkdirAll(path, 0755)
}

// Exists returns true if the path exists (follows symlinks).
func Exists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// ExistsNoFollow returns true if the path exists (does not follow symlinks).
func ExistsNoFollow(path string) bool {
	_, err := os.Lstat(path)
	return err == nil
}

// IsDir returns true if the path is a directory (follows symlinks).
func IsDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// IsDirNoFollow returns true if the path is a directory (does not follow symlinks).
func IsDirNoFollow(path string) bool {
	info, err := os.Lstat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}
