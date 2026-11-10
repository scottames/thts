package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/scottames/thts/internal/config"
)

func initTestGitRepo(t *testing.T, dir string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("git", "init", "-q")
	cmd.Dir = dir
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init %s: %v\n%s", dir, err, out)
	}
}

func mkdirAll(t *testing.T, dir string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0755); err != nil {
		t.Fatal(err)
	}
	return dir
}

func profileCfg(profiles map[string]string, defaultName string) *config.Config {
	cfg := &config.Config{User: "testuser", Profiles: map[string]*config.ProfileConfig{}}
	for name, repo := range profiles {
		cfg.Profiles[name] = &config.ProfileConfig{
			ThoughtsRepo: repo,
			ReposDir:     "repos",
			GlobalDir:    "global",
			Default:      name == defaultName,
		}
	}
	return cfg
}

func TestResolveThoughtsRepoContext(t *testing.T) {
	t.Setenv("THTS_PROFILE", "")

	t.Run("standalone thoughts repo root", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "thoughts")
		initTestGitRepo(t, repo)
		cfg := profileCfg(map[string]string{"personal": repo}, "personal")

		ctx := resolveThoughtsRepoContext(cfg, repo)
		if ctx == nil {
			t.Fatal("expected context for thoughts repo root")
		}
		if ctx.Profile.ProfileName != "personal" {
			t.Errorf("profile = %q, want personal", ctx.Profile.ProfileName)
		}
		if ctx.HostsOtherFiles() {
			t.Error("standalone thoughts repo should not host other files")
		}
	})

	t.Run("inside thoughts repo", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "thoughts")
		initTestGitRepo(t, repo)
		sub := mkdirAll(t, filepath.Join(repo, "repos", "project", "shared"))
		cfg := profileCfg(map[string]string{"personal": repo}, "personal")

		if ctx := resolveThoughtsRepoContext(cfg, sub); ctx == nil {
			t.Fatal("expected context inside thoughts repo")
		}
	})

	t.Run("thoughts repo is a subdirectory of host repo", func(t *testing.T) {
		root := t.TempDir()
		host := filepath.Join(root, "notes")
		initTestGitRepo(t, host)
		repo := mkdirAll(t, filepath.Join(host, "thoughts"))
		daily := mkdirAll(t, filepath.Join(host, "Daily"))
		cfg := profileCfg(map[string]string{"work": repo}, "work")

		for _, dir := range []string{host, daily} {
			ctx := resolveThoughtsRepoContext(cfg, dir)
			if ctx == nil {
				t.Fatalf("expected context for %s", dir)
			}
			if !ctx.HostsOtherFiles() {
				t.Errorf("expected HostsOtherFiles for %s", dir)
			}
			if ctx.GitRoot != canonicalDir(host) {
				t.Errorf("GitRoot = %q, want %q", ctx.GitRoot, canonicalDir(host))
			}
		}
	})

	t.Run("nested separate git repo does not match", func(t *testing.T) {
		root := t.TempDir()
		host := filepath.Join(root, "notes")
		initTestGitRepo(t, host)
		repo := mkdirAll(t, filepath.Join(host, "thoughts"))
		nested := filepath.Join(host, "vendor", "other")
		initTestGitRepo(t, nested)
		cfg := profileCfg(map[string]string{"work": repo}, "work")

		if ctx := resolveThoughtsRepoContext(cfg, nested); ctx != nil {
			t.Fatalf("nested repo should not match, got profile %q", ctx.Profile.ProfileName)
		}
	})

	t.Run("project repo does not match", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "thoughts")
		initTestGitRepo(t, repo)
		project := filepath.Join(root, "project")
		initTestGitRepo(t, project)
		cfg := profileCfg(map[string]string{"personal": repo}, "personal")

		if ctx := resolveThoughtsRepoContext(cfg, project); ctx != nil {
			t.Fatal("project repo should not match")
		}
	})

	t.Run("non-git directory outside repos does not match", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "thoughts")
		initTestGitRepo(t, repo)
		other := mkdirAll(t, filepath.Join(root, "elsewhere"))
		cfg := profileCfg(map[string]string{"personal": repo}, "personal")

		if ctx := resolveThoughtsRepoContext(cfg, other); ctx != nil {
			t.Fatal("unrelated directory should not match")
		}
	})

	t.Run("several matches prefer THTS_PROFILE then default", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "thoughts")
		initTestGitRepo(t, repo)
		cfg := profileCfg(map[string]string{"a": repo, "b": repo, "c": repo}, "c")

		if ctx := resolveThoughtsRepoContext(cfg, repo); ctx.Profile.ProfileName != "c" {
			t.Errorf("profile = %q, want default c", ctx.Profile.ProfileName)
		}

		t.Setenv("THTS_PROFILE", "b")
		if ctx := resolveThoughtsRepoContext(cfg, repo); ctx.Profile.ProfileName != "b" {
			t.Errorf("profile = %q, want THTS_PROFILE b", ctx.Profile.ProfileName)
		}

		t.Setenv("THTS_PROFILE", "")
		delete(cfg.Profiles, "c")
		if ctx := resolveThoughtsRepoContext(cfg, repo); ctx.Profile.ProfileName != "a" {
			t.Errorf("profile = %q, want first sorted a", ctx.Profile.ProfileName)
		}
	})

	t.Run("symlinked paths resolve", func(t *testing.T) {
		root := t.TempDir()
		repo := filepath.Join(root, "real", "thoughts")
		initTestGitRepo(t, repo)
		link := filepath.Join(root, "link")
		if err := os.Symlink(filepath.Join(root, "real"), link); err != nil {
			t.Fatal(err)
		}
		cfg := profileCfg(map[string]string{"personal": filepath.Join(link, "thoughts")}, "personal")

		if ctx := resolveThoughtsRepoContext(cfg, repo); ctx == nil {
			t.Fatal("expected match through symlinked profile path")
		}
		if ctx := resolveThoughtsRepoContext(cfg, filepath.Join(link, "thoughts")); ctx == nil {
			t.Fatal("expected match through symlinked cwd")
		}
	})

	t.Run("nil and empty config", func(t *testing.T) {
		if resolveThoughtsRepoContext(nil, t.TempDir()) != nil {
			t.Error("nil config should not match")
		}
		if resolveThoughtsRepoContext(&config.Config{}, t.TempDir()) != nil {
			t.Error("empty config should not match")
		}
	})
}

