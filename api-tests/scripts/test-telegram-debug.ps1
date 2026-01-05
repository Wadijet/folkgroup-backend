# Script test và debug gửi thông báo Telegram
# Sử dụng: .\scripts\test-telegram-debug.ps1

$BaseURL = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

Write-Host "`n[DEBUG] Kiểm tra routing và channels cho Telegram" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Headers
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role và organization ID
$roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
    if ($roleResponse.data[0].roleId) {
        $headers["X-Active-Role-ID"] = $roleResponse.data[0].roleId
    }
    if ($roleResponse.data[0].organizationId) {
        $activeOrgID = $roleResponse.data[0].organizationId
        Write-Host "`nActive Organization ID: $activeOrgID" -ForegroundColor Cyan
    }
}

# Kiểm tra Telegram channel
Write-Host "`n[1] Kiểm tra Telegram channel..." -ForegroundColor Yellow
$channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
$telegramChannel = $channelResponse.data | Where-Object { $_.channelType -eq "telegram" } | Select-Object -First 1

if ($telegramChannel) {
    Write-Host "   Channel ID: $($telegramChannel._id)" -ForegroundColor Green
    Write-Host "   Name: $($telegramChannel.name)" -ForegroundColor Green
    Write-Host "   OwnerOrganizationID: $($telegramChannel.ownerOrganizationId)" -ForegroundColor Green
    Write-Host "   ChatIDs: $($telegramChannel.chatIDs -join ', ')" -ForegroundColor Green
    Write-Host "   IsActive: $($telegramChannel.isActive)" -ForegroundColor Green
} else {
    Write-Host "   [ERROR] Không tìm thấy Telegram channel!" -ForegroundColor Red
    exit 1
}

# Kiểm tra routing rules
Write-Host "`n[2] Kiểm tra routing rules..." -ForegroundColor Yellow
$routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers

# Tìm routing rules có OrganizationIDs chứa channel's organization ID
$channelOrgID = $telegramChannel.ownerOrganizationId
$matchingRules = $routingResponse.data | Where-Object {
    $_.isActive -eq $true -and
    $_.organizationIds -and
    $_.organizationIds -contains $channelOrgID
}

Write-Host "   Tổng số routing rules: $($routingResponse.data.Count)" -ForegroundColor Gray
Write-Host "   Rules match với channel org ($channelOrgID): $($matchingRules.Count)" -ForegroundColor $(if ($matchingRules.Count -gt 0) { "Green" } else { "Yellow" })

if ($matchingRules.Count -eq 0) {
    Write-Host "   [WARN] Không có routing rule nào match với channel organization!" -ForegroundColor Yellow
    Write-Host "   Các routing rules hiện có:" -ForegroundColor Yellow
    foreach ($rule in $routingResponse.data | Where-Object { $_.isActive -eq $true } | Select-Object -First 5) {
        $orgIDs = if ($rule.organizationIds) { $rule.organizationIds -join ', ' } else { "empty" }
        Write-Host "     - $($rule.eventType): OrganizationIDs = $orgIDs" -ForegroundColor DarkGray
    }
}

# Tìm eventType có routing rule match
$testEventType = $null
if ($matchingRules.Count -gt 0) {
    $testEventType = $matchingRules[0].eventType
    Write-Host "`n   [OK] Sử dụng eventType: $testEventType" -ForegroundColor Green
} else {
    # Thử với system_error
    $testEventType = "system_error"
    Write-Host "`n   [WARN] Không có rule match, thử với: $testEventType" -ForegroundColor Yellow
}

# Kiểm tra template
Write-Host "`n[3] Kiểm tra template cho eventType: $testEventType..." -ForegroundColor Yellow
$templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
$template = $templateResponse.data | Where-Object {
    $_.eventType -eq $testEventType -and
    $_.channelType -eq "telegram" -and
    ($_.ownerOrganizationId -eq $channelOrgID -or $_.ownerOrganizationId -eq $null)
} | Select-Object -First 1

if ($template) {
    Write-Host "   [OK] Tìm thấy template: $($template.name)" -ForegroundColor Green
    Write-Host "   OwnerOrganizationID: $($template.ownerOrganizationId)" -ForegroundColor Gray
} else {
    Write-Host "   [WARN] Không tìm thấy template cho eventType này!" -ForegroundColor Yellow
}

# Gửi notification
Write-Host "`n[4] Gửi notification..." -ForegroundColor Yellow
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
        Write-Host "   Nguyên nhân có thể:" -ForegroundColor Yellow
        Write-Host "   1. Routing rule không match (OrganizationIDs không chứa channel org)" -ForegroundColor Yellow
        Write-Host "   2. Không có template cho eventType và channel type" -ForegroundColor Yellow
        Write-Host "   3. Channel không active hoặc không có ChatIDs" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
