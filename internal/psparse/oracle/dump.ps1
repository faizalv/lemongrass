# SPDX-License-Identifier: MPL-2.0
# Copyright (C) 2026 The Lemongrass Authors

# JSON lines on stdin, one response per batch. ParseInput only parses the supplied
# text; none of the corpus is invoked. Keep stdout exclusively for the protocol.
$ErrorActionPreference = 'Stop'
[Console]::InputEncoding = New-Object System.Text.UTF8Encoding($false)
[Console]::OutputEncoding = New-Object System.Text.UTF8Encoding($false)
$script:schema = $null
$script:binding = [Reflection.BindingFlags]'Instance,Public,NonPublic'
$script:culture = [Globalization.CultureInfo]::InvariantCulture
$script:jsonEscapes = [char[]](@(0..31) + @(34, 92))
# Container levels per converted tree, far beyond any tree the Go parser accepts
# under Limits.MaxDepth and well inside encoding/json's 10000 nesting limit.
$script:depthCap = 4000
$script:nodeSpec = [pscustomobject]@{ Kind = 'node' }
$script:tokenSpec = [pscustomobject]@{ Kind = 'token' }
$script:tokenListSpec = [pscustomobject]@{ Kind = 'list'; Element = $script:tokenSpec }

function Write-OracleJsonString([string]$Value, [Text.StringBuilder]$Output) {
    if ($Value.IndexOfAny($script:jsonEscapes) -lt 0) {
        [void]$Output.Append('"').Append($Value).Append('"')
    } else {
        [void]$Output.Append((ConvertTo-Json -InputObject $Value -Compress))
    }
}

# ConvertTo-Json truncates containers beyond depth 100, and recursion would hit
# PowerShell's script call-depth limit, so containers are written from an
# explicit stack. Rejecting every other value kind prevents silent loss.
function Convert-OracleJson($Value) {
    $output = New-Object Text.StringBuilder
    $stack = New-Object Collections.Stack
    $pending = $Value
    $hasPending = $true
    while ($true) {
        if ($hasPending) {
            $hasPending = $false
            if ($null -eq $pending) {
                [void]$output.Append('null')
            } elseif ($pending -is [string]) {
                Write-OracleJsonString $pending $output
            } elseif ($pending -is [bool]) {
                if ($pending) { [void]$output.Append('true') } else { [void]$output.Append('false') }
            } elseif ($pending -is [Collections.IDictionary]) {
                $keys = New-Object Collections.ArrayList
                foreach ($key in $pending.Keys) {
                    if ($key -isnot [string]) { throw 'oracle: JSON object key is not a string' }
                    [void]$keys.Add($key)
                }
                [void]$output.Append('{')
                $stack.Push(@{ Items = $keys; Dictionary = $pending; Index = 0 })
            } elseif ($pending -is [int[]]) {
                [void]$output.Append('[').Append(($pending -join ',')).Append(']')
            } elseif ($pending -is [Collections.IEnumerable]) {
                $items = New-Object Collections.ArrayList
                foreach ($item in $pending) { [void]$items.Add($item) }
                [void]$output.Append('[')
                $stack.Push(@{ Items = $items; Dictionary = $null; Index = 0 })
            } elseif ([Type]::GetTypeCode($pending.GetType()).ToString() -in @('Byte', 'SByte', 'Int16', 'UInt16', 'Int32', 'UInt32', 'Int64', 'UInt64', 'Single', 'Double', 'Decimal')) {
                $number = [Convert]::ToString($pending, $script:culture)
                if ($number -in @('NaN', 'Infinity', '-Infinity')) { throw 'oracle: non-finite JSON number' }
                [void]$output.Append($number)
            } else {
                throw "oracle: unsupported JSON value $($pending.GetType().FullName)"
            }
        }
        if ($stack.Count -eq 0) { break }
        $frame = $stack.Peek()
        if ($frame.Index -ge $frame.Items.Count) {
            if ($null -eq $frame.Dictionary) { [void]$output.Append(']') } else { [void]$output.Append('}') }
            [void]$stack.Pop()
            continue
        }
        if ($frame.Index -gt 0) { [void]$output.Append(',') }
        $item = $frame.Items[$frame.Index]
        $frame.Index = $frame.Index + 1
        if ($null -eq $frame.Dictionary) {
            $pending = $item
        } else {
            Write-OracleJsonString $item $output
            [void]$output.Append(':')
            $pending = $frame.Dictionary[$item]
        }
        $hasPending = $true
    }
    return $output.ToString()
}

