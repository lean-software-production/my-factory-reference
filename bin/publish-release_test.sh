#!/usr/bin/env bash
# Checks bin/publish-release's choice of version, without building or
# publishing anything.
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
scratch=$(mktemp -d)
trap 'rm -rf "$scratch"' EXIT

# A repository of its own, with a copy of the script and no origin.
mkdir -p "$scratch/r/bin"
cp "$repo/bin/publish-release" "$scratch/r/bin/"
git -C "$scratch/r" init -q
git -C "$scratch/r" -c user.name=t -c user.email=t@t commit -q --allow-empty -m first

next() { (cd "$scratch/r" && RELEASE_DATE="$1" bin/publish-release --next-version); }
fail() { echo "FAIL: $*" >&2; exit 1; }

[ "$(next 2026.10.27)" = 2026.10.27 ] || fail "no release today should give today's date"

git -C "$scratch/r" tag 2026.10.27
[ "$(next 2026.10.27)" = 2026.10.27.1 ] || fail "one release today should give .1"

git -C "$scratch/r" tag 2026.10.27.1
git -C "$scratch/r" tag 2026.10.27.2
[ "$(next 2026.10.27)" = 2026.10.27.3 ] || fail "three releases today should give .3"

[ "$(next 2026.10.28)" = 2026.10.28 ] || fail "another day's releases don't count"

git -C "$scratch/r" tag 2026.10.29.1
[ "$(next 2026.10.29)" = 2026.10.29 ] || fail "a free plain date is taken first"

out="$(cd "$scratch/r" && bin/publish-release a b 2>&1)" && fail "two arguments should be refused"
grep -q usage <<<"$out" || fail "two arguments should print the usage"

echo "publish-release tests passed"
