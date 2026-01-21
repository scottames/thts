package config

import (
	"bytes"
	"text/template"
	"time"
)

// CommitMessageData contains variables available for commit message templates.
type CommitMessageData struct {
	Date          time.Time // Current time, supports .Format "..."
	Repo          string    // Repository name
	Profile       string    // Active profile name
	User          string    // Username from config
	CommitMessage string    // Triggering commit message (hook only)
}

// RenderCommitMessage renders a commit message template with the given data.
// Returns the rendered message or an error if template parsing/execution fails.
func RenderCommitMessage(tmplStr string, data CommitMessageData) (string, error) {
	tmpl, err := template.New("commitMessage").Parse(tmplStr)
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}
