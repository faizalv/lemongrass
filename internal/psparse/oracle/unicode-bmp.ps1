# SPDX-License-Identifier: MPL-2.0
# Copyright (C) 2026 The Lemongrass Authors

# Test fixture generator. Invoke with pwsh -NoProfile -File; never runs in the
# shipped parser. Runs describe identical adjacent UTF-16 character categories.
$ErrorActionPreference = 'Stop'
$runs = New-Object 'System.Collections.Generic.List[object]'
$previous = -1
for ($code = 0; $code -lt 65536; $code++) {
    $category = [int][char]::GetUnicodeCategory([char]$code)
    if ($category -ne $previous) {
        $runs.Add(@($code, $category))
        $previous = $category
    }
}
$folds = New-Object 'System.Collections.Generic.List[object]'
for ($code = 0; $code -lt 65536; $code++) {
    $upper = [int][char]::ToUpperInvariant([char]$code)
    if ($upper -ne $code) { $folds.Add(@($code, $upper)) }
}
$comparisons = @(foreach ($pair in @(@('i', 'I'), @('i', [string][char]0x131), @('s', [string][char]0x17f), @('k', [string][char]0x212a))) {
    @{ A = $pair[0]; B = $pair[1]; Equal = [string]::Equals($pair[0], $pair[1], [StringComparison]::OrdinalIgnoreCase) }
})
@{
    Source = 'pwsh 7.6.9, System.Char.GetUnicodeCategory and ToUpperInvariant, UTF-16 BMP'
    Version = $PSVersionTable.PSVersion.ToString()
    CategoryRuns = $runs.ToArray()
    Uppercase = $folds.ToArray()
    OrdinalComparisons = $comparisons
} | ConvertTo-Json -Depth 8 -Compress
