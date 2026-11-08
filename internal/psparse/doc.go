// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

// Package psparse parses PowerShell into the syntax tree PowerShell itself builds. It is a port of PowerShell's
// tokenizer and parser (github.com/PowerShell/PowerShell, tag v7.6.9, MIT; see LICENSE.PowerShell), kept faithful
// because the danger gate judges what this tree says will run: a construct read differently from PowerShell is a
// way past the gate. Node and field names follow PowerShell's own, so the tree compares one to one with the
// reference parser's in the oracle tests.
//
// The grammar is pwsh 7's, a superset of Windows PowerShell 5.1: everything 7 added is a parse error in 5.1, so
// nothing runs there. Parsing stops at the first error, with no recovery tree, since PowerShell runs nothing from
// input that has a parse error. DSC `configuration` and dynamic keywords depend on modules loaded at run time and
// are reported as unsupported rather than parsed. The package knows nothing of policy; internal/guard judges.
package psparse
