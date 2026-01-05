# Script test notification trigger cuối cùng
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Notification Trigger Test" -ForegroundColor Magenta
Write-Host ("=" * 50) -ForegroundColor Magenta

# Lấy role ID
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -TimeoutSec 5
    $roleID = $roleResp.data[0].roleId
    $headers["X-Active-Role-ID"] = $roleID
    Write-Host "[OK] Role ID: $roleID" -ForegroundColor Green
} catch {
    Write-Host "[ERROR] Khong the lay role: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Trigger notification
Write-Host "`n[TEST] Triggering notification với system_error..." -ForegroundColor Cyan
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification sau khi fix bug"
        errorCode = "TEST_FIX_001"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$baseURL/notification/trigger" -Method POST -Headers $headers -Body $payload -TimeoutSec 10
    
    Write-Host "`n[RESPONSE]" -ForegroundColor Yellow
    Write-Host "  EventType: $($resp.eventType)" -ForegroundColor Gray
    Write-Host "  Queued: $($resp.queued)" -ForegroundColor $(if ($resp.queued -gt 0) { "Green" } else { "Yellow" })
    Write-Host "  Message: $($resp.message)" -ForegroundColor Gray
    
    if ($resp.queued -gt 0) {
        Write-Host "`n[SUCCESS] Da queue $($resp.queued) notification!" -ForegroundColor Green
        Write-Host "Notification se duoc gui qua Telegram bot" -ForegroundColor Green
    } else {
        Write-Host "`n[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "Hay kiem tra logs server de xem debug logs:" -ForegroundColor Yellow
        Write-Host "  - So luong rules tim thay" -ForegroundColor DarkGray
        Write-Host "  - So luong channels tim thay" -ForegroundColor DarkGray
        Write-Host "  - So luong routes duoc tao" -ForegroundColor DarkGray
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Chi tiet: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

Write-Host ""
