#!/usr/bin/env bash
set -euo pipefail

pass() { printf 'PASS: %s\n' "$1"; }
fail() { printf 'FAIL: %s\n' "$1"; exit 1; }
require_file() { [[ -f "$1" ]] || fail "expected file: $1"; }
require_absent() { [[ ! -e "$1" ]] || fail "expected absent path: $1"; }

script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
repo_root=$(git -C "$script_dir/.." rev-parse --show-toplevel)
temp_dir=$(mktemp -d "${TMPDIR:-/tmp}/thts-opencode-verify.XXXXXX")
trap 'rm -rf "$temp_dir"' EXIT
binary="$temp_dir/thts"
go -C "$repo_root" build -o "$binary" ./cmd/thts

export HOME="$temp_dir/home"
export XDG_CONFIG_HOME="$temp_dir/config"
export XDG_STATE_HOME="$temp_dir/state"
export THTS_CONFIG_PATH="$XDG_CONFIG_HOME/thts/config.yaml"
export GIT_CONFIG_GLOBAL=/dev/null GIT_CONFIG_NOSYSTEM=1 GIT_TERMINAL_PROMPT=0
mkdir -p "$HOME" "$XDG_CONFIG_HOME" "$XDG_STATE_HOME" "$temp_dir/project"
cd "$temp_dir/project"

output=$("$binary" init agents --agents opencode --with-settings --dry-run)
[[ "$output" == *"has no thts-managed settings"* ]] || fail "dry-run omitted settings ownership"
require_absent .opencode
pass "fresh dry-run creates nothing"

mkdir -p .opencode
printf '{"model":"user/model"}\n' >.opencode/opencode.json
cp .opencode/opencode.json "$temp_dir/user-settings"
"$binary" init agents --agents opencode --with-settings >/dev/null
for path in plugins/thts-integration.ts skills/thts-integrate/SKILL.md commands/thts-handoff.md commands/thts-resume.md agents/thoughts-locator.md agents/thoughts-analyzer.md; do
  require_file ".opencode/$path"
done
cmp .opencode/opencode.json "$temp_dir/user-settings"
require_absent .opencode/AGENTS.local.md
pass "project install preserves settings and installs all resources"

"$binary" init agents --agents pi --force >/dev/null
printf 'stale opencode\n' >.opencode/plugins/thts-integration.ts
printf 'stale pi\n' >.pi/extensions/thts-integration.ts
cp .opencode/thts-manifest.json "$temp_dir/manifest"
output=$("$binary" init agents --agents opencode --refresh --dry-run)
[[ "$output" == *"Would refresh: opencode"* ]] || fail "refresh ignored agent selection"
cmp .opencode/thts-manifest.json "$temp_dir/manifest"
[[ $(<.opencode/plugins/thts-integration.ts) == "stale opencode" ]] || fail "dry-run changed plugin"
"$binary" init agents --agents opencode --refresh >/dev/null
cmp .opencode/plugins/thts-integration.ts "$repo_root/embedded/plugins/opencode/thts-integration.ts"
[[ $(<.pi/extensions/thts-integration.ts) == "stale pi" ]] || fail "refresh changed another agent"
pass "targeted refresh and refresh dry-run honor selection"

"$binary" init agents --agents pi --global=all >/dev/null
"$binary" init agents --agents opencode --global=all >/dev/null
global="$XDG_CONFIG_HOME/opencode"
require_file "$global/plugins/thts-integration.ts"
require_absent "$global/opencode.json"
"$binary" init agents --agents opencode --refresh >/dev/null
require_absent .opencode/plugins/thts-integration.ts
require_file "$global/plugins/thts-integration.ts"
pass "global transition removes the owned project plugin"

printf 'unrelated plugin\n' >"$global/plugins/user.ts"
"$binary" uninit agents --agents opencode --global --force >/dev/null
require_absent "$global/plugins/thts-integration.ts"
require_file "$global/plugins/user.ts"
require_file "$HOME/.pi/agent/extensions/thts-integration.ts"
pass "global removal preserves unrelated files and other agents"

"$binary" uninit agents --agents opencode --force >/dev/null
require_absent .opencode/thts-manifest.json
require_absent .opencode/skills/thts-integrate/SKILL.md
cmp .opencode/opencode.json "$temp_dir/user-settings"
require_file .pi/extensions/thts-integration.ts
pass "project removal preserves user settings and other agents"
