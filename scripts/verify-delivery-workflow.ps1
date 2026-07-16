[CmdletBinding()]
param()

$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
$workflowDirectory = Join-Path $root ".github\workflows"
$workflowFiles = Get-ChildItem -LiteralPath $workflowDirectory -File | Where-Object { $_.Extension -in ".yml", ".yaml" }
if ($workflowFiles.Count -eq 0) {
    throw "No GitHub Actions workflows found"
}

$allWorkflows = ($workflowFiles | ForEach-Object { Get-Content -Raw -LiteralPath $_.FullName }) -join "`n"
$forbidden = [ordered]@{
    "kubectl apply" = "(?i)kubectl\s+apply"
    "kubectl set image" = "(?i)kubectl\s+set\s+image"
    "kubectl rollout" = "(?i)kubectl\s+rollout"
    "kubeconfig" = "(?i)kubeconfig|KUBE_CONFIG"
    "cluster secret" = "(?i)KUBE_TOKEN|K8S_TOKEN|CLUSTER_TOKEN"
}
foreach ($entry in $forbidden.GetEnumerator()) {
    if ($allWorkflows -match $entry.Value) {
        throw "GitHub Actions must not contain $($entry.Key)"
    }
}

$ciPath = Join-Path $workflowDirectory "ci.yml"
$deliveryPath = Join-Path $workflowDirectory "delivery.yml"
if (-not (Test-Path -LiteralPath $deliveryPath)) {
    throw "Missing .github/workflows/delivery.yml"
}
$ci = Get-Content -Raw -LiteralPath $ciPath
$delivery = Get-Content -Raw -LiteralPath $deliveryPath

$ciRequirements = [ordered]@{
    "full Git history" = "fetch-depth:\s*0"
    "protobuf breaking check" = "breaking-check|buf breaking|contract-check"
    "OpenAPI normalization" = "openapi-check|contract-check"
    "generated diff" = "git diff --quiet --exit-code"
    "Go tests" = "go test"
    "traceability" = "traceability-check"
    "binary build" = "make build"
    "Kustomize render" = "kustomize build"
    "container build" = "docker/build-push-action"
    "PR container push disabled" = "push:\s*false"
}
foreach ($entry in $ciRequirements.GetEnumerator()) {
    if ($ci -notmatch $entry.Value) {
        throw "CI is missing $($entry.Key)"
    }
}

$deliveryRequirements = [ordered]@{
    "verification dependency" = "needs:\s*verify"
    "registry digest capture" = "steps\.build\.outputs\.digest"
    "digest-pinned image" = "kustomize edit set image"
    "rendered manifest" = "dist/manifests/aisphere-iam\.yaml"
    "image reference metadata" = "image-ref\.txt"
    "source SHA metadata" = "source-sha\.txt"
    "OpenAPI contract artifact" = "aisphere\.swagger\.json"
    "checksums" = "SHA256SUMS"
    "artifact upload" = "actions/upload-artifact"
    "release attachment" = "softprops/action-gh-release"
}
foreach ($entry in $deliveryRequirements.GetEnumerator()) {
    if ($delivery -notmatch $entry.Value) {
        throw "Delivery workflow is missing $($entry.Key)"
    }
}

Write-Host "GitHub Actions delivery safety checks passed."
