# SPDX-License-Identifier: MPL-2.0
# Copyright (C) 2026 The Lemongrass Authors

# Fixed native argument probe requested for the semantic tables. This script
# never consumes corpus text and is separate from the parse-only oracle server.
$ErrorActionPreference = 'Stop'
$literal = & /usr/bin/printf '[%s]' a,b
$items = 'a','b'
$variable = & /usr/bin/printf '[%s]' $items
@{ Version = $PSVersionTable.PSVersion.ToString(); ArrayLiteral = $literal; ArrayVariable = $variable } | ConvertTo-Json -Compress
