# Script to auto-build Termi with NSIS installer and apply versioning to output filenames.
# Requires 'makensis' (NSIS) to be installed and available in PATH.

$wailsJsonPath = "wails.json"
if (-Not (Test-Path $wailsJsonPath)) {
    Write-Host "Error: wails.json not found!" -ForegroundColor Red
    exit 1
}

$wailsJson = Get-Content -Raw $wailsJsonPath | ConvertFrom-Json
$version = $wailsJson.info.productVersion
$appName = $wailsJson.name

# 1. Verify NSIS is installed and inject into PATH if missing
try {
    $null = Get-Command makensis -ErrorAction Stop
} catch {
    # Check default installation paths
    $nsisPath86 = "C:\Program Files (x86)\NSIS"
    $nsisPath64 = "C:\Program Files\NSIS"
    
    if (Test-Path "$nsisPath86\makensis.exe") {
        $env:PATH = "$nsisPath86;" + $env:PATH
        Write-Host "Auto-discovered NSIS at $nsisPath86 and linked it to session." -ForegroundColor Cyan
    } elseif (Test-Path "$nsisPath64\makensis.exe") {
        $env:PATH = "$nsisPath64;" + $env:PATH
        Write-Host "Auto-discovered NSIS at $nsisPath64 and linked it to session." -ForegroundColor Cyan
    } else {
        Write-Host "Error: NSIS (makensis) is not installed or not in PATH." -ForegroundColor Red
        Write-Host "You must install NSIS to generate Windows Setup Installers." -ForegroundColor Yellow
        Write-Host "Easiest way using Scoop (if installed): scoop install nsis"
        Write-Host "Or download from: https://nsis.sourceforge.io/Download"
        exit 1
    }
}

Write-Host "Building $appName v$version with NSIS Output..." -ForegroundColor Cyan

# 2. Trigger natively packaged Wails NSIS build
wails build -nsis
if ($LASTEXITCODE -ne 0) {
    Write-Host "Error: 'wails build -nsis' failed with exit code $LASTEXITCODE." -ForegroundColor Red
    exit $LASTEXITCODE
}

$binDir = "build\bin"
$baseExe = "$binDir\${appName}.exe"
$installerExe = "$binDir\${appName}-amd64-installer.exe"

$newBaseExe = "$binDir\${appName}-v${version}.exe"
$newInstallerExe = "$binDir\${appName}-v${version}-setup.exe"

# 3. Rename binaries gracefully to include the version
if (Test-Path $baseExe) {
    Move-Item -Path $baseExe -Destination $newBaseExe -Force
    Write-Host "[OK] Renamed App: $(Split-Path $newBaseExe -Leaf)" -ForegroundColor Green
}

if (Test-Path $installerExe) {
    Move-Item -Path $installerExe -Destination $newInstallerExe -Force
    Write-Host "[OK] Renamed Installer: $(Split-Path $newInstallerExe -Leaf)" -ForegroundColor Green
} else {
    Write-Host "Error: expected installer not found at $installerExe." -ForegroundColor Red
    exit 1
}

# 4. Warn about SmartScreen
Write-Host "`n=================================================" -ForegroundColor Magenta
Write-Host "Code Signing and Windows SmartScreen ('Safe to Run')" -ForegroundColor White
Write-Host "=================================================" -ForegroundColor Magenta
Write-Host "The generated .exe will trigger 'Windows protected your PC' warnings for end-users."
Write-Host "To permanently bypass this block and prove your application is safe, you MUST cryptographically"
Write-Host "sign your executables using a purchased Authenticode Code Signing Certificate (e.g., from DigiCert/Sectigo)."
Write-Host "`nIf you acquire a certificate (.pfx file), you sign the installer using the Windows SDK signtool:" -ForegroundColor Yellow
Write-Host "  signtool sign /tr http://timestamp.digicert.com /td sha256 /fd sha256 /f mycert.pfx /p password $newInstallerExe"
Write-Host ""