function Read-Property($Object, [string]$Name) {
    $type = $Object.GetType()
    while ($null -ne $type) {
        $property = $type.GetProperty($Name, $script:binding -bor [Reflection.BindingFlags]::DeclaredOnly)
        if ($null -ne $property) { return ,$property.GetValue($Object, $null) }
        $field = $type.GetField($Name, $script:binding -bor [Reflection.BindingFlags]::DeclaredOnly)
        if ($null -ne $field) { return ,$field.GetValue($Object) }
        $type = $type.BaseType
    }
    if ($PSVersionTable.PSVersion.Major -lt 6) {
        if ($Name -in @('CleanBlock', 'GenericTypeArguments')) { return $null }
        if ($Name -in @('NullConditional', 'Background')) { return $false }
    }
    throw "oracle: property $Name missing on $($Object.GetType().Name)"
}

function Convert-Extent($Extent) {
    return @{ Start = $Extent.StartOffset; End = $Extent.EndOffset }
}

function Convert-Number($Value) {
    $name = $Value.GetType().Name
    $text = if ($Value -is [double] -or $Value -is [single]) {
        $Value.ToString('R', $script:culture)
    } else { $Value.ToString($script:culture) }
    return @{ Type = $name; Value = $text }
}

function Convert-String([string]$Value) {
    return @{ UTF16 = [int[]]($Value.ToCharArray()) }
}

function Convert-TokenRecord($Token) {
    $flags = [long]$Token.TokenFlags
    if ($PSVersionTable.PSVersion.Major -lt 6) {
        $precedence = $flags -band $script:precedenceMask
        if ($precedence -ne 0) {
            $key = [string]$precedence
            if (-not $script:canonicalPrecedence.ContainsKey($key)) { throw "oracle: unknown legacy precedence $precedence" }
            $flags = ($flags -band (-bnot $script:precedenceMask)) -bor $script:canonicalPrecedence[$key]
        }
    }
    return @{
        Kind = $Token.Kind.ToString(); Flags = $flags
        Class = $Token.GetType().Name
        Start = $Token.Extent.StartOffset; End = $Token.Extent.EndOffset
        NestedTokens = @()
    }
}

