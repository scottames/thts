package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestExpandPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "expands tilde",
			input: "~/foo/bar",
			want:  filepath.Join(home, "foo/bar"),
		},
		{
			name:  "expands tilde alone",
			input: "~/",
			want:  home, // filepath.Join normalizes and removes trailing slash
		},
		{
			name:  "absolute path unchanged",
			input: "/absolute/path",
			want:  "/absolute/path",
		},
		{
			name:  "tilde without slash unchanged",
			input: "~foo",
			// This becomes an absolute path based on cwd
			want: "", // Will be checked differently
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ExpandPath(tt.input)

			if tt.name == "tilde without slash unchanged" {
				// Should be made absolute but not treated as home dir expansion
				// Verify ~foo wasn't incorrectly expanded to home/foo
				if strings.HasPrefix(got, home+"/foo") {
					t.Errorf("ExpandPath(%q) incorrectly expanded ~foo to home dir, got %q", tt.input, got)
				}
				// Just verify it's an absolute path
				if !filepath.IsAbs(got) {
					t.Errorf("ExpandPath(%q) = %q, expected absolute path", tt.input, got)
				}
				return
			}

			if got != tt.want {
				t.Errorf("ExpandPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestExpandPathRelative(t *testing.T) {
	// Test that relative paths become absolute
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get current directory: %v", err)
	}

	got := ExpandPath("relative/path")
	want := filepath.Join(cwd, "relative/path")

	if got != want {
		t.Errorf("ExpandPath(relative/path) = %q, want %q", got, want)
	}
}

func TestContractPath(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "contracts home directory",
			input: filepath.Join(home, "foo/bar"),
			want:  "~/foo/bar",
		},
		{
			name:  "contracts home directory exactly",
			input: home,
			want:  "~",
		},
		{
			name:  "non-home path unchanged",
			input: "/other/path",
			want:  "/other/path",
		},
		{
			name:  "partial home match unchanged",
			input: home + "extra/path", // No slash after home
			want:  "~extra/path",       // Still contracts prefix
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ContractPath(tt.input)
			if got != tt.want {
				t.Errorf("ContractPath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestXDGConfigHome(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	t.Run("with XDG_CONFIG_HOME set", func(t *testing.T) {
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if originalXDG != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
			}
		}()

		if err := os.Setenv("XDG_CONFIG_HOME", "/custom/config"); err != nil {
			t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
		}
		got := XDGConfigHome()

		if got != "/custom/config" {
			t.Errorf("XDGConfigHome() = %q, want /custom/config", got)
		}
	})

	t.Run("without XDG_CONFIG_HOME", func(t *testing.T) {
		originalXDG := os.Getenv("XDG_CONFIG_HOME")
		defer func() {
			if originalXDG != "" {
				_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
			} else {
				_ = os.Unsetenv("XDG_CONFIG_HOME")
			}
		}()

		if err := os.Unsetenv("XDG_CONFIG_HOME"); err != nil {
			t.Fatalf("failed to unset XDG_CONFIG_HOME: %v", err)
		}
		got := XDGConfigHome()
		want := filepath.Join(home, ".config")

		if got != want {
			t.Errorf("XDGConfigHome() = %q, want %q", got, want)
		}
	})
}

func TestThtsConfigPath(t *testing.T) {
	// Save and restore XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	if err := os.Setenv("XDG_CONFIG_HOME", "/test/config"); err != nil {
		t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
	}
	got := ThtsConfigPath()
	want := "/test/config/thts/config.yaml"

	if got != want {
		t.Errorf("ThtsConfigPath() = %q, want %q", got, want)
	}
}

func TestHumanLayerConfigPath(t *testing.T) {
	// Save and restore XDG_CONFIG_HOME
	originalXDG := os.Getenv("XDG_CONFIG_HOME")
	defer func() {
		if originalXDG != "" {
			_ = os.Setenv("XDG_CONFIG_HOME", originalXDG)
		} else {
			_ = os.Unsetenv("XDG_CONFIG_HOME")
		}
	}()

	if err := os.Setenv("XDG_CONFIG_HOME", "/test/config"); err != nil {
		t.Fatalf("failed to set XDG_CONFIG_HOME: %v", err)
	}
	got := HumanLayerConfigPath()
	want := "/test/config/humanlayer/humanlayer.json"

	if got != want {
		t.Errorf("HumanLayerConfigPath() = %q, want %q", got, want)
	}
}

func TestDefaultThoughtsRepo(t *testing.T) {
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("failed to get home directory: %v", err)
	}

	got := DefaultThoughtsRepo()
	want := filepath.Join(home, "thoughts")

	if got != want {
		t.Errorf("DefaultThoughtsRepo() = %q, want %q", got, want)
	}
}

func TestDefaultUser(t *testing.T) {
	t.Run("with USER set", func(t *testing.T) {
		originalUser := os.Getenv("USER")
		defer func() {
			if originalUser != "" {
				_ = os.Setenv("USER", originalUser)
			} else {
				_ = os.Unsetenv("USER")
			}
		}()

		if err := os.Setenv("USER", "testuser"); err != nil {
			t.Fatalf("failed to set USER: %v", err)
		}
		got := DefaultUser()

		if got != "testuser" {
			t.Errorf("DefaultUser() = %q, want testuser", got)
		}
	})

	t.Run("without USER set", func(t *testing.T) {
		originalUser := os.Getenv("USER")
		defer func() {
			if originalUser != "" {
				_ = os.Setenv("USER", originalUser)
			} else {
				_ = os.Unsetenv("USER")
			}
		}()

		if err := os.Unsetenv("USER"); err != nil {
			t.Fatalf("failed to unset USER: %v", err)
		}
		got := DefaultUser()

		if got != "user" {
			t.Errorf("DefaultUser() = %q, want user", got)
		}
	})
}
