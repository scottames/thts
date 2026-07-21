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

require_only_files() {
  local root=$1
  shift
  local path relative
  local -a actual=()
  local -A expected=()
  for relative in "$@"; do
    expected["$relative"]=1
  done

  shopt -s globstar nullglob
  for path in "$root"/**/*; do
    [[ -f "$path" ]] || continue
    relative=${path#"$root"/}
    [[ ${expected["$relative"]+present} ]] || fail "unexpected file: $path"
    actual+=("$relative")
  done
  (( ${#actual[@]} == $# )) || fail "expected $# files under $root, found ${#actual[@]}"
}

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir/.." rev-parse --show-toplevel)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/thts-pi-verify.XXXXXX")
trap 'rm -rf "$temp_dir"' EXIT

binary="$temp_dir/thts"
go -C "$repo_root" build -o "$binary" ./cmd/thts

export HOME="$temp_dir/home"
export XDG_CONFIG_HOME="$temp_dir/config"
export XDG_STATE_HOME="$temp_dir/state"
export THTS_CONFIG_PATH="$XDG_CONFIG_HOME/thts/config.yaml"
export PI_CODING_AGENT_DIR="$temp_dir/pi"
export GIT_CONFIG_GLOBAL=/dev/null
export GIT_CONFIG_NOSYSTEM=1
export GIT_TERMINAL_PROMPT=0

mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME"

dry_run_project="$temp_dir/projects/dry-run"
hook_project="$temp_dir/projects/hook"
global_project="$temp_dir/projects/global"
mkdir -p "$dry_run_project" "$hook_project" "$global_project"

info "project dry-run creates no files"
output=$(cd "$dry_run_project" && "$binary" init agents --agents pi --dry-run)
[[ "$output" == *"Dry run complete. No files were created."* ]] || fail "unexpected dry-run output: $output"
require_empty_dir "$dry_run_project"
require_empty_dir "$HOME"
require_empty_dir "$XDG_CONFIG_HOME"
require_empty_dir "$XDG_STATE_HOME"
require_absent "$PI_CODING_AGENT_DIR"
pass "project dry-run creates no files"

info "project hook install creates only Pi-managed resources"
cd "$hook_project"
"$binary" init agents --agents pi >/dev/null
pi_dir="$hook_project/.pi"
require_file "$pi_dir/skills/thts-integrate/SKILL.md"
require_file "$pi_dir/prompts/thts-handoff.md"
require_file "$pi_dir/prompts/thts-resume.md"
require_file "$pi_dir/extensions/thts-integration.ts"
require_file "$pi_dir/thts-manifest.json"
require_contains "$pi_dir/thts-manifest.json" "skills/thts-integrate/SKILL.md"
require_contains "$pi_dir/thts-manifest.json" "prompts/thts-handoff.md"
require_contains "$pi_dir/thts-manifest.json" "prompts/thts-resume.md"
require_contains "$pi_dir/thts-manifest.json" "extensions/thts-integration.ts"
require_absent "$pi_dir/agents"
require_absent "$pi_dir/settings.json"
require_absent "$hook_project/AGENTS.md"
pass "project hook install creates the expected Pi resources"

info "refresh replaces a stale Pi extension"
printf 'stale extension\n' >"$pi_dir/extensions/thts-integration.ts"
output=$("$binary" init agents --agents pi --refresh)
[[ "$output" == *"Updated 1 extension(s)"* ]] || fail "unexpected refresh output: $output"
require_contains "$pi_dir/extensions/thts-integration.ts" "before_agent_start"
pass "refresh replaces a stale Pi extension"

info "forced project uninit preserves unrelated Pi files"
printf 'user extension\n' >"$pi_dir/extensions/user-extension.ts"
"$binary" uninit agents --agents pi --force >/dev/null
require_absent "$pi_dir/skills/thts-integrate/SKILL.md"
require_absent "$pi_dir/prompts/thts-handoff.md"
require_absent "$pi_dir/prompts/thts-resume.md"
require_absent "$pi_dir/extensions/thts-integration.ts"
require_absent "$pi_dir/thts-manifest.json"
require_file "$pi_dir/extensions/user-extension.ts"
pass "forced project uninit preserves unrelated Pi files"

info "targeted global install creates only Pi resources"
cd "$global_project"
"$binary" init agents --agents pi --global=all >/dev/null
global_files=(
  "skills/thts-integrate/SKILL.md"
  "prompts/thts-handoff.md"
  "prompts/thts-resume.md"
  "extensions/thts-integration.ts"
)
for relative in "${global_files[@]}"; do
  require_file "$PI_CODING_AGENT_DIR/$relative"
done
require_only_files "$PI_CODING_AGENT_DIR" "${global_files[@]}"
require_file "$XDG_STATE_HOME/thts/global-manifest.json"
require_absent "$PI_CODING_AGENT_DIR/agents"
require_absent "$PI_CODING_AGENT_DIR/settings.json"
require_absent "$global_project/.pi"
pass "targeted global install creates only Pi resources"

info "forced global uninit removes managed files and preserves unrelated Pi files"
printf 'user extension\n' >"$PI_CODING_AGENT_DIR/extensions/user-extension.ts"
"$binary" uninit agents --agents pi --global --force >/dev/null
for relative in "${global_files[@]}"; do
  require_absent "$PI_CODING_AGENT_DIR/$relative"
done
require_absent "$XDG_STATE_HOME/thts/global-manifest.json"
require_file "$PI_CODING_AGENT_DIR/extensions/user-extension.ts"
pass "forced global uninit removes managed files and preserves unrelated Pi files"
