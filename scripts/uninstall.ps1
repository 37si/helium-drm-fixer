$ErrorActionPreference = 'Stop'

$purge = $env:HDF_PURGE
if (-not $purge -and ($args -contains '--purge')) {
    $purge = '1'
}

$bindir = if ($env:HDF_BIN) { $env:HDF_BIN } else { Join-Path $env:LOCALAPPDATA 'hdf' }
$binary = Join-Path $bindir 'hdf.exe'

if (Test-Path $binary) {
    Remove-Item -Force $binary
    Write-Host "removed $binary"
} else {
    Write-Host "$binary was not there"
}

$path = [Environment]::GetEnvironmentVariable('Path', 'User')
if ($path) {
    $parts = $path -split ';' | Where-Object { $_ -and $_.TrimEnd('\') -ne $bindir.TrimEnd('\') }
    $next = $parts -join ';'
    if ($next -ne $path) {
        [Environment]::SetEnvironmentVariable('Path', $next, 'User')
        Write-Host "removed $bindir from your user PATH"
    }
}

$config = Join-Path $env:APPDATA 'hdf'
$cache = Join-Path $env:LOCALAPPDATA 'hdf'

if ($purge -eq '1' -or $purge -eq 'true' -or $purge -eq 'yes') {
    Remove-Item -Recurse -Force $config, $cache -ErrorAction SilentlyContinue
    Write-Host "removed $config and $cache"
} else {
    Write-Host "kept settings and backups in $config"
    Write-Host "set HDF_PURGE=1 to remove them too"
}
