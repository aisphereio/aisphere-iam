Add-Type -AssemblyName System.IO.Compression
Add-Type -AssemblyName System.IO.Compression.FileSystem

$SourceDir = "E:\coding\aisphereio\aisphere-iam"
$OutDir    = Join-Path $SourceDir ".output"
$ZipName   = "aisphere-iam_$(Get-Date -Format 'yyyyMMdd_HHmmss').zip"
$ZipPath   = Join-Path $OutDir $ZipName

if (-not (Test-Path $OutDir)) { New-Item -ItemType Directory -Path $OutDir -Force | Out-Null }

$ExcludePatterns = @(
    '*.exe',
    '.bin\*', '*\.bin\*',
    'bin\*',   '*\bin\*',
    '.git\*',  '*\.git\*',
    '.output\*'
)
$guid = [Guid]::NewGuid().ToString("N")
$TempDir = Join-Path $env:TEMP "pkg_$guid"

Write-Host "Collecting files (exclude: .bin / bin / .git / .output dirs, *.exe)"
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null

$count = 0
Get-ChildItem -Path $SourceDir -Recurse | Where-Object {
    $relative = $_.FullName.Substring($SourceDir.Length).TrimStart('\')
    foreach ($p in $ExcludePatterns) {
        if ($relative -like $p) { return $false }
    }
    return $true
} | ForEach-Object {
    $relative = $_.FullName.Substring($SourceDir.Length).TrimStart('\')
    $dest = Join-Path $TempDir $relative
    if ($_.PSIsContainer) {
        New-Item -ItemType Directory -Path $dest -Force | Out-Null
    } else {
        $destDir = Split-Path $dest -Parent
        if (-not (Test-Path $destDir)) { New-Item -ItemType Directory -Path $destDir -Force | Out-Null }
        Copy-Item $_.FullName $dest -Force
        $count++
    }
}

Write-Host "Packing $count files -> $ZipPath"
[System.IO.Compression.ZipFile]::CreateFromDirectory($TempDir, $ZipPath)

$size = (Get-Item $ZipPath).Length / 1MB
$msg = "Done: $ZipPath  ({0:N2} MB)" -f $size
Write-Host $msg
