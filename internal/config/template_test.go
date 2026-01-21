package config

import (
	"strings"
	"testing"
	"time"
)

func TestRenderCommitMessage(t *testing.T) {
	fixedTime := time.Date(2024, 6, 15, 14, 30, 45, 0, time.UTC)

	tests := []struct {
		name     string
		template string
		data     CommitMessageData
		want     string
		wantErr  bool
	}{
		{
			name:     "basic repo variable",
			template: "Sync {{.Repo}}",
			data:     CommitMessageData{Repo: "my-project"},
			want:     "Sync my-project",
		},
		{
			name:     "basic user variable",
			template: "{{.User}} synced thoughts",
			data:     CommitMessageData{User: "alice"},
			want:     "alice synced thoughts",
		},
		{
			name:     "basic profile variable",
			template: "[{{.Profile}}] sync",
			data:     CommitMessageData{Profile: "work"},
			want:     "[work] sync",
		},
		{
			name:     "date with format",
			template: `Sync - {{.Date.Format "2006-01-02"}}`,
			data:     CommitMessageData{Date: fixedTime},
			want:     "Sync - 2024-06-15",
		},
		{
			name:     "date with RFC3339 format",
			template: `{{.Date.Format "2006-01-02T15:04:05Z07:00"}}`,
			data:     CommitMessageData{Date: fixedTime},
			want:     "2024-06-15T14:30:45Z",
		},
		{
			name:     "commit message variable (hook)",
			template: "Auto-sync: {{.CommitMessage}}",
			data:     CommitMessageData{CommitMessage: "feat: add new feature"},
			want:     "Auto-sync: feat: add new feature",
		},
		{
			name:     "multiple variables",
			template: "[{{.Profile}}] {{.Repo}} - {{.User}}",
			data: CommitMessageData{
				Profile: "work",
				Repo:    "my-project",
				User:    "bob",
			},
			want: "[work] my-project - bob",
		},
		{
			name:     "all variables",
			template: `[{{.Profile}}] {{.Repo}} by {{.User}} at {{.Date.Format "15:04"}} - {{.CommitMessage}}`,
			data: CommitMessageData{
				Date:          fixedTime,
				Repo:          "thts",
				Profile:       "default",
				User:          "scta",
				CommitMessage: "fix bug",
			},
			want: "[default] thts by scta at 14:30 - fix bug",
		},
		{
			name:     "empty variables don't panic",
			template: "[{{.Profile}}] {{.Repo}}",
			data:     CommitMessageData{},
			want:     "[] ",
		},
		{
			name:     "static text only",
			template: "Simple sync message",
			data:     CommitMessageData{},
			want:     "Simple sync message",
		},
		{
			name:     "invalid template syntax",
			template: "{{.Invalid",
			data:     CommitMessageData{},
			wantErr:  true,
		},
		{
			name:     "undefined field access",
			template: "{{.NonExistent}}",
			data:     CommitMessageData{},
			wantErr:  true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := RenderCommitMessage(tt.template, tt.data)

			if tt.wantErr {
				if err == nil {
					t.Errorf("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("RenderCommitMessage() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDefaultCommitMessage(t *testing.T) {
	tmpl := DefaultCommitMessage()

	// Should contain Date.Format
	if !strings.Contains(tmpl, "{{.Date.Format") {
		t.Errorf("DefaultCommitMessage should use Date.Format, got: %s", tmpl)
	}

	// Should be renderable
	data := CommitMessageData{
		Date: time.Now(),
	}
	result, err := RenderCommitMessage(tmpl, data)
	if err != nil {
		t.Errorf("DefaultCommitMessage template failed to render: %v", err)
	}

	// Should contain "sync:"
	if !strings.Contains(result, "sync:") {
		t.Errorf("DefaultCommitMessage result should contain 'sync:', got: %s", result)
	}
}

func TestDefaultCommitMessageHook(t *testing.T) {
	tmpl := DefaultCommitMessageHook()

	// Should contain CommitMessage variable
	if !strings.Contains(tmpl, "{{.CommitMessage}}") {
		t.Errorf("DefaultCommitMessageHook should use CommitMessage, got: %s", tmpl)
	}

	// Should be renderable
	data := CommitMessageData{
		CommitMessage: "test commit",
	}
	result, err := RenderCommitMessage(tmpl, data)
	if err != nil {
		t.Errorf("DefaultCommitMessageHook template failed to render: %v", err)
	}

	// Should contain the commit message
	if !strings.Contains(result, "test commit") {
		t.Errorf("DefaultCommitMessageHook result should contain commit message, got: %s", result)
	}

	// Should contain "sync(auto)"
	if !strings.Contains(result, "sync(auto)") {
		t.Errorf("DefaultCommitMessageHook result should contain 'sync(auto)', got: %s", result)
	}
}