# Converts from an explicit work stack: recursion would hit PowerShell's script
# call-depth limit on deep recovery trees. Past $script:depthCap container levels
# the result is $null with Capped set, because Go's encoding/json refuses nesting
# past 10000 and the harness must never compare a truncated reference.
function Convert-OracleTree($Root, $RootSpec) {
    $holder = @{ Value = $null }
    $stack = New-Object Collections.Stack
    $stack.Push(@($Root, $RootSpec, $holder, 'Value', 0))
    while ($stack.Count -ne 0) {
        $task = $stack.Pop()
        $Value = $task[0]; $Spec = $task[1]; $target = $task[2]; $slot = $task[3]; $depth = $task[4]
        if ($depth -gt $script:depthCap) { return @{ Value = $null; Capped = $true } }
        if ($null -eq $Value) {
            if ($Spec.Kind -eq 'list') { $target[$slot] = @() }
            elseif ($Spec.Kind -eq 'string') { $target[$slot] = (Convert-String '') }
            else { $target[$slot] = $null }
            continue
        }
        switch ($Spec.Kind) {
            'node' {
                $name = $Value.GetType().Name
                $result = @{ Type = $name; Start = $Value.Extent.StartOffset; End = $Value.Extent.EndOffset }
                $target[$slot] = $result
                # These constructs are explicitly refused by the Go parser on every
                # OS. Windows registers DSC even in a fresh ParseInput process.
                if ($name -in @('ConfigurationDefinitionAst', 'DynamicKeywordStatementAst')) { break }
                # Recovery nodes have no counterpart in the fail-fast Go parser.
                if ($script:hasParseErrors -and $name -in @('ErrorStatementAst', 'ErrorExpressionAst')) { break }
                if ($null -eq $script:schema.Nodes.$name) { throw "oracle: unknown AST $name" }
                foreach ($field in $script:schema.Nodes.$name.PSObject.Properties) {
                    $property = $field.Name
                    if ($name -eq 'ExpandableStringExpressionAst' -and $property -eq 'FormatString') { $property = 'FormatExpression' }
                    $stack.Push(@((Read-Property $Value $property), $field.Value, $result, $field.Name, ($depth + 1)))
                }
            }
            'list' {
                $source = New-Object Collections.ArrayList
                foreach ($item in $Value) { [void]$source.Add($item) }
                $items = New-Object object[] $source.Count
                $target[$slot] = $items
                for ($i = 0; $i -lt $source.Count; $i++) {
                    $stack.Push(@($source[$i], $Spec.Element, $items, $i, ($depth + 1)))
                }
            }
            'object' {
                $result = @{}
                $target[$slot] = $result
                foreach ($field in $script:schema.Objects.($Spec.Name).PSObject.Properties) {
                    $property = $field.Name
                    if ($Spec.Name -eq 'ScriptRequirements' -and $property -eq 'RequiredApplicationID') { $property = 'RequiredApplicationId' }
                    # ast.cs represents these helper records as Tuple<T1,T2>.
                    if ($Spec.Name -in @('IfClause', 'SwitchClause', 'KeyValuePair')) {
                        $property = if ($field.Name -in @('Condition', 'Key')) { 'Item1' } else { 'Item2' }
                    }
                    $stack.Push(@((Read-Property $Value $property), $field.Value, $result, $field.Name, ($depth + 1)))
                }
            }
            'token' {
                $result = Convert-TokenRecord $Value
                $target[$slot] = $result
                if ($Value -is [System.Management.Automation.Language.StringExpandableToken] -and $null -ne $Value.NestedTokens) {
                    $nested = New-Object Collections.ArrayList
                    foreach ($item in $Value.NestedTokens) { [void]$nested.Add($item) }
                    $items = New-Object object[] $nested.Count
                    $result.NestedTokens = $items
                    for ($i = 0; $i -lt $nested.Count; $i++) {
                        $stack.Push(@($nested[$i], $script:tokenSpec, $items, $i, ($depth + 2)))
                    }
                }
            }
            'typename' {
                $target[$slot] = @{
                    Type = $Value.GetType().Name; Start = $Value.Extent.StartOffset; End = $Value.Extent.EndOffset
                    Name = Convert-String $Value.Name; FullName = Convert-String $Value.FullName; AssemblyName = Convert-String $Value.AssemblyName
                    IsArray = $Value.IsArray; IsGeneric = $Value.IsGeneric
                }
            }
            'variable' { $target[$slot] = @{ UserPath = Convert-String $Value.UserPath } }
            'extent' { $target[$slot] = (Convert-Extent $Value) }
            'constant' {
                if ($Value -is [string]) { $target[$slot] = (Convert-String $Value) }
                elseif ($Value -is [bool]) { $target[$slot] = $Value }
                else { $target[$slot] = (Convert-Number $Value) }
            }
            'string' { $target[$slot] = (Convert-String $Value) }
            'bool' { $target[$slot] = [bool]$Value }
            'int' { $target[$slot] = [long]$Value }
            default { throw "oracle: unknown normalization kind $($Spec.Kind)" }
        }
    }
    return @{ Value = $holder.Value; Capped = $false }
}

