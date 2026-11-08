# PowerShell parser oracle

`dump.ps1` is a test-only JSON-lines server over `Parser.ParseInput`. Each request
contains a batch of UTF-16 arrays. The first request also supplies a schema derived
from the exported fields of the Go AST. Responses contain the normalized tree,
tokens (runtime class, kind, flags, extents, nested tokens), and parse errors.
The server parses text without invoking the supplied scripts.

String fields use UTF-16 arrays too, preserving unpaired surrogate values that
ordinary Go JSON strings would replace. Empty collections normalize to `[]` and
the Go empty extent normalizes to offset zero. Numeric constants retain their
.NET type and exact value. The explicit adapters are `FormatString` to the
upstream `FormatExpression` and `RequiredApplicationID` to `RequiredApplicationId`.
Recovery nodes have only their type and extent because the Go parser stops at
the first error and never compares recovery trees.

Run `go test ./internal/psparse -run TestOracle -count=1 -v`. The tests skip when
`pwsh` is absent. Set `LGRASS_PSPARSE_ORACLE_EXE` to select a particular executable,
and `LGRASS_PSPARSE_ORACLE_REQUIRED=1` to fail instead of skipping if it is absent.
`go test -short` checks the curated corpus; the full run also extracts quoted
string values and script-block bodies from the pinned upstream test files using
the real parser. Inputs carry their source filename and UTF-16 offset.

`TestOracleProtocol`, `TestOracleNormalization`, and `TestOracleReferenceCorpus`
validate the reference protocol without relying on the port. `TestOracleCorpus`
compares the port, with both-error ID and offset agreement reported separately.
Only the nine reviewed IDs in `oracleSemanticIDs` qualify for semantic-only
acceptance. Explicit resource limits, unsupported dynamic keywords, and the
relative-assembly filesystem divergence have distinct classes. An unreachable
recovery branch fails if PowerShell accepts the input.

During port development only the exact `notPorted` readiness panic skips the
Go comparison and fuzz targets. Remove that guard when the port is complete.
Any other panic or a rejected readiness command fails.

Fuzz with `go test ./internal/psparse -run '^$' -fuzz '^FuzzParseOracle$' -fuzztime=30s`.
Each worker keeps one reference process alive. `FuzzParseNoPanic` runs without
PowerShell and tests the bounded parser on arbitrary strings.

`unicode-bmp.ps1` regenerates `testdata/dotnet-unicode-bmp.json` with the reference
runtime's BMP category runs, invariant uppercase mapping, and selected ordinal
comparisons. It is a fixture generator, never part of the shipped parser.
