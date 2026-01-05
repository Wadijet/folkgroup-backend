# Script test gửi thông báo Telegram bằng curl
# Sử dụng: .\scripts\test-telegram-curl.ps1

$BaseURL = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

Write-Host "`n[TEST] Test gửi thông báo Telegram" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Lấy role ID
Write-Host "`nLấy role ID..." -ForegroundColor Yellow
$roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

$activeRoleID = $null
if ($roleResponse.data -and $roleResponse.data.Count -gt 0 -and $roleResponse.data[0].roleId) {
    $activeRoleID = $roleResponse.data[0].roleId
    Write-Host "[OK] Role ID: $activeRoleID" -ForegroundColor Green
}

# Headers với role ID
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}
if ($activeRoleID) {
    $headers["X-Active-Role-ID"] = $activeRoleID
}

# Kiểm tra templates
Write-Host "`nKiểm tra templates Telegram..." -ForegroundColor Yellow
$templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
$telegramTemplates = $templateResponse.data | Where-Object { $_.channelType -eq "telegram" }

$testEventType = "system_error"
if ($telegramTemplates.Count -gt 0) {
    $testEventType = $telegramTemplates[0].eventType
    Write-Host "[OK] Sử dụng eventType: $testEventType" -ForegroundColor Green
} else {
    Write-Host "[WARN] Không có template Telegram, dùng system_error" -ForegroundColor Yellow
}

# Gửi notification
Write-Host "`nGửi notification với eventType: $testEventType" -ForegroundColor Yellow

$payload = @{
    eventType = $testEventType
    payload = @{
        message = "Test notification Telegram - $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
        testMode = $true
        errorMessage = "Test error message"
        errorCode = "TEST_001"
    }
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    Write-Host "`n[RESULT]" -ForegroundColor Cyan
    Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor $(if ($response.queued -gt 0) { "Green" } else { "Yellow" })
    
    if ($response.queued -gt 0) {
        Write-Host "`n[SUCCESS] Đã queue $($response.queued) notification(s)!" -ForegroundColor Green
    } else {
        Write-Host "`n[WARN] Không có notification nào được queue" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
