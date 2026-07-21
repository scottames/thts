# Adding a New Agent to thts

This guide walks through adding support for a new AI coding agent. We'll use a
fictional "testbot" agent as a concrete example.

<!-- mtoc-start -->

- [Overview](#overview)
- [Prerequisites](#prerequisites)
- [Step 1: Register the Agent Type](#step-1-register-the-agent-type)
- [Step 2: Add Agent Label](#step-2-add-agent-label)
- [Step 3: Add Agent Configuration](#step-3-add-agent-configuration)
- [Step 4: Update ParseAgentType](#step-4-update-parseagenttype)
- [Step 5: Configure Embedded Templates](#step-5-configure-embedded-templates)
- [Step 6: Add Embed Directives](#step-6-add-embed-directives)
- [Step 7: Add Integration Adapters](#step-7-add-integration-adapters)
- [Step 8: Add Settings Template](#step-8-add-settings-template)
- [Step 9: Run Tests](#step-9-run-tests)
- [Step 10: Verify End-to-End](#step-10-verify-end-to-end)
- [AgentConfig Reference](#agentconfig-reference)
- [Global Component Ownership](#global-component-ownership)
- [Common Pitfalls](#common-pitfalls)

<!-- mtoc-end -->

## Overview

Adding an agent requires changes to these files:

| File                            | Purpose                             |
| ------------------------------- | ----------------------------------- |
| `internal/agents/types.go`      | Agent type, capabilities, and paths |
| `internal/agents/template.go`   | Agent-specific template data        |
| `embed.go`                      | Shared embed and render functions   |
| `embedded/skills/*.tmpl`        | Shared skill templates              |
| `embedded/commands/*.tmpl`      | Shared command templates            |
| `embedded/agents/*.tmpl`        | Shared agent templates              |
| `embedded/plugins/{agent}/`     | Optional native runtime adapters    |
| `embedded/settings/{filename}`  | Optional default settings template  |
| `internal/agents/types_test.go` | Completeness tests                  |

The `TestAgentCompleteness` test will catch most missing pieces.

## Prerequisites

Before starting, gather this information about your agent:

1. **Config directory name**: What directory does the agent use? (e.g., `.claude`)
2. **Skill file structure**: Flat files or subdirectories with SKILL.md?
3. **Settings file format**: JSON or TOML? Does thts own a safe default?
4. **Global config location**: Standard dotfile or XDG?
5. **Commands directory name**: "commands", "prompts", "command", or other?
6. **Commands file format**: Markdown (`.md`) or TOML (`.toml`)? (Most use markdown)
7. **Agents support**: Does the agent support native sub-agents? (Leave
   `AgentsDir` empty if not.)
8. **Runtime adapter**: Does it use shell hooks, a plugin, an extension, or no
   native adapter? Pi uses an `extensions/` directory.

For testbot, we'll assume:

- Config directory: `.testbot`
- Skills: Flat files (like Claude)
- Settings: `config.json` (JSON format)
- Global: Standard dotfile (`~/.testbot/`)
- Commands: `commands/`

## Step 1: Register the Agent Type

Find the agent constants:

```bash
grep -n 'AgentType = "' internal/agents/types.go
```

Add your constant in the `const` block:

```go
const (
    AgentClaude   AgentType = "claude"
    AgentCodex    AgentType = "codex"
    AgentOpenCode AgentType = "opencode"
    AgentGemini   AgentType = "gemini"
    AgentPi       AgentType = "pi"
    AgentTestbot  AgentType = "testbot"  // Add this
)
```

Update `AllAgentTypes()`:

```bash
grep -n 'func AllAgentTypes' internal/agents/types.go
```

```go
func AllAgentTypes() []AgentType {
    return []AgentType{AgentClaude, AgentCodex, AgentOpenCode, AgentGemini, AgentPi, AgentTestbot}
}
```

**Verify**: `go build ./...`

## Step 2: Add Agent Label

Find the labels map:

```bash
grep -n 'AgentTypeLabels' internal/agents/types.go
```

Add your label:

```go
var AgentTypeLabels = map[AgentType]string{
    AgentClaude:   "Claude Code",
    AgentCodex:    "OpenAI Codex CLI",
    AgentOpenCode: "OpenCode",
    AgentGemini:   "Google Gemini CLI",
    AgentPi:       "Pi",
    AgentTestbot:  "Testbot",  // Add this
}
```

## Step 3: Add Agent Configuration

Find the configs map:

```bash
grep -n 'AgentConfigs.*=.*map' internal/agents/types.go
```

Add your configuration (see [AgentConfig Reference](#agentconfig-reference) for field details):

```go
AgentTestbot: {
    Type:                  AgentTestbot,
    RootDir:               ".testbot",
    InstructionsFile:      "",
    IntegrationType:       "marker",
    InstructionTargetFile: "TESTBOT.md",
    SkillsDir:             "skills",
    SkillNeedsDir:         false,
    AgentsDir:             "agents", // Empty when native sub-agents are unsupported
    SupportsCommands:      true,
    CommandsDir:           "commands",
    CommandsGlobalOnly:    false,
    GlobalUsesXDG:         false,
    SettingsFile:          "config.json",
    SettingsTemplate:      "testbot.json", // Empty when thts must not manage settings
    SettingsFormat:        "json",
    SupportsHooks:         true,
    HooksDir:              "hooks",
},
```

## Step 4: Update ParseAgentType

Find the parse function:

```bash
grep -n 'func ParseAgentType' internal/agents/types.go
```

Add a case for your agent:

```go
func ParseAgentType(s string) (AgentType, error) {
    normalized := strings.ToLower(strings.TrimSpace(s))
    switch normalized {
    case "claude":
        return AgentClaude, nil
    case "codex":
        return AgentCodex, nil
    case "opencode":
        return AgentOpenCode, nil
    case "gemini":
        return AgentGemini, nil
    case "pi":
        return AgentPi, nil
    case "testbot":              // Add this case
        return AgentTestbot, nil
    default:
        return "", fmt.Errorf("unknown agent type: %q (valid: claude, codex, opencode, gemini, pi, testbot)", s)
    }
}
```

**Verify**: `go build ./...`

## Step 5: Configure Embedded Templates

Skills, commands or prompts, and native sub-agents use shared templates from
`embedded/`. Add the new agent's differences to `GetEmbedTemplateData()` in
`internal/agents/template.go` rather than copying per-agent files.

The `thts-integrate` skill must remain independent of the selected integration
mode. It loads the canonical policy from the CLI when the policy is not already
available:

```markdown
---
name: thts-integrate
description: Activate thoughts/ integration for the current task.
---

# thoughts/ Integration

For this task, actively integrate with the thoughts/ directory.

Run `thts init --check`, then run `thts agent-instructions` from the current
project and apply its output.

Now continue with the user's task, applying these integration points throughout.
```

Only add an agent-specific template when the shared template cannot express a
required native format.

## Step 6: Add Embed Directives

The shared templates are already covered by:

```go
//go:embed embedded/**/*.tmpl
var EmbeddedTemplates embed.FS
```

Add a dedicated embed only for native assets that are not rendered from these
templates, such as an OpenCode plugin.

## Step 7: Add Integration Adapters

Set `HooksDir` for agents using shell hooks or `PluginsDir` for a native runtime
adapter. `PluginsDir` is the existing generic field: use `plugins/` for an
OpenCode plugin and `extensions/` for a Pi extension. In user-facing output and
documentation, call a Pi runtime adapter an **extension**, not a plugin. If an
agent supports neither, hook mode automatically falls back to managed project
instructions.

Pi is the reference configuration for an extension-only agent: it uses
`.pi/skills/`, `.pi/prompts/`, and `.pi/extensions/`, with no native sub-agents
and no thts-managed settings. Do not add an empty settings template merely to
make `--with-settings` create `settings.json`.

Only native runtime-adapter assets require an FS case:

```go
func getPluginsFS(agentType agents.AgentType) fs.FS {
    switch agentType {
    case agents.AgentTestbot:
        return thtsfiles.TestbotPlugins
    default:
        return nil
    }
}
```

**Verify**: `go build ./...`

## Step 8: Add Settings Template

Create a settings template only when thts should manage a default settings file.
The template filename must match `SettingsTemplate`; `SettingsFile` remains the
destination name. Leave `SettingsTemplate` empty when the agent's settings are
user-owned, as for Pi.

For testbot with `SettingsFile: "config.json"`:

```bash
# Create settings/config.json (or whatever your SettingsFile is named)
cat > settings/config.json << 'EOF'
{
  "model": "testbot-default",
  "permissions": {
    "allow": []
  }
}
EOF
```

Settings are automatically embedded via `//go:embed settings/*` in `embed.go`
and looked up by `SettingsTemplate` using `GetDefaultSettings()`.

**Note**: Claude is special - its settings are built dynamically with user input
in `buildClaudeSettings()`. If your agent needs dynamic settings, you'll need to
add a case to `writeAgentSettings()` in init.go.

## Step 9: Run Tests

The completeness test will verify you haven't missed anything:

```bash
go test ./internal/agents/... -v
```

Run all tests:

```bash
go test ./...
```

## Step 10: Verify End-to-End

Build:

```bash
go build -o thts ./cmd/thts
```

Test initialization:

```bash
cd /tmp
mkdir test-project && cd test-project
git init
/path/to/thts init agents --agents testbot
```

Verify files were created:

```bash
ls -la .testbot/
```

## AgentConfig Reference

See the `AgentConfig` struct in `internal/agents/types.go` for the authoritative
field list. Key fields:

| Field                   | Description                                                      |
| ----------------------- | ---------------------------------------------------------------- |
| `RootDir`               | Agent's config directory (e.g., `.claude`)                       |
| `InstructionsFile`      | Legacy generated filename; empty for current agents              |
| `IntegrationType`       | `"marker"` for current managed instruction integration           |
| `InstructionTargetFile` | File to modify for marker integration                            |
| `SkillsDir`             | Skills directory name                                            |
| `SkillNeedsDir`         | `true` = subdirs with SKILL.md, `false` = flat .md files         |
| `AgentsDir`             | Agents directory name (empty = agent doesn't support agents)     |
| `CommandsDir`           | Commands directory name                                          |
| `CommandsGlobalOnly`    | `true` = commands install globally only                          |
| `CommandsFormat`        | `"md"` (default) or `"toml"` for command file format             |
| `GlobalUsesXDG`         | `true` = uses `~/.config/` instead of `~/.`                      |
| `SettingsFile`          | Settings filename                                                |
| `SettingsTemplate`      | Embedded default identity; empty when settings are user-owned    |
| `SettingsFormat`        | `"json"` or `"toml"`                                             |
| `SettingsContextKey`    | JSON key for context file (e.g., `"contextFileName"` for Gemini) |

## Global Component Ownership

Global components are owned per agent, not as one all-or-nothing switch. Record
successful paths with `GlobalManifest.RecordAgentComponent`, which replaces only
that agent's paths for a component and preserves the other agents' paths. Set
the matching `agents.perAgent.<agent>.<component>` mode only after that
agent/component pair installs successfully.

When removing global files, retain manifest ownership and the global mode for
paths that could not be removed. Reset only the successfully removed agent's
component mode; never downgrade another agent because a shared component was
selected.

## Common Pitfalls

Before submitting:

- [ ] Added constant to `AllAgentTypes()` return value
- [ ] Added case to `ParseAgentType()` switch
- [ ] Added agent-specific template data in `GetEmbedTemplateData()`
- [ ] Added native hook, plugin, or extension assets only when required
- [ ] Used extension terminology and `extensions/` for Pi-like adapters
- [ ] Left `AgentsDir` and `SettingsTemplate` empty when unsupported or user-owned
- [ ] Global manifest updates preserve other agents' component ownership
- [ ] Settings template created in `embedded/settings/` only when it matches `SettingsTemplate`
- [ ] All tests pass: `go test ./...`
- [ ] Linting passes: `trunk check`
