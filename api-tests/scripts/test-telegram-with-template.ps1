# Script test gửi thông báo Telegram với eventType có template
# Sử dụng: .\scripts\test-telegram-with-template.ps1

$BaseURL = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

Write-Host "`n[TEST] Gửi thông báo Telegram với eventType có template" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
if ($roleResponse.data -and $roleResponse.data.Count -gt 0 -and $roleResponse.data[0].roleId) {
    $headers["X-Active-Role-ID"] = $roleResponse.data[0].roleId
}

# Lấy Telegram channel
$channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
$telegramChannel = $channelResponse.data | Where-Object { $_.channelType -eq "telegram" } | Select-Object -First 1
$channelOrgID = $telegramChannel.ownerOrganizationId

Write-Host "`nTelegram Channel Organization ID: $channelOrgID" -ForegroundColor Cyan

# Lấy tất cả templates Telegram
Write-Host "`nTìm templates Telegram..." -ForegroundColor Yellow
$templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
$telegramTemplates = $templateResponse.data | Where-Object { $_.channelType -eq "telegram" }

Write-Host "   Tìm thấy $($telegramTemplates.Count) template(s) Telegram" -ForegroundColor $(if ($telegramTemplates.Count -gt 0) { "Green" } else { "Yellow" })

if ($telegramTemplates.Count -eq 0) {
    Write-Host "   [ERROR] Không có template Telegram nào!" -ForegroundColor Red
    Write-Host "   Cần tạo template cho Telegram trước khi test" -ForegroundColor Yellow
    exit 1
}

# Hiển thị các template có sẵn
Write-Host "`n   Các template Telegram có sẵn:" -ForegroundColor Cyan
foreach ($tpl in $telegramTemplates) {
    $orgMatch = if ($tpl.ownerOrganizationId -eq $channelOrgID) { "Match org" } elseif ($tpl.ownerOrganizationId -eq $null) { "Global" } else { "Other org" }
    Write-Host "     - $($tpl.eventType) ($orgMatch)" -ForegroundColor DarkGray
}

# Chọn eventType từ template
$testEventType = $telegramTemplates[0].eventType
Write-Host "`n   Sử dụng eventType: $testEventType" -ForegroundColor Green

# Kiểm tra routing rule cho eventType này
Write-Host "`nKiểm tra routing rule cho eventType: $testEventType" -ForegroundColor Yellow
$routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
$matchingRule = $routingResponse.data | Where-Object {
    $_.eventType -eq $testEventType -and
    $_.isActive -eq $true -and
    $_.organizationIds -and
    $_.organizationIds -contains $channelOrgID
} | Select-Object -First 1

if ($matchingRule) {
    Write-Host "   [OK] Có routing rule match!" -ForegroundColor Green
} else {
    Write-Host "   [WARN] Không có routing rule match cho eventType này" -ForegroundColor Yellow
    Write-Host "   Nhưng vẫn thử gửi..." -ForegroundColor Yellow
}

# Gửi notification
Write-Host "`nGửi notification..." -ForegroundColor Yellow
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
        Write-Host "`n[SUCCESS] THANH CONG! Da queue $($response.queued) notification(s)!" -ForegroundColor Green
        Write-Host "   Notification se duoc gui qua Telegram trong vai giay" -ForegroundColor Cyan
    } else {
        Write-Host "`n[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
