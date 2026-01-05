# Script test notification trigger với debug đầy đủ
param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[DEBUG] Test Notification Trigger với Debug" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta

# Lấy role và organization
$roleResp = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
$roleID = $roleResp.data[0].roleId
$orgID = $roleResp.data[0].organizationId
$headers["X-Active-Role-ID"] = $roleID

Write-Host "`n[INFO] User Context:" -ForegroundColor Cyan
Write-Host "  Role ID: $roleID" -ForegroundColor Gray
Write-Host "  Organization ID: $orgID" -ForegroundColor Gray

# Kiểm tra routing rule
Write-Host "`n[INFO] Checking routing rule for system_error..." -ForegroundColor Cyan
$routingResp = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
$systemErrorRule = $routingResp.data | Where-Object { $_.eventType -eq "system_error" } | Select-Object -First 1

if ($systemErrorRule) {
    Write-Host "  [OK] Found routing rule:" -ForegroundColor Green
    Write-Host "    OrganizationIDs: $($systemErrorRule.organizationIds -join ', ')" -ForegroundColor Gray
    Write-Host "    ChannelTypes: $($systemErrorRule.channelTypes -join ', ')" -ForegroundColor Gray
    Write-Host "    IsActive: $($systemErrorRule.isActive)" -ForegroundColor Gray
    
    # Kiểm tra channels
    Write-Host "`n[INFO] Checking channels for organization $($systemErrorRule.organizationIds[0])..." -ForegroundColor Cyan
    $channelResp = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    $matchingChannels = $channelResp.data | Where-Object { 
        $_.ownerOrganizationId -eq $systemErrorRule.organizationIds[0] -and 
        $_.isActive -eq $true -and
        ($systemErrorRule.channelTypes -contains $_.channelType)
    }
    
    Write-Host "  [INFO] Found $($matchingChannels.Count) matching active channels:" -ForegroundColor Yellow
    foreach ($ch in $matchingChannels) {
        Write-Host "    - $($ch.name) (Type: $($ch.channelType))" -ForegroundColor Gray
        if ($ch.channelType -eq "telegram") {
            Write-Host "      ChatIDs: $($ch.chatIDs.Count)" -ForegroundColor DarkGray
        }
    }
    
    if ($matchingChannels.Count -eq 0) {
        Write-Host "  [WARN] Không có channel nào match!" -ForegroundColor Red
    }
} else {
    Write-Host "  [ERROR] Không tìm thấy routing rule cho system_error" -ForegroundColor Red
}

# Trigger notification
Write-Host "`n[TEST] Triggering notification..." -ForegroundColor Yellow
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification debug"
        errorCode = "TEST_DEBUG_001"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    Write-Host "`n[RESPONSE]" -ForegroundColor Cyan
    Write-Host "  EventType: $($resp.eventType)" -ForegroundColor Gray
    Write-Host "  Queued: $($resp.queued)" -ForegroundColor $(if ($resp.queued -gt 0) { "Green" } else { "Yellow" })
    Write-Host "  Message: $($resp.message)" -ForegroundColor Gray
    
    if ($resp.queued -eq 0) {
        Write-Host "`n[DEBUG INFO]" -ForegroundColor Yellow
        Write-Host "  Router đã tìm thấy routing rules nhưng không tìm thấy channels" -ForegroundColor Yellow
        Write-Host "  Hoặc channels không có recipients/chatIDs" -ForegroundColor Yellow
        Write-Host "  Hãy kiểm tra logs của server để xem debug logs từ router" -ForegroundColor Yellow
        Write-Host "  Logs sẽ hiển thị:" -ForegroundColor Yellow
        Write-Host "    - Số lượng rules tìm thấy" -ForegroundColor DarkGray
        Write-Host "    - Số lượng channels tìm thấy cho mỗi organization" -ForegroundColor DarkGray
        Write-Host "    - Số lượng routes được tạo" -ForegroundColor DarkGray
    } else {
        Write-Host "`n[SUCCESS] Đã queue $($resp.queued) notification!" -ForegroundColor Green
    }
} catch {
    Write-Host "`n[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
