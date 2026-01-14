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
- [Step 5: Create Embedded Files](#step-5-create-embedded-files)
- [Step 6: Add Embed Directives](#step-6-add-embed-directives)
- [Step 7: Update FS Switch Statements](#step-7-update-fs-switch-statements)
- [Step 8: Add Settings Template](#step-8-add-settings-template)
- [Step 9: Run Tests](#step-9-run-tests)
- [Step 10: Verify End-to-End](#step-10-verify-end-to-end)
- [AgentConfig Reference](#agentconfig-reference)
- [Common Pitfalls](#common-pitfalls)

<!-- mtoc-end -->

## Overview

Adding an agent requires changes to these files:

| File                            | Purpose                    |
| ------------------------------- | -------------------------- |
| `internal/agents/types.go`      | Agent type, label, config  |
| `embed.go`                      | Embed directives           |
| `internal/cmd/agents/init.go`   | FS switch statements       |
| `skills/{agent}/*`              | Skill files                |
| `commands/{agent}/*`            | Command files              |
| `agents/{agent}/*`              | Agent definition files     |
| `settings/{filename}`           | Default settings template  |
| `internal/agents/types_test.go` | Test cases (auto-verified) |

The `TestAgentCompleteness` test will catch most missing pieces.

## Prerequisites

Before starting, gather this information about your agent:

1. **Config directory name**: What directory does the agent use? (e.g., `.claude`)
2. **Skill file structure**: Flat files or subdirectories with SKILL.md?
3. **Settings file format**: JSON or TOML?
4. **Global config location**: Standard dotfile or XDG?
5. **Commands directory name**: "commands", "prompts", "command", or other?
6. **Commands file format**: Markdown (`.md`) or TOML (`.toml`)? (Most use markdown)
7. **Agents support**: Does the agent support sub-agents? (Leave AgentsDir empty if not)

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
    AgentTestbot  AgentType = "testbot"  // Add this
)
```

Update `AllAgentTypes()`:

```bash
grep -n 'func AllAgentTypes' internal/agents/types.go
```

```go
func AllAgentTypes() []AgentType {
    return []AgentType{AgentClaude, AgentCodex, AgentOpenCode, AgentTestbot}
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
    InstructionsFile:      "thts-instructions.md",
    IntegrationType:       "marker",
    InstructionTargetFile: "TESTBOT.md",
    SkillsDir:             "skills",
    SkillNeedsDir:         false,
    AgentsDir:             "agents",
    SupportsCommands:      true,
    CommandsDir:           "commands",
    CommandsGlobalOnly:    false,
    GlobalUsesXDG:         false,
    SettingsFile:          "config.json",
    SettingsFormat:        "json",
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
    case "testbot":              // Add this case
        return AgentTestbot, nil
    default:
        return "", fmt.Errorf("unknown agent type: %q (valid: claude, codex, opencode, testbot)", s)
    }
}
```

**Verify**: `go build ./...`

## Step 5: Create Embedded Files

Create the directory structure based on your `SkillNeedsDir` setting.

### If SkillNeedsDir is false (flat files, like testbot)

```bash
mkdir -p skills/testbot commands/testbot agents/testbot
```

### If SkillNeedsDir is true (subdirectories)

```bash
mkdir -p skills/testbot/thts-integrate commands/testbot agents/testbot
```

### Create skill file

For flat structure (`skills/testbot/thts-integrate.md`):

```markdown
---
name: thts-integrate
description: Activate thoughts/ integration for the current task.
---

# thoughts/ Integration

For this task, actively integrate with the thoughts/ directory.

Read and apply the full integration instructions:

@.testbot/thts-instructions.md

**For this task specifically:**

1. Before starting, use `thoughts-locator` to check for existing context
2. While working, note key findings worth preserving
3. After completing, write to the appropriate thoughts/ location and run `thts sync`

Now continue with the user's task, applying these integration points throughout.
```

For subdirectory structure, create `skills/testbot/thts-integrate/SKILL.md` with similar content.

### Create command files

Copy from an existing agent and adjust paths:

```bash
cp commands/claude/thts-handoff.md commands/testbot/
cp commands/claude/thts-resume.md commands/testbot/
```

### Create agent files

```bash
cp agents/claude/thoughts-locator.md agents/testbot/
cp agents/claude/thoughts-analyzer.md agents/testbot/
```

## Step 6: Add Embed Directives

Find the embed section in `embed.go`:

```bash
grep -n 'go:embed' embed.go
```

Add your directives. Pattern depends on `SkillNeedsDir`:

```go
// For flat skills (SkillNeedsDir: false)
//go:embed skills/testbot/*.md
var TestbotSkills embed.FS

// For subdirectory skills (SkillNeedsDir: true)
//go:embed skills/testbot/*/SKILL.md
var TestbotSkills embed.FS

// Commands and agents are always flat
//go:embed commands/testbot/*.md
var TestbotCommands embed.FS

//go:embed agents/testbot/*.md
var TestbotAgents embed.FS
```

**Verify**: `go build ./...`

## Step 7: Update FS Switch Statements

Find the FS functions:

```bash
grep -n 'func get.*FS' internal/cmd/agents/init.go
```

Add cases to each:

```go
func getSkillsFS(agentType agents.AgentType) fs.FS {
    switch agentType {
    case agents.AgentClaude:
        return thtsfiles.ClaudeSkills
    case agents.AgentCodex:
        return thtsfiles.CodexSkills
    case agents.AgentOpenCode:
        return thtsfiles.OpenCodeSkills
    case agents.AgentTestbot:           // Add this
        return thtsfiles.TestbotSkills
    default:
        return nil
    }
}
```

Repeat for `getAgentsFS` and `getCommandsFS`.

**Verify**: `go build ./...`

## Step 8: Add Settings Template

If your agent has a settings file, create it in the `settings/` directory. The
filename must match `SettingsFile` in your AgentConfig.

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

The settings are automatically embedded via `//go:embed settings/*` in embed.go
and looked up by filename using `GetDefaultSettings()`.

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
/path/to/thts agents init --agents testbot
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
| `InstructionsFile`      | Filename for thts instructions (empty = inline in target file)   |
| `IntegrationType`       | `"marker"` (HTML comments) or `"config"` (JSON array)            |
| `InstructionTargetFile` | File to modify for marker integration                            |
| `SkillsDir`             | Skills directory name                                            |
| `SkillNeedsDir`         | `true` = subdirs with SKILL.md, `false` = flat .md files         |
| `AgentsDir`             | Agents directory name (empty = agent doesn't support agents)     |
| `CommandsDir`           | Commands directory name                                          |
| `CommandsGlobalOnly`    | `true` = commands install globally only                          |
| `CommandsFormat`        | `"md"` (default) or `"toml"` for command file format             |
| `GlobalUsesXDG`         | `true` = uses `~/.config/` instead of `~/.`                      |
| `SettingsFile`          | Settings filename                                                |
| `SettingsFormat`        | `"json"` or `"toml"`                                             |
| `SettingsContextKey`    | JSON key for context file (e.g., `"contextFileName"` for Gemini) |

## Common Pitfalls

Before submitting:

- [ ] Added constant to `AllAgentTypes()` return value
- [ ] Added case to `ParseAgentType()` switch
- [ ] Added all three FS switch cases (`getSkillsFS`, `getCommandsFS`, `getAgentsFS`)
- [ ] Embed pattern matches `SkillNeedsDir` setting
- [ ] Copied/created all embedded files (skills, commands, agents)
- [ ] Settings file created in `settings/` matching `SettingsFile` name
- [ ] All tests pass: `go test ./...`
- [ ] Linting passes: `trunk check`
