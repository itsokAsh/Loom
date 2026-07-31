param(
    [switch]$Production,
    [switch]$Tls
)

$Root = Split-Path -Parent (Split-Path -Parent $MyInvocation.MyCommand.Path)
Set-Location $Root

if (-not (Test-Path ".env")) {
    Write-Host "Creating .env from .env.example — edit secrets before production deploy."
    Copy-Item ".env.example" ".env"
}

$files = @("-f", "docker-compose.yml")
if ($Production) {
    $files += @("-f", "docker-compose.prod.yml")
}
if ($Tls) {
    $files += @("-f", "deploy/docker-compose.tls.yml")
}

$profile = @()
if ($Tls) {
    $profile = @("--profile", "tls")
}

$mode = if ($Production) { "production" } else { "local" }
if ($Tls) { $mode += " + tls" }
Write-Host "Deploying Loom ($mode)..."

docker compose @files @profile up -d --build

Write-Host ""
Write-Host "Status:"
docker compose @files ps
Write-Host ""
if ($Tls) {
    Write-Host "Open https://your-domain (set DEPLOY_DOMAIN in .env)"
} else {
    Write-Host "Open http://localhost:3000"
}
