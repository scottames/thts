package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/scottames/thts/internal/config"
	"github.com/scottames/thts/internal/git"
	"github.com/scottames/thts/internal/ui"
)

// thoughtsRepoContext describes a directory that belongs to a profile's
// thoughts repo: the repo itself, a path inside it, or a path in the git
// repository that contains it.
type thoughtsRepoContext struct {
	Profile  *config.ResolvedProfile
	RepoPath string // canonical thoughts repo path
	GitRoot  string // canonical git top-level, empty when not in git
}

// HostsOtherFiles reports whether the thoughts repo is a subdirectory of a
// larger git repository. Sync stages with `git add -A`, which then commits
// changes across the whole host repository, not only the thoughts repo.
func (c *thoughtsRepoContext) HostsOtherFiles() bool {
	return c.GitRoot != "" && c.GitRoot != c.RepoPath
}

// resolveThoughtsRepoContext returns the thoughts repo context for dir, or nil
// when dir is not part of any profile's thoughts repo.
//
// A profile matches when dir is inside its thoughts repo, or when dir is in the
// same git repository as its thoughts repo. When several profiles match,
// THTS_PROFILE wins, then the default profile, then the first name in order.
func resolveThoughtsRepoContext(cfg *config.Config, dir string) *thoughtsRepoContext {
	if cfg == nil || len(cfg.Profiles) == 0 {
		return nil
	}

	dir = canonicalDir(dir)
	var dirGitRoot string
	dirGitRootLoaded := false

	names := make([]string, 0, len(cfg.Profiles))
	for name := range cfg.Profiles {
		names = append(names, name)
	}
	slices.Sort(names)

	matches := map[string]*thoughtsRepoContext{}
	var order []string
	for _, name := range names {
		profile := cfg.Profiles[name]
		if profile == nil || profile.ThoughtsRepo == "" {
			continue
		}

		repoPath := canonicalDir(config.ExpandPath(profile.ThoughtsRepo))
		repoGitRoot := gitTopLevel(repoPath)

		matched := isWithinDir(dir, repoPath)
		if !matched && repoGitRoot != "" {
			if !dirGitRootLoaded {
				dirGitRoot = gitTopLevel(dir)
				dirGitRootLoaded = true
			}
			matched = dirGitRoot == repoGitRoot
		}
		if !matched {
			continue
		}

		matches[name] = &thoughtsRepoContext{
			Profile: &config.ResolvedProfile{
				ThoughtsRepo: profile.ThoughtsRepo,
				ReposDir:     profile.ReposDir,
				GlobalDir:    profile.GlobalDir,
				ProfileName:  name,
			},
			RepoPath: repoPath,
			GitRoot:  repoGitRoot,
		}
		order = append(order, name)
	}

	if len(order) == 0 {
		return nil
	}
	if ctx := matches[os.Getenv("THTS_PROFILE")]; ctx != nil {
		return ctx
	}
	if _, defaultName := cfg.GetDefaultProfile(); matches[defaultName] != nil {
		return matches[defaultName]
	}
	return matches[order[0]]
}

// canonicalDir returns an absolute path with symlinks resolved when possible.
func canonicalDir(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		abs = path
	}
	if resolved, err := filepath.EvalSymlinks(abs); err == nil {
		return resolved
	}
	return filepath.Clean(abs)
}

// gitTopLevel returns the canonical git top-level for dir, or "" when dir is
// not in a git repository.
func gitTopLevel(dir string) string {
	if _, err := os.Stat(dir); err != nil {
		return ""
	}
	top, err := git.GetRepoTopLevelAt(dir)
	if err != nil || top == "" {
		return ""
	}
	return canonicalDir(top)
}

// isWithinDir reports whether path equals base or is nested inside it.
func isWithinDir(path, base string) bool {
	rel, err := filepath.Rel(base, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator)))
}

// printThoughtsRepoRefusal explains why a project-only command cannot run in
// a thoughts repo. Both init and uninit would replace or remove ./thoughts,
// which here holds real thoughts content instead of symlinks.
func printThoughtsRepoRefusal(command string, ctx *thoughtsRepoContext) {
	fmt.Println(ui.ErrorF("This directory is the thoughts repo for profile %q.", ctx.Profile.ProfileName))
	fmt.Printf("  %s manages a project's thoughts/ symlinks and cannot run in the thoughts repo itself.\n", ui.Accent(command))
	fmt.Printf("  Thoughts repo: %s\n", ui.Accent(config.ContractPath(ctx.RepoPath)))
	fmt.Printf("  Use %s or %s here instead.\n", ui.Accent("thts sync"), ui.Accent("thts status"))
}
