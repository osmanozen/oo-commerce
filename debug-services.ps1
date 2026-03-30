$services = @("catalog", "cart", "ordering", "inventory", "profiles", "reviews", "wishlists", "coupons")
$servicePorts = @{
    "catalog" = 8081
    "cart" = 8082
    "ordering" = 8083
    "inventory" = 8084
    "profiles" = 8085
    "reviews" = 8086
    "wishlists" = 8087
    "coupons" = 8088
}

$repoRoot = Split-Path -Parent $MyInvocation.MyCommand.Path
Set-Location $repoRoot

Write-Host "Checking infrastructure..." -ForegroundColor Green
# Start infrastructure (if it's not running)
make up

Write-Host "Starting 8 microservices..." -ForegroundColor Green
foreach ($svc in $services) {
    $servicePath = Join-Path $repoRoot "src/services/$svc"
    if (-not (Test-Path $servicePath)) {
        Write-Host "Skipping ${svc}: path not found ($servicePath)" -ForegroundColor Red
        continue
    }

    # Open this specific microservice in a new PowerShell window without closing the terminal
    Start-Process pwsh -WorkingDirectory $servicePath -ArgumentList "-NoExit", "-Command", "`$host.UI.RawUI.WindowTitle='$svc Service'; go run ./cmd/server/"
}
Write-Host "All service terminal windows opened successfully! Wait for 'listening' logs on the black screens." -ForegroundColor Yellow

Write-Host "Healthcheck addresses:" -ForegroundColor Cyan
foreach ($svc in $services) {
    $port = $servicePorts[$svc]
    if (-not $port) {
        continue
    }

    Write-Host ("- {0}: http://localhost:{1}/health | https://localhost/{0}/health" -f $svc, $port)
}
