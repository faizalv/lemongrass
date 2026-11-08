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
`ConfigurationDefinitionAst` and `DynamicKeywordStatementAst` also remain
type/extent leaves: the Go parser explicitly refuses them, and Windows registers
configuration in a fresh reference process.

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
assembly filesystem divergences have distinct classes. An unreachable
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
The `default` token may have 5.1's Identifier kind or omit the Keyword flag,
with every other token field and the tree still equal. New numeric spellings
must match the exact 5.1 generic-word token and command or argument reading;
other tree and token fields remain exact. Enum base types, keyword method names,
and bare array generic arguments qualify as 7-only grammar only at the error
extent belonging to that specific construct.
Unknown differences fail. The workflow uploads the complete 5.1 comparison
report, including cases beyond the first twenty failures printed in the log.

`TestOracleLegacyMeanings` and `TestOracleSevenFeatures` exercise these checks on
the host, including negative cases. Actual 5.1 parity requires the Windows run.

5.1 uses different numeric values for the binary-precedence categories in token
flags. The reference maps only those bits by the runtime enum category name to
the pinned 7 values. Every other flag bit still compares exactly, and the full
tree comparison checks actual operator grouping. Reports retain each runtime's
raw precedence enum values. A reference `ErrorLoadingAssembly` qualifies for the
filesystem class only on the matching using-assembly name and extent, for an
accepted file-URI spelling, with exact tree and token parity.

The Windows report is written on test exit even if a later batch aborts. Its
`Complete` and `Stage` fields distinguish a full comparison from partial data.
Mismatches include both normalized reference results. A ParseInput exception is
attached to its individual input as `ReferenceException`, remains a failure,
and lets the batch continue. It never counts as an ordinary parse error.

The named 5.1 exceptions captured in run 42485566814 qualify as `both-reject`
only for their exact inputs and matching Go/7 errors. `version-deny` also requires
matching Go/7 error IDs and extents for the approved clean, numeric, escape and
empty using-namespace readings. Newline-leading pipes, certain if-valued hash
entries, comma indices in method arguments and the observed here-string shape
have error-position-qualified 7-only classes. A bare Unicode escape token is
accepted only as a separate refusal class because the judge refuses that input.

`gen-definition-semantics.py` reads the pinned SemanticChecks.cs and
SymbolResolver.cs and excludes every resource ID referenced by Parser.cs or
tokenizer.cs. The generated `definition-semantic-ids.json` is eligible only when
the Go tree holds a definition or module/assembly using statement that the judge
refuses. The explicit general semantic list remains separate; syntax-shared IDs
never gain eligibility from the generated data. Windows reports retain source
lines for each accepted generated ID. Fuzzing writes the same evidence, and
reference exceptions with a Go rejection, to the JSON-lines path selected by
`LGRASS_PSPARSE_ORACLE_DEFINITION_REPORT` (use an absolute path).
Shared module IDs have a separate generated subset: their only syntax-source
references must be inside tokenizer.cs's Requires region, with no Parser.cs or
other tokenizer references. The reference error extent must be contained by a
module UsingStatementAst, there must be no #requires line, and the successfully
parsed Go tree and tokens must match exactly. This admits RequiresModuleInvalid
(SymbolResolver.cs:552) and RequiresArgumentMustBeConstant (:545); generated
provenance retains every syntax-reference line as well as the semantic origins.

`windows51-regressions.json` holds 60 short exact reference snapshots from run
42485566814. Inputs retain their original source attribution and the upstream
test fixtures' MIT notices remain in testdata. The default replay test uses these
snapshots without starting 5.1; `LGRASS_PSPARSE_ORACLE_REPLAY` can select the full
captured mismatch array. Replay verifies classification, while actual runtime
parity still requires the Windows workflow.
