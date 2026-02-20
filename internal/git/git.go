package git

import (
	"errors"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// ErrNotInGitRepo is returned when the current directory is not in a git repository.
var ErrNotInGitRepo = errors.New("not in a git repository")

// IsInGitRepo checks if the current directory is in a git repository.
func IsInGitRepo() bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	return cmd.Run() == nil
}

// IsInGitRepoAt checks if the given directory is in a git repository.
func IsInGitRepoAt(dir string) bool {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	return cmd.Run() == nil
}

// GetGitDir returns the .git directory path for the current repository.
func GetGitDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitDirAt returns the .git directory path for the repository at the given directory.
func GetGitDirAt(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-dir")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}

	result := strings.TrimSpace(string(output))
	// If relative path, make it absolute relative to dir
	if !filepath.IsAbs(result) {
		result = filepath.Join(dir, result)
	}
	return result, nil
}

// GetGitCommonDir returns the common git directory (for worktree support).
// In a regular repo this is the same as git-dir.
// In a worktree, this is the main repo's git directory where hooks are shared.
func GetGitCommonDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitCommonDirAt returns the common git directory for the repo at the given directory.
func GetGitCommonDirAt(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--git-common-dir")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}

	result := strings.TrimSpace(string(output))
	// If relative path, make it absolute relative to dir
	if !filepath.IsAbs(result) {
		result = filepath.Join(dir, result)
	}
	return result, nil
}

// IsWorktree returns true if the current directory is in a git worktree.
func IsWorktree() bool {
	gitDir, err := GetGitDir()
	if err != nil {
		return false
	}
	commonDir, err := GetGitCommonDir()
	if err != nil {
		return false
	}
	return gitDir != commonDir
}

// IsWorktreeAt returns true if the given directory is in a git worktree.
func IsWorktreeAt(dir string) bool {
	gitDir, err := GetGitDirAt(dir)
	if err != nil {
		return false
	}
	commonDir, err := GetGitCommonDirAt(dir)
	if err != nil {
		return false
	}
	return gitDir != commonDir
}

// GetRemoteURL returns the URL of the origin remote.
func GetRemoteURL() (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("no origin remote configured")
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRemoteURLAt returns the URL of the origin remote for the repo at the given directory.
func GetRemoteURLAt(dir string) (string, error) {
	cmd := exec.Command("git", "remote", "get-url", "origin")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", errors.New("no origin remote configured")
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRepoNameFromRemote extracts the repository name from a git remote URL.
// Handles various formats:
//   - https://github.com/user/repo.git -> repo
//   - git@github.com:user/repo.git -> repo
//   - https://github.com/user/repo -> repo
//   - ssh://git@github.com/user/repo.git -> repo
func GetRepoNameFromRemote(url string) string {
	if url == "" {
		return ""
	}

	// Remove trailing .git
	url = strings.TrimSuffix(url, ".git")

	// Handle SSH format: git@github.com:user/repo
	if strings.Contains(url, ":") && !strings.Contains(url, "://") {
		parts := strings.Split(url, ":")
		if len(parts) == 2 {
			url = parts[1]
		}
	}

	// Extract last path component
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}

	return ""
}

// GetRepoTopLevel returns the top-level directory of the repository.
func GetRepoTopLevel() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRepoTopLevelAt returns the top-level directory of the repository at the given directory.
func GetRepoTopLevelAt(dir string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	cmd.Dir = dir
	output, err := cmd.Output()
	if err != nil {
		return "", ErrNotInGitRepo
	}
	return strings.TrimSpace(string(output)), nil
}

// GetRepoIdentity returns a stable repository identity for the current repo.
// Format: git-common-dir:<absolute-common-dir>
func GetRepoIdentity() (string, error) {
	commonDir, err := GetGitCommonDir()
	if err != nil {
		return "", err
	}
	return formatRepoIdentity(commonDir), nil
}

// GetRepoIdentityAt returns a stable repository identity for the repo at dir.
// Format: git-common-dir:<absolute-common-dir>
func GetRepoIdentityAt(dir string) (string, error) {
	commonDir, err := GetGitCommonDirAt(dir)
	if err != nil {
		return "", err
	}
	return formatRepoIdentity(commonDir), nil
}

func formatRepoIdentity(commonDir string) string {
	return "git-common-dir:" + filepath.Clean(commonDir)
}

// SanitizeRepoName sanitizes a string for use as a directory name.
func SanitizeRepoName(name string) string {
	// Replace any non-alphanumeric characters (except - and _) with _
	re := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return re.ReplaceAllString(name, "_")
}
