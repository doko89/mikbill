#Requires -RunAsAdministrator
<#
.SYNOPSIS
  Pemasang Mikbil untuk Windows (PowerShell 5.1+).
.DESCRIPTION
  Mengunduh rilis Mikbil dari GitHub, memverifikasi SHA256 terhadap
  checksums.txt rilis, memasang ke C:\Program Files\Mikbil, menyimpan data di
  C:\ProgramData\Mikbil, lalu mendaftarkan Scheduled Task agar jalan saat startup.
  Idempotent: aman dijalankan ulang untuk upgrade (DB + ENC_KEY dipertahankan).
.EXAMPLE
  irm https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.ps1 | iex
.EXAMPLE
  $env:MIKBIL_VERSION = 'v1.0.0'; irm https://raw.githubusercontent.com/doko89/mikbill/main/deploy/install.ps1 | iex
#>
[CmdletBinding()]
param(
  [string]$Version = $(if ($env:MIKBIL_VERSION) { $env:MIKBIL_VERSION } else { 'latest' })
)

Set-StrictMode -Version Latest
$ErrorActionPreference = 'Stop'

# #Requires diabaikan saat via iex; cek eksplisit.
$IsAdmin = ([Security.Principal.WindowsPrincipal][Security.Principal.WindowsIdentity]::GetCurrent()).IsInRole(
  [Security.Principal.WindowsBuiltInRole]::Administrator)
if (-not $IsAdmin) { throw 'Jalankan skrip ini sebagai Administrator.' }
[Net.ServicePointManager]::SecurityProtocol = [Net.SecurityProtocolType]::Tls12

$Repo = 'doko89/mikbill'
$InstallDir = 'C:\Program Files\Mikbil'
$DataDir = 'C:\ProgramData\Mikbil'
$TaskName = 'Mikbil'

function Write-Step($msg) { Write-Host "==> $msg" -ForegroundColor Cyan }

# Tentukan basis URL unduhan rilis.
$Tag = $Version.Trim()
if ([string]::IsNullOrWhiteSpace($Tag) -or $Tag -eq 'latest') {
  $Base = "https://github.com/$Repo/releases/latest/download"
} else {
  if (-not $Tag.StartsWith('v')) { $Tag = "v$Tag" }
  $Base = "https://github.com/$Repo/releases/download/$Tag"
}

