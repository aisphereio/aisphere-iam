[CmdletBinding()]
param(
    [string]$Kustomize
)

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$deployDirectory = Join-Path $root "deploy"

if ([string]::IsNullOrWhiteSpace($Kustomize)) {
    $command = Get-Command kustomize -ErrorAction SilentlyContinue
    if ($null -eq $command) {
        $local = Join-Path $root ".bin\kustomize.exe"
        if (-not (Test-Path -LiteralPath $local)) {
            throw "kustomize was not found in PATH or .bin"
        }
        $Kustomize = $local
    } else {
        $Kustomize = $command.Source
    }
}

$previousErrorActionPreference = $ErrorActionPreference
$ErrorActionPreference = "Continue"
$renderedLines = & $Kustomize build $deployDirectory 2>&1
$buildExitCode = $LASTEXITCODE
$ErrorActionPreference = $previousErrorActionPreference
if ($buildExitCode -ne 0) {
    throw "kustomize build failed:`n$($renderedLines -join "`n")"
}
$rendered = $renderedLines -join "`n"

foreach ($kind in "Deployment", "Service", "NetworkPolicy") {
    if ($rendered -notmatch "(?m)^kind:\s+$kind\s*$") {
        throw "Rendered delivery YAML is missing kind $kind"
    }
}
if ($rendered -match "(?m)^kind:\s+Secret\s*$") {
    throw "Rendered delivery YAML must not contain a Secret"
}
if ($rendered -match "CHANGE_ME") {
    throw "Rendered delivery YAML contains a CHANGE_ME placeholder"
}

$routeFiles = Get-ChildItem -LiteralPath (Join-Path $deployDirectory "generated\gateway") -Recurse -File -Filter "*.yaml"
$renderedRouteCount = ([regex]::Matches($rendered, "(?m)^kind:\s+HTTPRoute\s*$")).Count
if ($renderedRouteCount -ne $routeFiles.Count) {
    throw "Rendered HTTPRoute count is $renderedRouteCount; generated route count is $($routeFiles.Count)"
}

foreach ($routeFile in $routeFiles) {
    $route = Get-Content -Raw -LiteralPath $routeFile.FullName
    $nameMatch = [regex]::Match($route, '(?m)^\s*name:\s*["]?([^"\r\n]+)["]?\s*$')
    if (-not $nameMatch.Success) {
        throw "Cannot read HTTPRoute name from $($routeFile.FullName)"
    }
    $name = $nameMatch.Groups[1].Value.Trim()
    if ($rendered -notmatch "(?m)^\s*name:\s+$([regex]::Escape($name))\s*$") {
        throw "Rendered delivery YAML is missing generated HTTPRoute $name"
    }
}

Write-Host "Kustomize delivery contains $renderedRouteCount generated HTTPRoutes and no secret material."
