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
parser_ids = set(reference.findall((root / "Parser.cs").read_text()))
tokenizer = (root / "tokenizer.cs").read_text()
requires_start = tokenizer.index("#region Requires")
requires_end = tokenizer.index("#endregion Requires", requires_start)
requires_refs = {}
other_tokenizer_ids = set()
for match in reference.finditer(tokenizer):
    name = match.group(1)
    if requires_start <= match.start() < requires_end:
        requires_refs.setdefault(name, []).append(f"tokenizer.cs:{tokenizer[:match.start()].count(chr(10)) + 1}")
    else:
        other_tokenizer_ids.add(name)
for filename in ("Parser.cs", "tokenizer.cs"):
    syntax.update(reference.findall((root / filename).read_text()))
ids = {}
module_ids = {}
for filename in ("SemanticChecks.cs", "SymbolResolver.cs"):
    for line, text in enumerate((root / filename).read_text().splitlines(), 1):
        for name in error_id.findall(text):
            if name not in syntax:
                ids.setdefault(name, []).append(f"{filename}:{line}")
            elif name not in parser_ids and name not in other_tokenizer_ids and name in requires_refs:
                module_ids.setdefault(name, []).append(f"{filename}:{line}")
print(json.dumps({"Commit": "bf5fc434120666e5f1a8d1dfd6ecac7efc17a4be", "IDs": ids,
                  "UsingModuleIDs": module_ids,
                  "RequiresSyntaxRefs": {name: requires_refs[name] for name in module_ids}}, indent=2, sort_keys=True))
