# Script test cuối cùng gửi thông báo Telegram
$BaseURL = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
if ($roleResponse.data -and $roleResponse.data.Count -gt 0 -and $roleResponse.data[0].roleId) {
    $headers["X-Active-Role-ID"] = $roleResponse.data[0].roleId
}

Write-Host "`n[TEST] Gửi thông báo Telegram - Test cuối cùng" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Test với system_error (có template system)
$testEventType = "system_error"
Write-Host "`nGửi notification với eventType: $testEventType" -ForegroundColor Yellow

$payload = @{
    eventType = $testEventType
    payload = @{
        message = "Test notification Telegram - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
        testMode = $true
        errorMessage = "Test error message từ script PowerShell"
        errorCode = "TEST_TELEGRAM_001"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

Write-Host "`nRequest body:" -ForegroundColor Cyan
Write-Host $payload -ForegroundColor DarkGray

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    Write-Host "`n[RESULT]" -ForegroundColor Cyan
    Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor $(if ($response.queued -gt 0) { "Green" } else { "Yellow" })
    
    if ($response.queued -gt 0) {
        Write-Host "`n[SUCCESS] THANH CONG! Da queue $($response.queued) notification(s)!" -ForegroundColor Green
        Write-Host "   Notification se duoc gui qua Telegram trong vai giay" -ForegroundColor Cyan
    } else {
        Write-Host "`n[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "   Co the do:" -ForegroundColor Yellow
        Write-Host "   - Routing rule khong match" -ForegroundColor Yellow
        Write-Host "   - Template khong tim thay (co the do code chua duoc build lai)" -ForegroundColor Yellow
        Write-Host "   - Channel khong active hoac khong co ChatIDs" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($errorDetail) {
            Write-Host "   Code: $($errorDetail.code)" -ForegroundColor Red
            Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
        }
    }
}

Write-Host ""
