# Script test nhanh gửi thông báo Telegram
# Sử dụng: .\scripts\test-telegram-quick.ps1

param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Bearer token
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Test gửi thông báo Telegram" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Lấy role ID
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0 -and $roleResponse.data[0].roleId) {
        $headers["X-Active-Role-ID"] = $roleResponse.data[0].roleId
        Write-Host "[OK] Đã lấy role ID" -ForegroundColor Green
    }
} catch {
    Write-Host "[WARN] Không lấy được role ID" -ForegroundColor Yellow
}

# Kiểm tra templates cho Telegram
Write-Host "`nKiểm tra templates Telegram..." -ForegroundColor Yellow
try {
    $templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers -ErrorAction Stop
    $telegramTemplates = $templateResponse.data | Where-Object { $_.channelType -eq "telegram" }
    
    if ($telegramTemplates.Count -gt 0) {
        Write-Host "[OK] Có $($telegramTemplates.Count) template Telegram" -ForegroundColor Green
        $testEventType = $telegramTemplates[0].eventType
        Write-Host "   Sử dụng eventType: $testEventType" -ForegroundColor Cyan
    } else {
        Write-Host "[WARN] Không có template Telegram, thử với system_error" -ForegroundColor Yellow
        $testEventType = "system_error"
    }
} catch {
    Write-Host "[WARN] Không lấy được templates, thử với system_error" -ForegroundColor Yellow
    $testEventType = "system_error"
}

# Gửi notification
Write-Host "`nGửi notification với eventType: $testEventType" -ForegroundColor Yellow

$requestBody = @{
    eventType = $testEventType
    payload = @{
        message = "Test notification Telegram - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
        testMode = $true
        errorMessage = "Test error message"
        errorCode = "TEST_001"
    }
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $requestBody -ErrorAction Stop
    
    Write-Host "`n[RESULT]" -ForegroundColor Cyan
    Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor $(if ($response.queued -gt 0) { "Green" } else { "Yellow" })
    
    if ($response.queued -gt 0) {
        Write-Host "`n✅ THÀNH CÔNG! Đã queue $($response.queued) notification(s)" -ForegroundColor Green
        Write-Host "   Notification sẽ được gửi qua Telegram trong vài giây" -ForegroundColor Cyan
    } else {
        Write-Host "`n⚠️ Không có notification nào được queue" -ForegroundColor Yellow
        Write-Host "   Có thể do:" -ForegroundColor Yellow
        Write-Host "   - Routing rule không match với organization/channel" -ForegroundColor Yellow
        Write-Host "   - Không có template cho eventType này" -ForegroundColor Yellow
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
Write-Host "============================================================" -ForegroundColor Magenta