$TempDir = Join-Path ([IO.Path]::GetTempPath()) ("mikbil-install-" + [Guid]::NewGuid().ToString('N'))
New-Item -ItemType Directory -Path $TempDir -Force | Out-Null
try {
  # Unduh checksums.txt dulu; nama file zip diambil dari sini agar tahan
  # terhadap variasi penamaan aset rilis.
  Write-Step "Mengunduh checksums.txt dari $Base ..."
  $SumsFile = Join-Path $TempDir 'checksums.txt'
  Invoke-WebRequest -Uri "$Base/checksums.txt" -OutFile $SumsFile -UseBasicParsing

  $ZipName = $null
  $WantHash = $null
  foreach ($line in (Get-Content -Path $SumsFile)) {
    $parts = $line.Trim() -split '\s+', 2
    if ($parts.Count -eq 2 -and $parts[1] -like '*windows_amd64.zip') {
      $WantHash = $parts[0].ToLowerInvariant()
      $ZipName = $parts[1].TrimStart('*')
    }
  }
  if (-not $ZipName) { throw 'checksums.txt tidak memuat entri *windows_amd64.zip' }

  Write-Step "Mengunduh $ZipName ..."
  $ZipFile = Join-Path $TempDir $ZipName
  Invoke-WebRequest -Uri "$Base/$ZipName" -OutFile $ZipFile -UseBasicParsing

  Write-Step 'Memverifikasi SHA256 ...'
  $GotHash = (Get-FileHash -Path $ZipFile -Algorithm SHA256).Hash.ToLowerInvariant()
  if ($GotHash -ne $WantHash) {
    throw "SHA256 tidak cocok untuk $ZipName (dapat $GotHash, harap $WantHash)"
  }
  Write-Host 'SHA256 cocok.' -ForegroundColor Green

  # Hentikan task lama bila ada (abaikan bila belum ada).
  $OldTask = Get-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  if ($OldTask) {
    Write-Step "Menghentikan task terjadwal '$TaskName' ..."
    Stop-ScheduledTask -TaskName $TaskName -ErrorAction SilentlyContinue
  }

  New-Item -ItemType Directory -Path $InstallDir -Force | Out-Null
  New-Item -ItemType Directory -Path $DataDir -Force | Out-Null

  Write-Step "Mengekstrak ke $InstallDir ..."
  Add-Type -AssemblyName System.IO.Compression.FileSystem
  [IO.Compression.ZipFile]::ExtractToDirectory($ZipFile, $TempDir + '\zip')
  $Exe = Get-ChildItem -Path ($TempDir + '\zip') -Filter 'mikbil.exe' -Recurse | Select-Object -First 1
  if (-not $Exe) { throw 'arsip tidak memuat mikbil.exe' }
  Copy-Item -Path $Exe.FullName -Destination (Join-Path $InstallDir 'mikbil.exe') -Force
  $Readme = Get-ChildItem -Path ($TempDir + '\zip') -Filter 'README*' -Recurse | Select-Object -First 1
  if ($Readme) { Copy-Item -Path $Readme.FullName -Destination $InstallDir -Force }

  $ExePath = Join-Path $InstallDir 'mikbil.exe'
  $DbPath = Join-Path $DataDir 'mikbil.db'

  # Env level mesin. Kunci enkripsi dibuat sekali dan dipertahankan saat upgrade.
  [Environment]::SetEnvironmentVariable('MIKBIL_DB', $DbPath, 'Machine')
  $Key = [Environment]::GetEnvironmentVariable('MIKBIL_ENC_KEY', 'Machine')
  if ([string]::IsNullOrWhiteSpace($Key)) {
    Write-Step 'Membuat MIKBIL_ENC_KEY baru ...'
    $Key = (& $ExePath genkey | Out-String).Trim()
    if ($Key -notmatch '^[0-9a-fA-F]{64}$') { throw 'genkey tidak menghasilkan hex 64 char' }
    [Environment]::SetEnvironmentVariable('MIKBIL_ENC_KEY', $Key, 'Machine')
  } else {
    Write-Host 'MIKBIL_ENC_KEY lama dipertahankan.' -ForegroundColor Green
  }

  Write-Step "Mendaftarkan task terjadwal '$TaskName' ..."
  $Action = New-ScheduledTaskAction -Execute $ExePath -Argument 'serve' -WorkingDirectory $DataDir
  $Trigger = New-ScheduledTaskTrigger -AtStartup
  $Principal = New-ScheduledTaskPrincipal -UserId 'SYSTEM' -LogonType ServiceAccount -RunLevel Highest
  $Settings = New-ScheduledTaskSettingsSet -RestartCount 3 -RestartInterval (New-TimeSpan -Minutes 1) -StartWhenAvailable
  Register-ScheduledTask -TaskName $TaskName -Action $Action -Trigger $Trigger `
    -Principal $Principal -Settings $Settings -Description 'Mikbil billing hotspot voucher' -Force | Out-Null

  Write-Step 'Menjalankan Mikbil ...'
  Start-ScheduledTask -TaskName $TaskName
  Start-Sleep -Seconds 3
  $State = (Get-ScheduledTask -TaskName $TaskName).State
  Write-Host "Status task: $State" -ForegroundColor Green
} finally {
  Remove-Item -Path $TempDir -Recurse -Force -ErrorAction SilentlyContinue
}

Write-Host ''
Write-Host 'Instalasi selesai.' -ForegroundColor Green
Write-Host 'Langkah lanjut:'
Write-Host '  1. Buka http://localhost:8080/setup untuk wizard penyiapan awal.'
Write-Host '  2. Bila diakses dari LAN, buka port 8080 di Windows Firewall, misal:'
Write-Host '     New-NetFirewallRule -DisplayName "Mikbil" -Direction Inbound -Action Allow -Protocol TCP -LocalPort 8080'
Write-Host '  3. Data di C:\ProgramData\Mikbil ; log/diag via Task Scheduler > Mikbil.'
Write-Host '  4. Upgrade: jalankan ulang skrip ini (DB + kunci dipertahankan).'
