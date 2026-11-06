#!/bin/sh
# SPDX-License-Identifier: AGPL-3.0-only
# Copyright (C) 2026 The Lemongrass Authors

# Adds the license header to every tracked source file that lacks one. With --check it only lists those files and fails.
# git ls-files does not descend into the bibliothek submodule, which is a separate repository with its own license.
# Files under sankasten/ are the Sankasten design system, Apache-2.0 under its own holder, and the check fails one
# that carries any other identifier, so a file moved in from the app is restamped rather than kept AGPL.
set -eu

LICENSE_ID="AGPL-3.0-only"
HOLDER="The Lemongrass Authors"
SANKASTEN_LICENSE_ID="Apache-2.0"
SANKASTEN_HOLDER="The Sankasten Authors"

cd "$(dirname "$0")/.."

check=0
[ "${1:-}" = "--check" ] && check=1

year=$(date +%Y)
missing=0
IFS='
'
for file in $(git ls-files -- '*.go' '*.ts' '*.vue' '*.mjs' '*.css' '*.sh'); do
	case "$file" in
	sankasten/*) id="$SANKASTEN_LICENSE_ID" holder="$SANKASTEN_HOLDER" ;;
	*) id="$LICENSE_ID" holder="$HOLDER" ;;
	esac

	# Only the top of the file counts, so a file that merely mentions the identifier still gets stamped.
	top=$(head -n 3 "$file")
	if printf '%s\n' "$top" | grep -q 'SPDX-License-Identifier'; then
		case "$file" in
		sankasten/*)
			printf '%s\n' "$top" | grep -q "SPDX-License-Identifier: $id" && continue
			missing=$((missing + 1))
			if [ "$check" -eq 1 ]; then
				echo "$file: not $id"
				continue
			fi
			# Restamp in place: the identifier and the holder change, the comment form and the year stay.
			restamped=$(mktemp)
			sed -E "1,3{s/SPDX-License-Identifier: [^ ]+/SPDX-License-Identifier: $id/;s/(Copyright \(C\) [0-9]+) .*[A-Za-z]/\1 $holder/;}" "$file" >"$restamped"
			cat "$restamped" >"$file"
			rm -f "$restamped"
			continue
			;;
		*) continue ;;
		esac
	fi
	missing=$((missing + 1))
	if [ "$check" -eq 1 ]; then
		echo "$file"
		continue
	fi

	case "$file" in
	*.vue) header="<!-- SPDX-License-Identifier: $id -->\\n<!-- Copyright (C) $year $holder -->" ;;
	*.css) header="/* SPDX-License-Identifier: $id */\\n/* Copyright (C) $year $holder */" ;;
	*.sh) header="# SPDX-License-Identifier: $id\\n# Copyright (C) $year $holder" ;;
	*) header="// SPDX-License-Identifier: $id\\n// Copyright (C) $year $holder" ;;
	esac

	# The blank line after the header keeps Go from reading it as the package doc comment. A shebang must stay first.
	stamped=$(mktemp)
	awk -v header="$header" '
		NR == 1 && /^#!/ { print; print header; print ""; next }
		NR == 1 { print header; print "" }
		{ print }
		END { if (NR == 0) print header }
	' "$file" >"$stamped"
	# Writing through cat keeps the file mode, so the shell scripts stay executable.
	cat "$stamped" >"$file"
	rm -f "$stamped"
done

if [ "$check" -eq 1 ] && [ "$missing" -gt 0 ]; then
	echo "license-headers: $missing file(s) above have no license header or the wrong one. Run make license." >&2
	exit 1
fi
[ "$check" -eq 1 ] || echo "license-headers: stamped $missing file(s)."
