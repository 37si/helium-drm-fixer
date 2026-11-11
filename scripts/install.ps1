$ErrorActionPreference = 'Stop'

$repo = if ($env:HDF_REPO) { $env:HDF_REPO } else { '37si/helium-drm-fixer' }
$version = $env:HDF_VERSION
$bindir = if ($env:HDF_BIN) { $env:HDF_BIN } else { Join-Path $env:LOCALAPPDATA 'hdf' }

$arch = switch ($env:PROCESSOR_ARCHITECTURE) {
    'AMD64' { 'amd64' }
    'ARM64' { 'arm64' }
    default { throw "unsupported architecture: $env:PROCESSOR_ARCHITECTURE" }
}

if (-not $version) {
    $release = Invoke-RestMethod -Uri "https://api.github.com/repos/$repo/releases/latest" -Headers @{ 'User-Agent' = 'hdf' }
    $version = $release.tag_name.TrimStart('v')
}

if (-not $version) { throw "could not work out the newest release of $repo" }

$name = "hdf_${version}_windows_${arch}.exe"
$base = "https://github.com/$repo/releases/download/v$version"

$tmp = Join-Path ([System.IO.Path]::GetTempPath()) ([System.Guid]::NewGuid())
New-Item -ItemType Directory -Path $tmp | Out-Null

try {
    Write-Host "downloading $name"
    $binary = Join-Path $tmp 'hdf.exe'
    Invoke-WebRequest -Uri "$base/$name" -OutFile $binary -Headers @{ 'User-Agent' = 'hdf' }

    $sums = Invoke-WebRequest -Uri "$base/sha256.txt" -Headers @{ 'User-Agent' = 'hdf' } | Select-Object -ExpandProperty Content
    $line = ($sums -split "`n") | Where-Object {
        $parts = $_ -split '\s+'
        $parts.Count -ge 2 -and $parts[1].TrimStart('*') -eq $name
    } | Select-Object -First 1
    if (-not $line) { throw "sha256.txt has no entry for $name" }

    $expected = ($line -split '\s+')[0].ToUpper()
    $actual = (Get-FileHash -Path $binary -Algorithm SHA256).Hash.ToUpper()
    if ($expected -ne $actual) { throw "checksum mismatch for $name" }

    New-Item -ItemType Directory -Path $bindir -Force | Out-Null
    Move-Item -Path $binary -Destination (Join-Path $bindir 'hdf.exe') -Force

    Write-Host "installed $(Join-Path $bindir 'hdf.exe') ($version)"

    $path = [Environment]::GetEnvironmentVariable('Path', 'User')
    if ($path -notlike "*$bindir*") {
        [Environment]::SetEnvironmentVariable('Path', "$path;$bindir", 'User')
        Write-Host "added $bindir to your user PATH, open a new terminal to use it"
    }
}
finally {
    Remove-Item -Recurse -Force $tmp -ErrorAction SilentlyContinue
}
