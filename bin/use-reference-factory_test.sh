#!/usr/bin/env bash
# Tests for bin/use-reference-factory, against a scratch copy of lines/ and
# bin/ (so the real tetris/ is untouched), a scratch HOME, and a fake factory
# that writes its arguments to a file.
set -euo pipefail

repo_root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

copy="$scratch/repo"
fake_home="$scratch/home"
args_file="$scratch/args"
mkdir -p "$copy/bin" "$fake_home"
cp -R "$repo_root/lines" "$copy/lines"
cp "$repo_root/bin/use-reference-factory" "$copy/bin/"
mkdir -p "$copy/tetris"
printf 'a spec\n' > "$copy/tetris/spec.md"

cat > "$scratch/factory" <<FAKE
#!/usr/bin/env bash
printf '%s\n' "\$@" > "$args_file"
FAKE
chmod +x "$scratch/factory"

line="$copy/tetris/tetris-000/.assembly-lines/implement-with-ralph-loop"

fail() { echo "FAIL: $1" >&2; exit 1; }

# Runs the script in the scratch copy, from the scratch copy.
use_reference_factory() {
  rm -rf "$copy/tetris/tetris-000" "$args_file"
  (cd "$copy" && env -u MODEL HOME="$fake_home" FACTORY_BIN="$scratch/factory" \
    "$@" /bin/bash "$copy/bin/use-reference-factory")
}

# Defaults.
use_reference_factory >/dev/null || fail "defaults: the script failed"
expected=$(printf '%s\n' --run tetris --target tetris/tetris-000 \
  --line implement-with-ralph-loop --seed "$copy/tetris/spec.md")
[ "$(cat "$args_file")" = "$expected" ] || fail "defaults: the factory got
$(cat "$args_file")"
for machine in planner ralph; do
  [ -f "$line/machines/$machine/machine.json" ] || fail "defaults: no $machine/machine.json"
done

# With a login.
mkdir -p "$fake_home/.pi/agent"
printf '{"openai-codex":{"type":"oauth"}}\n' > "$fake_home/.pi/agent/auth.json"
use_reference_factory >/dev/null || fail "with a login: the script failed"
for machine in planner ralph; do
  file="$line/machines/$machine/auth.json"
  [ -f "$file" ] || fail "with a login: no $machine/auth.json"
  mode=$(stat -c %a "$file" 2>/dev/null || stat -f %Lp "$file")
  [ "$mode" = 600 ] || fail "with a login: $machine/auth.json is not mode 600"
done

# No login, a key in the environment.
rm -rf "$fake_home/.pi"
use_reference_factory OPENROUTER_API_KEY=test-key >/dev/null || fail "no login: the script failed"
for machine in planner ralph; do
  [ ! -e "$line/machines/$machine/auth.json" ] || fail "no login: $machine/auth.json was written"
done
[ -f "$args_file" ] || fail "no login: the factory was not called"

# MODEL replaces the model both machines name.
use_reference_factory MODEL=openrouter/some-model >/dev/null || fail "MODEL: the script failed"
for machine in planner ralph; do
  grep -q '"model": "openrouter/some-model"' "$line/machines/$machine/machine.json" \
    || fail "MODEL: $machine/machine.json does not name it"
done

printf 'use-reference-factory tests passed\n'
