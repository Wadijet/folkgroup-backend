param(
    [string]$Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"
)

$baseURL = "http://localhost:8080/api/v1"
$headers = @{}
$headers.Add("Authorization", "Bearer $Token")
$headers.Add("Content-Type", "application/json")

Write-Host ""
Write-Host "[TEST] Test Notification Routing (Cach 2)" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

# Lay role ID
Write-Host "[INFO] Lay role ID..." -ForegroundColor Cyan
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers
    $roleID = $roleResp.data[0].roleId
    $headers.Add("X-Active-Role-ID", $roleID)
    Write-Host "[OK] Role ID: $roleID" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Khong the lay role ID: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Test trigger voi system_error
Write-Host ""
Write-Host "[TEST] Trigger notification voi system_error..." -ForegroundColor Cyan

$payloadObj = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification qua routing system"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
}

$payload = $payloadObj | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$baseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    Write-Host ""
    if ($resp.queued -gt 0) {
        Write-Host "[OK] Da queue $($resp.queued) notification(s)!" -ForegroundColor Green
        Write-Host "   EventType: $($resp.eventType)" -ForegroundColor Gray
        Write-Host "   Message: $($resp.message)" -ForegroundColor Gray
        Write-Host ""
        Write-Host "[INFO] Vui long kiem tra Telegram trong vong 10-30 giay..." -ForegroundColor Cyan
    } else {
        Write-Host "[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "   Message: $($resp.message)" -ForegroundColor Gray
        Write-Host "   EventType: $($resp.eventType)" -ForegroundColor Gray
    }
} catch {
    Write-Host "[ERROR] Loi: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiet: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

Write-Host ""
