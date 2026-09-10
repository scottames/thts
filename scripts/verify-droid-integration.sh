#!/usr/bin/env bash
set -euo pipefail

pass() { printf '\033[0;32mPASS\033[0m: %s\n' "$1"; }
fail() { printf '\033[0;31mFAIL\033[0m: %s\n' "$1"; exit 1; }
info() { printf '\033[1;33mTEST\033[0m: %s\n' "$1"; }

require_file() {
  [[ -f "$1" ]] || fail "expected file: $1"
}

require_absent() {
  [[ ! -e "$1" ]] || fail "expected absent path: $1"
}

require_contains() {
  grep -Fq -- "$2" "$1" || fail "expected $1 to contain: $2"
}

require_empty_dir() {
  local entries
  shopt -s dotglob nullglob
  entries=("$1"/*)
  (( ${#entries[@]} == 0 )) || fail "expected empty directory: $1"
}

require_hook_count() {
  local file=$1
  local command=$2
  local expected=$3
  local count
  count=$(jq --arg command "$command" '[to_entries[] | select(.key == "SessionStart" or .key == "UserPromptSubmit") | .value[]? | .hooks[]? | select(.command == $command)] | length' "$file")
  [[ "$count" == "$expected" ]] || fail "expected $expected registrations for $command in $file, found $count"
}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir/.." rev-parse --show-toplevel)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/thts-droid-verify.XXXXXX")
trap 'rm -rf "$temp_dir"' EXIT

binary="$temp_dir/thts"
go -C "$repo_root" build -o "$binary" ./cmd/thts

export HOME="$temp_dir/home"
export XDG_CONFIG_HOME="$temp_dir/config"
export XDG_STATE_HOME="$temp_dir/state"
export THTS_CONFIG_PATH="$XDG_CONFIG_HOME/thts/config.yaml"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0

mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME"

dry_run_project="$temp_dir/projects/dry-run"
hook_project="$temp_dir/projects/hook"
global_project="$temp_dir/projects/global"
mkdir -p "$dry_run_project" "$hook_project" "$global_project"

info "project and global dry-runs are isolated"
output=$(cd "$dry_run_project" && "$binary" init agents --agents droid --dry-run)
[[ "$output" == *"Droid CLI"* ]] || fail "project dry-run omitted Droid CLI label"
[[ "$output" == *"hooks.json (hooks config)"* ]] || fail "project dry-run omitted standalone hooks.json"
require_empty_dir "$dry_run_project"
output=$(cd "$dry_run_project" && "$binary" init agents --agents droid --global=all --dry-run)
[[ "$output" == *"$HOME/.factory"* || "$output" == *"~/.factory"* ]] || fail "global dry-run omitted Factory root"
require_empty_dir "$HOME"
require_empty_dir "$XDG_CONFIG_HOME"
require_empty_dir "$XDG_STATE_HOME"
pass "project and global dry-runs are isolated"

info "project hook lifecycle preserves Factory configuration"
factory_dir="$hook_project/.factory"
mkdir -p "$factory_dir/droids"
cat >"$factory_dir/hooks.json" <<'JSON'
{
  "SessionStart": [
    {"hooks": [{"type": "command", "command": "./user-session-hook.sh"}]}
  ],
  "CustomEvent": [
    {"hooks": [{"type": "command", "command": "./custom-event.sh"}]}
  ],
  "customKey": {"preserve": true}
}
JSON
printf '{"theme":"dark"}\n' >"$factory_dir/settings.json"
cp "$factory_dir/settings.json" "$temp_dir/project-settings.expected"
printf 'user droid\n' >"$factory_dir/droids/user-droid.md"

cd "$hook_project"
"$binary" init agents --agents droid --with-settings >/dev/null
project_session='"$FACTORY_PROJECT_DIR"/.factory/hooks/thts-session-start.sh'
project_prompt='"$FACTORY_PROJECT_DIR"/.factory/hooks/thts-prompt-check.sh'
for relative in \
  skills/thts-integrate/SKILL.md \
  commands/thts-handoff.md \
  commands/thts-resume.md \
  droids/thoughts-locator.md \
  droids/thoughts-analyzer.md \
  hooks/thts-session-start.sh \
  hooks/thts-prompt-check.sh \
  hooks.json \
  thts-manifest.json; do
  require_file "$factory_dir/$relative"
done
require_contains "$factory_dir/droids/thoughts-locator.md" "tools: read-only"
require_contains "$factory_dir/hooks.json" '"CustomEvent"'
require_contains "$factory_dir/hooks.json" '"customKey"'
require_hook_count "$factory_dir/hooks.json" "$project_session" 1
require_hook_count "$factory_dir/hooks.json" "$project_prompt" 1
require_contains "$factory_dir/thts-manifest.json" '"settingsFile": "hooks.json"'
if grep -Fq '"hooks.json"' <(jq -r '.files[]' "$factory_dir/thts-manifest.json"); then
  fail "project manifest owns hooks.json as a file"
fi
cmp -s "$factory_dir/settings.json" "$temp_dir/project-settings.expected" || fail "project settings.json was modified"

printf 'stale hook\n' >"$factory_dir/hooks/thts-session-start.sh"
jq 'del(.UserPromptSubmit)' "$factory_dir/hooks.json" >"$temp_dir/hooks.refresh.json"
mv "$temp_dir/hooks.refresh.json" "$factory_dir/hooks.json"
"$binary" init agents --agents droid --refresh >/dev/null
require_contains "$factory_dir/hooks/thts-session-start.sh" "# thts session start hook"
[[ -x "$factory_dir/hooks/thts-session-start.sh" ]] || fail "refreshed project hook is not executable"
require_hook_count "$factory_dir/hooks.json" "$project_session" 1
require_hook_count "$factory_dir/hooks.json" "$project_prompt" 1

"$binary" uninit agents --agents droid --force >/dev/null
require_absent "$factory_dir/skills/thts-integrate/SKILL.md"
require_absent "$factory_dir/commands/thts-handoff.md"
require_absent "$factory_dir/droids/thoughts-locator.md"
require_absent "$factory_dir/hooks/thts-session-start.sh"
require_absent "$factory_dir/thts-manifest.json"
require_file "$factory_dir/droids/user-droid.md"
require_file "$factory_dir/hooks.json"
require_contains "$factory_dir/hooks.json" '"CustomEvent"'
require_contains "$factory_dir/hooks.json" '"customKey"'
require_hook_count "$factory_dir/hooks.json" "$project_session" 0
require_hook_count "$factory_dir/hooks.json" "$project_prompt" 0
cmp -s "$factory_dir/settings.json" "$temp_dir/project-settings.expected" || fail "project settings.json changed during uninit"
pass "project hook lifecycle preserves Factory configuration"

info "targeted global lifecycle stays under the isolated Factory root"
global_factory="$HOME/.factory"
mkdir -p "$global_factory/droids"
cat >"$global_factory/hooks.json" <<'JSON'
{
  "SessionStart": [
    {"hooks": [{"type": "command", "command": "/user/global-hook.sh"}]}
  ]
}
JSON
printf '{"theme":"dark"}\n' >"$global_factory/settings.json"
cp "$global_factory/settings.json" "$temp_dir/global-settings.expected"
printf 'user global droid\n' >"$global_factory/droids/user-droid.md"

cd "$global_project"
"$binary" init agents --agents droid --global=all >/dev/null
global_session="$global_factory/hooks/thts-session-start.sh"
global_prompt="$global_factory/hooks/thts-prompt-check.sh"
for relative in \
  skills/thts-integrate/SKILL.md \
  commands/thts-handoff.md \
  commands/thts-resume.md \
  droids/thoughts-locator.md \
  droids/thoughts-analyzer.md \
  hooks/thts-session-start.sh \
  hooks/thts-prompt-check.sh \
  hooks.json; do
  require_file "$global_factory/$relative"
done
require_hook_count "$global_factory/hooks.json" "$global_session" 1
require_hook_count "$global_factory/hooks.json" "$global_prompt" 1
require_contains "$XDG_STATE_HOME/thts/global-manifest.json" "$global_factory/hooks.json"
require_absent "$global_project/.factory"
cmp -s "$global_factory/settings.json" "$temp_dir/global-settings.expected" || fail "global settings.json was modified"

"$binary" uninit agents --agents droid --global --force >/dev/null
require_absent "$global_factory/skills/thts-integrate/SKILL.md"
require_absent "$global_factory/commands/thts-handoff.md"
require_absent "$global_factory/droids/thoughts-locator.md"
require_absent "$global_factory/hooks/thts-session-start.sh"
require_file "$global_factory/droids/user-droid.md"
require_file "$global_factory/hooks.json"
require_contains "$global_factory/hooks.json" "/user/global-hook.sh"
require_hook_count "$global_factory/hooks.json" "$global_session" 0
require_hook_count "$global_factory/hooks.json" "$global_prompt" 0
cmp -s "$global_factory/settings.json" "$temp_dir/global-settings.expected" || fail "global settings.json changed during uninit"
pass "targeted global lifecycle stays under the isolated Factory root"
