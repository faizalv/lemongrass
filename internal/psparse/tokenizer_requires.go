// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

// getScriptRequirements is GetScriptRequirements. Until its port lands (contract decision 11), a collected
// #requires line is refused with ErrRequiresNotSupported rather than passed unread: PowerShell checks its
// arguments and reports parse errors for them, so ignoring it would accept input PowerShell rejects.
func (t *tokenizer) getScriptRequirements() *ScriptRequirements {
	if t.requiresTokens == nil {
		return nil
	}
	tokens := *t.requiresTokens
	t.requiresTokens = nil
	if len(tokens) == 0 {
		return &ScriptRequirements{}
	}
	t.parser.reportError(tokens[0].Extent, ErrRequiresNotSupported)
	return nil
}
