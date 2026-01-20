// Package thtsfiles provides embedded agent integration files for thts.
// This package exists at the repo root to enable go:embed access to
// instructions/, templates/, and embedded/ directories.
//
// Template files in embedded/ are rendered at copy time with agent-specific data.
package thtsfiles

import (
	"bytes"
	"embed"
	"fmt"
	"regexp"
	"strings"
	"text/template"

	"github.com/scottames/thts/internal/agents"
)

// Instructions contains the shared thts-instructions.md file.
//
//go:embed instructions/thts-instructions.md
var Instructions embed.FS

// Templates contains embedded template files for thoughts/ documents.
// These are copied to thoughts/.templates/ during init.
//
//go:embed templates/*.md
var Templates embed.FS

// Settings contains embedded default settings files for agents.
// Files are named by agent type: codex.toml, opencode.json, etc.
// Claude settings are built dynamically and not embedded.
//
//go:embed settings/*
var Settings embed.FS

// OpenCodePlugins contains embedded plugin files for OpenCode.
//
//go:embed plugins/opencode/*.ts
var OpenCodePlugins embed.FS

// Defaults contains embedded default files for the thoughts repository.
// Currently includes the root README.md created during setup.
//
//go:embed defaults/*
var Defaults embed.FS

// GetDefaultSettings returns the default settings content for an agent.
// Returns empty string if no default settings exist (e.g., Claude builds dynamically).
func GetDefaultSettings(filename string) string {
	content, err := Settings.ReadFile("settings/" + filename)
	if err != nil {
		return ""
	}
	return string(content)
}

// ReadmeData holds the template data for the thoughts repo README.
type ReadmeData struct {
	Profile   string
	ReposDir  string
	GlobalDir string
}

// CategoryRow represents a category for template rendering in instructions.
type CategoryRow struct {
	Name        string // Category name, e.g., "research" or "plans/complete"
	Description string // Human-readable description
	Location    string // Path like "thoughts/shared/research/"
	Trigger     string // Optional auto-save trigger description
	Template    string // Template filename, e.g., "research.md"
}

// InstructionsData holds all data for rendering thts-instructions.md.
type InstructionsData struct {
	User       string        // Username from config
	Categories []CategoryRow // Flattened list including sub-categories
}

// GetDefaultReadme returns the default README.md content for the thoughts repository.
// It uses Go templates to replace placeholders with the provided values.
func GetDefaultReadme(data ReadmeData) (string, error) {
	content, err := Defaults.ReadFile("defaults/README.md")
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("readme").Parse(string(content))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// GetInstructions returns the rendered thts-instructions.md content.
// It executes the embedded template with the provided data.
func GetInstructions(data InstructionsData) (string, error) {
	content, err := Instructions.ReadFile("instructions/thts-instructions.md")
	if err != nil {
		return "", err
	}

	tmpl, err := template.New("instructions").Parse(string(content))
	if err != nil {
		return "", err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", err
	}

	return buf.String(), nil
}

// EmbeddedTemplates contains the consolidated template files for all agents.
// Templates are rendered at copy time with agent-specific data.
//
//go:embed embedded/**/*.tmpl
var EmbeddedTemplates embed.FS

// RenderSkill renders the skill template for a specific agent type.
// Returns the rendered markdown content.
func RenderSkill(agentType agents.AgentType, skillName string) (string, error) {
	data := agents.GetEmbedTemplateData(agentType)
	return renderTemplate("embedded/skills/"+skillName+".md.tmpl", data)
}

// RenderCommand renders a command template for a specific agent type.
// Returns the rendered markdown content, or TOML for Gemini.
func RenderCommand(agentType agents.AgentType, cmdName string) (string, error) {
	data := agents.GetEmbedTemplateData(agentType)
	content, err := renderTemplate("embedded/commands/"+cmdName+".md.tmpl", data)
	if err != nil {
		return "", err
	}

	// Convert to TOML for Gemini
	cfg := agents.GetConfig(agentType)
	if cfg != nil && cfg.CommandsFormat == "toml" {
		return convertMarkdownToTOML(content)
	}

	return content, nil
}

// RenderAgent renders an agent template for a specific agent type.
// Returns the rendered markdown content.
func RenderAgent(agentType agents.AgentType, agentName string) (string, error) {
	data := agents.GetEmbedTemplateData(agentType)
	return renderTemplate("embedded/agents/"+agentName+".md.tmpl", data)
}

// RenderHook renders a hook template for a specific agent type.
// Returns the rendered shell script content.
func RenderHook(agentType agents.AgentType, hookName string) (string, error) {
	data := agents.GetEmbedTemplateData(agentType)
	return renderTemplate("embedded/hooks/"+hookName+".sh.tmpl", data)
}

// renderTemplate reads and executes a template from the embedded FS.
func renderTemplate(path string, data agents.EmbedTemplateData) (string, error) {
	content, err := EmbeddedTemplates.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("failed to read template %s: %w", path, err)
	}

	tmpl, err := template.New(path).Parse(string(content))
	if err != nil {
		return "", fmt.Errorf("failed to parse template %s: %w", path, err)
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("failed to execute template %s: %w", path, err)
	}

	return buf.String(), nil
}

// convertMarkdownToTOML converts markdown command content to TOML format for Gemini.
// Input format:
//
//	---
//	description: Some description
//	---
//	# Title
//	Content...
//
// Output format:
//
//	description = "Some description"
//	prompt = """
//	# Title
//	Content...
//	"""
func convertMarkdownToTOML(content string) (string, error) {
	// Parse frontmatter
	if !strings.HasPrefix(content, "---") {
		return "", fmt.Errorf("expected markdown frontmatter starting with ---")
	}

	// Find end of frontmatter
	rest := content[3:]
	endIdx := strings.Index(rest, "---")
	if endIdx == -1 {
		return "", fmt.Errorf("expected closing --- for frontmatter")
	}

	frontmatter := strings.TrimSpace(rest[:endIdx])
	body := strings.TrimSpace(rest[endIdx+3:])

	// Extract description from frontmatter
	var description string
	re := regexp.MustCompile(`(?m)^description:\s*(.+)$`)
	match := re.FindStringSubmatch(frontmatter)
	if len(match) > 1 {
		description = strings.TrimSpace(match[1])
	}

	// Build TOML output
	var toml strings.Builder
	toml.WriteString(fmt.Sprintf("description = %q\n\n", description))
	toml.WriteString("prompt = \"\"\"\n")
	toml.WriteString(body)
	toml.WriteString("\n\"\"\"\n")

	return toml.String(), nil
}

// GetAvailableSkills returns the list of available skill template names.
func GetAvailableSkills() []string {
	return []string{"thts-integrate"}
}

// GetAvailableCommands returns the list of available command template names.
func GetAvailableCommands() []string {
	return []string{"thts-handoff", "thts-resume"}
}

// GetAvailableAgents returns the list of available agent template names.
func GetAvailableAgents() []string {
	return []string{"thoughts-locator", "thoughts-analyzer"}
}

// GetAvailableHooks returns the list of available hook template names.
func GetAvailableHooks() []string {
	return []string{"thts-session-start", "thts-prompt-check"}
}
