// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
)

func FuzzParseNoPanic(f *testing.F) {
	requireOracleParser(f)
	for _, entry := range loadOracleCorpus(f) {
		f.Add(entry.Input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// Resource refusals exercise the same API as the gate. Every unexpected
		// panic remains a fuzz failure, including panics on malformed UTF-8.
		_, _ = Parse(input, Limits{MaxInput: 4096, MaxDepth: 200})
	})
}

func FuzzParseOracle(f *testing.F) {
	o := startOracle(f)
	recorded := make(map[string]bool)
	requireOracleParser(f)
	for _, entry := range loadOracleCorpus(f) {
		f.Add(entry.Input)
	}
	f.Fuzz(func(t *testing.T, input string) {
		// Keep reference work bounded too. Oversize input is covered by the
		// pure-Go target and the parser's explicit resource-limit tests.
		if len(stringToUTF16(input)) > 4096 {
			t.Skip()
		}
		result, err := Parse(input, Limits{MaxInput: 4096, MaxDepth: 200})
		var limit *ParseError
		if errors.As(err, &limit) && (limit.ID == ErrInputTooLarge || limit.ID == ErrNestingTooDeep) {
			return
		}
		response, referenceErr := o.batch([]string{input})
		if referenceErr != nil {
			t.Fatal(referenceErr)
		}
		class, detail := compareOracle(result, err, response.Results[0], o.schema)
		if response.Results[0].ReferenceException != "" && (err != nil || oracleRefusedConstruct(result)) {
			goError := ""
			if err != nil {
				class = oracleBothError
				goError = err.Error()
			} else {
				class = oracleRefusedReferenceException
			}
			recordOracleFuzzEvidence(t, map[string]any{"Input": input, "GoError": goError, "GoRefusedConstruct": err == nil, "ReferenceException": response.Results[0].ReferenceException})
		}
		if class == oraclePostParseException {
			recordOracleFuzzEvidence(t, map[string]any{"Input": input, "PostParseException": response.Results[0].ReferenceException})
		}
		if class == oracleMismatch || class == oracleUnreachable {
			t.Fatalf("oracle class %s for %q: %s", class, input, detail)
		}
		if path := os.Getenv("LGRASS_PSPARSE_ORACLE_DEFINITION_REPORT"); path != "" && class == oracleSemantic {
			for _, e := range response.Results[0].Errors {
				if recorded[e.ID] {
					continue
				}
				lines := oraclePostParseSemanticSources(result, e.ID)
				if len(lines) == 0 {
					lines = oracleUsingModuleSemanticSources(result, response.Results[0].Tree, e)
				}
				if len(lines) != 0 {
					recordOracleFuzzEvidence(t, map[string]any{"ID": e.ID, "Sources": lines, "Input": input})
					recorded[e.ID] = true
				}
			}
		}
	})
}

func recordOracleFuzzEvidence(t *testing.T, entry map[string]any) {
	t.Helper()
	path := os.Getenv("LGRASS_PSPARSE_ORACLE_DEFINITION_REPORT")
	if path == "" {
		return
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
	if err != nil {
		t.Fatal(err)
	}
	_, writeErr := file.Write(append(data, '\n'))
	closeErr := file.Close()
	if writeErr != nil {
		t.Fatal(writeErr)
	}
	if closeErr != nil {
		t.Fatal(closeErr)
	}
}
