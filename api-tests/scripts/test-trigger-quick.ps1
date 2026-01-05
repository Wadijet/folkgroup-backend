# Script test nhanh notification trigger
param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Trigger notification..." -ForegroundColor Cyan

# Lấy role ID với timeout
try {
    $roleResp = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers -TimeoutSec 5
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "[OK] Role ID: $roleID" -ForegroundColor Green
    } else {
        Write-Host "[ERROR] Khong co role" -ForegroundColor Red
        exit 1
    }
} catch {
    Write-Host "[ERROR] Khong the lay role: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Trigger notification với timeout
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification"
        errorCode = "TEST_001"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload -TimeoutSec 10
    Write-Host "`n[RESPONSE]" -ForegroundColor Yellow
    Write-Host "  EventType: $($resp.eventType)" -ForegroundColor Gray
    Write-Host "  Queued: $($resp.queued)" -ForegroundColor $(if ($resp.queued -gt 0) { "Green" } else { "Yellow" })
    Write-Host "  Message: $($resp.message)" -ForegroundColor Gray
    
    if ($resp.queued -gt 0) {
        Write-Host "`n[SUCCESS] Da queue $($resp.queued) notification!" -ForegroundColor Green
    } else {
        Write-Host "`n[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "Hay kiem tra logs server de xem debug logs" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Chi tiet: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

Write-Host ""