while ($null -ne ($line = [Console]::ReadLine())) {
    try {
        $request = ConvertFrom-Json -InputObject $line
        if ($null -ne $request.Schema) {
            $script:schema = $request.Schema
            $flagType = [System.Management.Automation.Language.TokenFlags]
            $script:precedenceMask = [long][Enum]::Parse($flagType, 'BinaryPrecedenceMask')
            $script:rawPrecedence = @{}
            $script:canonicalPrecedence = @{}
            foreach ($name in [Enum]::GetNames($flagType)) {
                if ($name -notlike 'BinaryPrecedence*' -or $name -eq 'BinaryPrecedenceMask') { continue }
                $raw = [long][Enum]::Parse($flagType, $name)
                $canonical = $script:schema.Precedence.$name
                if ($null -eq $canonical) { throw "oracle: unknown precedence category $name" }
                $script:rawPrecedence[$name] = $raw
                $script:canonicalPrecedence[[string]$raw] = [long]$canonical
            }
        }
        if ($null -eq $script:schema) { throw 'oracle: first request must supply Schema' }
        if ($request.Operation -eq 'Extract') {
            $extractions = @(foreach ($inputText in $request.Inputs) {
                if ($inputText -isnot [string]) { $inputText = -join ([char[]]$inputText) }
                $tokens = $null
                $parseErrors = $null
                $tree = [System.Management.Automation.Language.Parser]::ParseInput([string]$inputText, [ref]$tokens, [ref]$parseErrors)
                $cases = @(foreach ($node in $tree.FindAll({ param($ast)
                    $ast -is [System.Management.Automation.Language.ScriptBlockExpressionAst] -or
                    ($ast -is [System.Management.Automation.Language.StringConstantExpressionAst] -and $ast.StringConstantType -ne 'BareWord')
                }, $true)) {
                    $text = if ($node -is [System.Management.Automation.Language.ScriptBlockExpressionAst]) {
                        $node.Extent.Text.Substring(1, $node.Extent.Text.Length - 2)
                    } else { $node.Value }
                    @{ Offset = $node.Extent.StartOffset; UTF16 = [int[]]($text.ToCharArray()) }
                })
                ,$cases
            })
            [Console]::WriteLine((Convert-OracleJson @{ Extractions = $extractions; Version = $PSVersionTable.PSVersion.ToString() }))
            continue
        }
        $results = @(foreach ($inputText in $request.Inputs) {
            if ($inputText -isnot [string]) { $inputText = -join ([char[]]$inputText) }
            $tokens = $null
            $parseErrors = $null
            try {
                $tree = [System.Management.Automation.Language.Parser]::ParseInput([string]$inputText, [ref]$tokens, [ref]$parseErrors)
            } catch {
                # A reference exception is not a parse error. Keep it attached to
                # this input so the harness fails it without losing later cases.
                # ParseInput wraps every parser fault in UnrecoverableParserError,
                # so the inner chain is the only record of where it threw.
                $text = $_.ToString()
                $inner = $_.Exception.InnerException
                $chain = @()
                while ($null -ne $inner) {
                    $text += " <- $($inner.GetType().FullName): $($inner.Message)"
                    $chain = @($inner) + $chain
                    $inner = $inner.InnerException
                }
                # The harness classifies by the parser phase on the stack, so keep
                # the innermost top frames plus every phase frame of the chain.
                $frames = New-Object Collections.ArrayList
                foreach ($exception in $chain) {
                    if ($null -eq $exception.StackTrace) { continue }
                    $lines = @($exception.StackTrace -split "\r?\n" | ForEach-Object { $_.Trim() } | Where-Object { $_ -ne '' })
                    for ($i = 0; $i -lt $lines.Count; $i++) {
                        if (([object]::ReferenceEquals($exception, $chain[0]) -and $i -lt 8) -or $lines[$i] -match '\.Language\.(SymbolResolver\.ResolveSymbols|SemanticChecks\.CheckAst|Parser\.\w+)\(') {
                            [void]$frames.Add($lines[$i])
                        }
                    }
                }
                if ($frames.Count -ne 0) { $text += ' | ' + ($frames -join ' | ') }
                @{ ReferenceException = $text }
                continue
            }
            $script:hasParseErrors = $parseErrors.Count -ne 0
            $errors = @(foreach ($parseError in $parseErrors) {
                @{
                    ID = $parseError.ErrorId; Start = $parseError.Extent.StartOffset
                    End = $parseError.Extent.EndOffset; Incomplete = $parseError.IncompleteInput
                }
            })
            $convertedTree = Convert-OracleTree $tree $script:nodeSpec
            $convertedTokens = Convert-OracleTree $tokens $script:tokenListSpec
            if ($convertedTree.Capped -or $convertedTokens.Capped) {
                @{ DepthCapped = $true; Errors = $errors }
            } else {
                @{ Tree = $convertedTree.Value; Tokens = $convertedTokens.Value; Errors = $errors }
            }
        })
        $response = @{ Version = $PSVersionTable.PSVersion.ToString(); Results = $results; Precedence = $script:rawPrecedence }
        [Console]::WriteLine((Convert-OracleJson $response))
    } catch {
        [Console]::WriteLine((Convert-OracleJson @{ ProtocolError = $_.ToString() }))
    }
}
