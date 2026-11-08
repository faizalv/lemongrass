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
    return @{ UTF16 = @($Value.ToCharArray() | ForEach-Object { [int]$_ }) }
}

function Convert-Token($Token) {
    if ($null -eq $Token) { return $null }
    $nested = @()
    if ($Token -is [System.Management.Automation.Language.StringExpandableToken]) {
        $nested = @(foreach ($item in $Token.NestedTokens) { Convert-Token $item })
    }
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
        NestedTokens = $nested
    }
}

function Convert-Value($Value, $Spec) {
    if ($null -eq $Value) {
        if ($Spec.Kind -eq 'list') { return ,@() }
        if ($Spec.Kind -eq 'string') { return (Convert-String '') }
        return $null
    }
    switch ($Spec.Kind) {
        'node' {
            $name = $Value.GetType().Name
            $result = @{ Type = $name; Start = $Value.Extent.StartOffset; End = $Value.Extent.EndOffset }
            # These constructs are explicitly refused by the Go parser on every
            # OS. Windows registers DSC even in a fresh ParseInput process.
            if ($name -in @('ConfigurationDefinitionAst', 'DynamicKeywordStatementAst')) { return $result }
            # Recovery nodes have no counterpart in the fail-fast Go parser.
            if ($script:hasParseErrors -and $name -in @('ErrorStatementAst', 'ErrorExpressionAst')) { return $result }
            if ($null -eq $script:schema.Nodes.$name) { throw "oracle: unknown AST $name" }
            foreach ($field in $script:schema.Nodes.$name.PSObject.Properties) {
                $property = $field.Name
                if ($name -eq 'ExpandableStringExpressionAst' -and $property -eq 'FormatString') { $property = 'FormatExpression' }
                $result[$field.Name] = Convert-Value (Read-Property $Value $property) $field.Value
            }
            return $result
        }
        'list' {
            $items = @(foreach ($item in $Value) { Convert-Value $item $Spec.Element })
            return ,$items
        }
        'object' {
            $result = @{}
            foreach ($field in $script:schema.Objects.($Spec.Name).PSObject.Properties) {
                $property = $field.Name
                if ($Spec.Name -eq 'ScriptRequirements' -and $property -eq 'RequiredApplicationID') { $property = 'RequiredApplicationId' }
                # ast.cs represents these helper records as Tuple<T1,T2>.
                if ($Spec.Name -in @('IfClause', 'SwitchClause', 'KeyValuePair')) {
                    $property = if ($field.Name -in @('Condition', 'Key')) { 'Item1' } else { 'Item2' }
                }
                $result[$field.Name] = Convert-Value (Read-Property $Value $property) $field.Value
            }
            return $result
        }
        'typename' {
            return @{
                Type = $Value.GetType().Name; Start = $Value.Extent.StartOffset; End = $Value.Extent.EndOffset
                Name = Convert-String $Value.Name; FullName = Convert-String $Value.FullName; AssemblyName = Convert-String $Value.AssemblyName
                IsArray = $Value.IsArray; IsGeneric = $Value.IsGeneric
            }
        }
        'variable' { return @{ UserPath = Convert-String $Value.UserPath } }
        'extent' { return (Convert-Extent $Value) }
        'token' { return (Convert-Token $Value) }
        'constant' {
            if ($Value -is [string]) { return (Convert-String $Value) }
            if ($Value -is [bool]) { return $Value }
            return (Convert-Number $Value)
        }
        'string' { return (Convert-String $Value) }
        'bool' { return [bool]$Value }
        'int' { return [long]$Value }
        default { throw "oracle: unknown normalization kind $($Spec.Kind)" }
    }
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
                    @{ Offset = $node.Extent.StartOffset; UTF16 = @($text.ToCharArray() | ForEach-Object { [int]$_ }) }
                })
                ,$cases
            })
            [Console]::WriteLine((ConvertTo-Json -InputObject @{ Extractions = $extractions; Version = $PSVersionTable.PSVersion.ToString() } -Depth 100 -Compress))
            continue
        }
        $results = @(foreach ($inputText in $request.Inputs) {
            if ($inputText -isnot [string]) { $inputText = -join ([char[]]$inputText) }
            $tokens = $null
            $parseErrors = $null
            $tree = [System.Management.Automation.Language.Parser]::ParseInput([string]$inputText, [ref]$tokens, [ref]$parseErrors)
            $script:hasParseErrors = $parseErrors.Count -ne 0
            @{
                Tree = Convert-Value $tree ([pscustomobject]@{ Kind = 'node' })
                Tokens = @(foreach ($token in $tokens) { Convert-Token $token })
                Errors = @(foreach ($parseError in $parseErrors) {
                    @{
                        ID = $parseError.ErrorId; Start = $parseError.Extent.StartOffset
                        End = $parseError.Extent.EndOffset; Incomplete = $parseError.IncompleteInput
                    }
                })
            }
        })
        $response = @{ Version = $PSVersionTable.PSVersion.ToString(); Results = $results; Precedence = $script:rawPrecedence }
        [Console]::WriteLine((ConvertTo-Json -InputObject $response -Depth 100 -Compress))
    } catch {
        [Console]::WriteLine((ConvertTo-Json -InputObject @{ ProtocolError = $_.ToString() } -Compress))
    }
}
