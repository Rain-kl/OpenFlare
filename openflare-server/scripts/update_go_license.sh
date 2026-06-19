#!/usr/bin/env bash

set -euo pipefail

MODE="write"

usage() {
  cat <<'EOF'
Usage:
  scripts/update_go_license.sh [--check]

Updates Go source files with the project SPDX license header.

Rules:
  - Files already carrying a correct SPDX header are left untouched.
  - Legacy block-comment (/* ... */) Apache headers are converted to
    the short SPDX form, preserving copyright attribution.
  - Go files without any license header receive:
      // Copyright 2026 Arctel.net
      // SPDX-License-Identifier: Apache-2.0
  - //go:build and legacy // +build constraints stay at the top.

Options:
  --check   Report files that would change and exit non-zero if any are found.
EOF
}

while (($#)); do
  case "$1" in
    --check)
      MODE="check"
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "unknown argument: $1" >&2
      usage >&2
      exit 2
      ;;
  esac
  shift
done

tmp_dir="$(mktemp -d)"
trap 'rm -rf "$tmp_dir"' EXIT

changed=0

find_go_files() {
  find . \
    \( -path './.*' \
    -o -path './*-source' \
    -o -path './docs' \
    -o -path './frontend/node_modules' \
    -o -path './frontend/.next' \
    -o -path './frontend/out' \
    -o -path './internal/router/dist' \
    -o -path './internal/router/root/dist' \
    -o -path './vendor' \) -prune \
    -o -type f -name '*.go' -print
}

process_file() {
  local file="$1"
  local out="$2"

  perl -0 - "$file" "$out" <<'PERL'
use strict;
use warnings;

my ($file, $out) = @ARGV;

open my $in, '<', $file or die "open $file: $!";
local $/;
my $src = <$in>;
close $in;

my $new_only_header =
    "// Copyright 2026 Arctel.net\n" .
    "// SPDX-License-Identifier: Apache-2.0";

my $modified_header =
    "// Copyright 2025 linux.do\n" .
    "// Copyright 2026 Arctel.net\n" .
    "// SPDX-License-Identifier: Apache-2.0";

my @lines = split /\n/, $src, -1;
my @prefix;
my $i = 0;

# Preserve //go:build and // +build constraint lines at the top.
if (@lines && ($lines[0] =~ m{^//go:build } || $lines[0] =~ m{^// \+build })) {
    while ($i < @lines) {
        if ($lines[$i] =~ m{^//go:build } || $lines[$i] =~ m{^// \+build }) {
            push @prefix, $lines[$i++];
            next;
        }
        if ($lines[$i] eq '') {
            push @prefix, $lines[$i++];
            last if $i >= @lines || ($lines[$i] !~ m{^//go:build } && $lines[$i] !~ m{^// \+build });
            next;
        }
        last;
    }
}

my $prefix = @prefix ? join("\n", @prefix) . "\n" : '';
my $body = join "\n", @lines[$i .. $#lines];
my $result;

# --- Case 1: already has SPDX header (correct format, leave untouched) ---
if ($body =~ m{\A(// Copyright [^\x0a]+\x0a(?:// Copyright [^\x0a]+\x0a)?// SPDX-License-Identifier: Apache-2\.0)(\x0a?)}s) {
    $result = $prefix . $body;
}
# --- Case 2: legacy block-comment header (/* ... */) ---
elsif ($body =~ m{\A(/\*.*?\*/)(\n*)}s) {
    my $block = $1;
    my $rest = substr($body, length($1) + length($2));
    $rest =~ s/\A\n+//;

    if ($block =~ /Licensed under the Apache License/ && $block =~ /Copyright /) {
        my $has_linux_do  = ($block =~ /Copyright [^\n]*linux\.do/);
        my $has_arctel    = ($block =~ /Copyright [^\n]*Arctel\.net/);
        my $has_modified  = ($block =~ /Modified by Arctel\.net/);

        if ($has_linux_do && ($has_arctel || $has_modified)) {
            $result = $prefix . $modified_header . "\n\n" . $rest;
        } elsif ($has_arctel && !$has_linux_do) {
            $result = $prefix . $new_only_header . "\n\n" . $rest;
        } elsif ($has_linux_do && !$has_arctel && !$has_modified) {
            my $h = "// Copyright 2025 linux.do\n" .
                    "// SPDX-License-Identifier: Apache-2.0";
            $result = $prefix . $h . "\n\n" . $rest;
        } else {
            $result = $prefix . $new_only_header . "\n\n" . $rest;
        }
    } else {
        # Non-Apache block comment — prepend new header.
        $result = $prefix . $new_only_header . "\n\n" . $block . "\n\n" . $rest;
    }
}
# --- Case 3: no header at all ---
else {
    $body =~ s/\A\n+//;
    $result = $prefix . $new_only_header . "\n\n" . $body;
}

open my $fh, '>', $out or die "write $out: $!";
print {$fh} $result;
close $fh;
PERL
}

while IFS= read -r file; do
  tmp_file="$tmp_dir/${file#./}"
  mkdir -p "$(dirname "$tmp_file")"
  process_file "$file" "$tmp_file"

  if ! cmp -s "$file" "$tmp_file"; then
    changed=1
    if [[ "$MODE" == "check" ]]; then
      echo "needs license update: $file"
    else
      cp "$tmp_file" "$file"
      echo "updated: $file"
    fi
  fi
done < <(find_go_files)

if [[ "$MODE" == "check" && "$changed" -ne 0 ]]; then
  exit 1
fi
