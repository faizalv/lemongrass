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
Only reviewed IDs in `oracleSemanticIDs` qualify for semantic-only acceptance.
The shared IDs `UnexpectedAttribute` and `DuplicateNamedArgument` additionally
require the exact post-check AST shape and full tree/token parity. Explicit
resource limits, unsupported dynamic keywords, unresolved enum base types, and
the relative-assembly filesystem divergence have distinct classes. An unreachable
recovery branch fails if PowerShell accepts the input. Each corpus panic is a
failure, and the run continues to identify independent failures.

Set `LGRASS_PSPARSE_ORACLE_REPORT` to write a JSON report of all comparison
classes, sources, inputs and differences. Differences identify the first
diverging field. The first-error ID and offset are a separate soft metric.

Fuzz with `go test ./internal/psparse -run '^$' -fuzz '^FuzzParseOracle$' -fuzztime=30s`.
Each worker keeps one reference process alive. `FuzzParseNoPanic` runs without
PowerShell and tests the bounded parser on arbitrary strings.

`unicode-bmp.ps1` regenerates `testdata/dotnet-unicode-bmp.json` with the reference
runtime's BMP category runs, invariant uppercase mapping, and selected ordinal
comparisons. It is a fixture generator, never part of the shipped parser.

`native-argv.ps1` is a separate fixed execution probe requested for the semantic
tables. It does not consume corpus text. On pwsh 7.6.9 on Unix, a literal `a,b`
reaches printf as one argument, while a two-element variable reaches it as two.

The Windows workflow installs portable pwsh 7.6.9 after checking its official
SHA-256 and makes it mandatory for the regular tests. A separate step sets
`LGRASS_PSPARSE_ORACLE_51=1` and runs `TestOracleWindows51` against both that
runtime and `%SystemRoot%\System32\WindowsPowerShell\v1.0\powershell.exe`.
The same corpus must match wherever 5.1 accepts it. Checked 7-only operators,
numeric syntax, background pipelines, generic method arguments, and clean blocks
have their own rejection class. The existing version meanings are checked
separately: a legacy clean command must preserve the exact block body, and
legacy string differences must follow only from the new escape sequences.
Unknown differences fail. The workflow uploads the complete 5.1 comparison
report, including cases beyond the first twenty failures printed in the log.

`TestOracleLegacyMeanings` and `TestOracleSevenFeatures` exercise these checks on
the host, including negative cases. Actual 5.1 parity requires the Windows run.
