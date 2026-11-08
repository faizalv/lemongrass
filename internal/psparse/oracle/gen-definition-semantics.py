# SPDX-License-Identifier: MPL-2.0
# Copyright (C) 2026 The Lemongrass Authors

"""Extract definition-qualified semantic IDs from the pinned PowerShell sources."""
import json
from pathlib import Path
import re
import sys

root = Path(sys.argv[1])
reference = re.compile(r"ParserStrings\.([A-Za-z0-9_]+)")
error_id = re.compile(r"nameof\(ParserStrings\.([A-Za-z0-9_]+)\)")
syntax = set()
for filename in ("Parser.cs", "tokenizer.cs"):
    syntax.update(reference.findall((root / filename).read_text()))
ids = {}
for filename in ("SemanticChecks.cs", "SymbolResolver.cs"):
    for line, text in enumerate((root / filename).read_text().splitlines(), 1):
        for name in error_id.findall(text):
            if name not in syntax:
                ids.setdefault(name, []).append(f"{filename}:{line}")
print(json.dumps({"Commit": "bf5fc434120666e5f1a8d1dfd6ecac7efc17a4be", "IDs": ids}, indent=2, sort_keys=True))
