#!/bin/bash
set -eu
set -o pipefail

# This linter enforces error message style conventions tree-wide on all Go
# files (excluding protobuf bindings).
#
# Checks enforced:
#   1. Gerund style -- "Failed reading" not "Failed to read".
#   2. No "unable to" -- prefer "cannot" (e.g. "cannot connect").
#   3. No contractions -- "cannot" not "can't", "do not" not "don't".

echo "Checking error message style in Go files..."

RC=0

# Only check code lines -- skip Go comments (lines starting with "//").
# Comments may contain quoted words that trigger false positives.
FILTER_COMMENTS='grep -v -P "^\S+:\d+:\s*//"'

# Check 1: "Failed to <verb>" -> use gerund form instead.
#   Bad:  fmt.Errorf("Failed to read %q: %w", path, err)
#   Good: fmt.Errorf("Failed reading %q: %w", path, err)
# Exclude "failed to verify certificate" which is a Go stdlib tls error string.
OUT=$(git grep -n --untracked -P '["`][Ff]ailed to [a-z]' '*.go' ':!:*.pb.go' | eval "${FILTER_COMMENTS}" | grep -v 'failed to verify certificate' || true)
if [ -n "${OUT}" ]; then
  echo "ERROR: error messages must use gerund style, not 'Failed to <verb>'"
  echo "       e.g., use 'Failed connecting' instead of 'Failed to connect'"
  echo "${OUT}"
  echo
  RC=1
fi

# Check 2: "unable to" -> use "cannot" instead.
#   Bad:  fmt.Errorf("Unable to connect: %w", err)
#   Good: fmt.Errorf("Cannot connect: %w", err)
# shellcheck disable=SC2016 # Backticks are literal regex characters, not command substitutions.
OUT=$(git grep -n --untracked -P '["`][^"`]*[Uu]nable to [a-z]' '*.go' ':!:*.pb.go' | eval "${FILTER_COMMENTS}" || true)
if [ -n "${OUT}" ]; then
  echo "ERROR: error messages must not use 'unable to'"
  echo "       e.g., use 'cannot connect' instead of 'unable to connect'"
  echo "${OUT}"
  echo
  RC=1
fi

# Check 3: Contractions (n't) -> expand to full form.
#   Bad:  fmt.Errorf("Can't connect: %w", err)
#   Good: fmt.Errorf("Cannot connect: %w", err)
# The pattern looks for n't between matching quote characters (" or `),
# catching can't, don't, won't, isn't, couldn't, wouldn't, etc.
# shellcheck disable=SC2016 # Backticks are literal regex characters, not command substitutions.
OUT=$(git grep -n --untracked -P '["`][^"`]*n'"'"'t[^"`]*["`]' '*.go' ':!:*.pb.go' | eval "${FILTER_COMMENTS}" || true)
if [ -n "${OUT}" ]; then
  echo "ERROR: error messages must not use contractions"
  echo "       e.g., use \"cannot\" instead of \"can't\", \"do not\" instead of \"don't\""
  echo "${OUT}"
  echo
  RC=1
fi

exit "${RC}"
