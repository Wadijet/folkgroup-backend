# Script chạy test cho Module 1 (Content Storage)
# Sử dụng: .\api-tests\scripts\test-content-storage.ps1

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TEST MODULE 1 - CONTENT STORAGE" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Đảm bảo chạy từ thư mục gốc của project
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent (Split-Path -Parent $scriptDir)
Set-Location $projectRoot

# Kiểm tra server có đang chạy không
Write-Host "[INFO] Kiểm tra server có đang chạy..." -ForegroundColor Yellow
try {
    $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/health" -Method GET -TimeoutSec 2 -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
        Write-Host "[OK] Server đang chạy" -ForegroundColor Green
    }
} catch {
    Write-Host "[ERROR] Server chưa chạy hoặc không thể kết nối!" -ForegroundColor Red
    Write-Host "[INFO] Vui lòng chạy server trước:" -ForegroundColor Yellow
    Write-Host "  cd api" -ForegroundColor Yellow
    Write-Host "  go run cmd/server/main.go" -ForegroundColor Yellow
    exit 1
}

# Chạy test
Write-Host ""
Write-Host "[INFO] Bắt đầu chạy test Module 1 (Content Storage)..." -ForegroundColor Yellow
Write-Host ""

Set-Location "$projectRoot\api-tests"

# Chạy test với verbose output
go test -v -run TestContentStorageModule ./cases

$exitCode = $LASTEXITCODE

Write-Host ""
if ($exitCode -eq 0) {
    Write-Host "[SUCCESS] Test Module 1 hoàn thành thành công!" -ForegroundColor Green
} else {
    Write-Host "[FAILED] Test Module 1 có lỗi (exit code: $exitCode)" -ForegroundColor Red
}

Set-Location $projectRoot
exit $exitCode
