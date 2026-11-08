// SPDX-License-Identifier: MPL-2.0
// Copyright (C) 2026 The Lemongrass Authors

package psparse

import "testing"

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
		response, referenceErr := o.batch([]string{input})
		if referenceErr != nil {
			t.Fatal(referenceErr)
		}
		class, detail := compareOracle(result, err, response.Results[0], o.schema)
		if class == oracleMismatch || class == oracleUnreachable {
			t.Fatalf("oracle class %s for %q: %s", class, input, detail)
		}
	})
}