func TestThoughtsRepoContextEditAndAdd(t *testing.T) {
	t.Setenv("THTS_PROFILE", "")

	root := t.TempDir()
	host := filepath.Join(root, "notes")
	initTestGitRepo(t, host)
	repo := mkdirAll(t, filepath.Join(host, "thoughts"))
	globalDir := mkdirAll(t, filepath.Join(repo, "global"))
	defaultRepo := filepath.Join(root, "default-thoughts")
	initTestGitRepo(t, defaultRepo)
	mkdirAll(t, filepath.Join(defaultRepo, "global"))

	cfg := profileCfg(map[string]string{"default": defaultRepo, "work": repo}, "default")
	t.Chdir(host)

	t.Run("edit opens matching thoughts repo", func(t *testing.T) {
		path, err := resolveEditPath(cfg, "")
		if err != nil {
			t.Fatalf("resolveEditPath() error = %v", err)
		}
		if path != canonicalDir(repo) {
			t.Errorf("resolveEditPath() = %q, want %q", path, canonicalDir(repo))
		}
	})

	t.Run("add targets matching profile global dir", func(t *testing.T) {
		dir, isGlobal, err := resolveThoughtsDir(cfg, &AddOptions{})
		if err != nil {
			t.Fatalf("resolveThoughtsDir() error = %v", err)
		}
		if !isGlobal || canonicalDir(dir) != canonicalDir(globalDir) {
			t.Errorf("resolveThoughtsDir() = %q (global=%v), want %q", dir, isGlobal, globalDir)
		}

		ctx, err := resolveSyncContext(cfg, &AddOptions{})
		if err != nil {
			t.Fatalf("resolveSyncContext() error = %v", err)
		}
		if ctx.ProfileName != "work" || ctx.RepoPath != canonicalDir(repo) {
			t.Errorf("resolveSyncContext() = %+v, want work at %q", ctx, canonicalDir(repo))
		}
	})
}
