# Script kiểm tra Telegram sender
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

Write-Host "`n[CHECK] Kiểm tra Telegram Sender" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Lấy System Organization
$orgResponse = Invoke-RestMethod -Uri "$BaseURL/organization/find" -Method GET -Headers $headers
$systemOrg = $orgResponse.data | Where-Object { $_.level -eq -1 -and $_.code -eq "SYSTEM" } | Select-Object -First 1
$systemOrgID = if ($systemOrg._id) { $systemOrg._id } else { $systemOrg.id }

Write-Host "`nSystem Organization ID: $systemOrgID" -ForegroundColor Cyan

# Kiểm tra senders
Write-Host "`n[1] Kiểm tra Telegram senders..." -ForegroundColor Yellow
$senderResponse = Invoke-RestMethod -Uri "$BaseURL/notification/sender/find" -Method GET -Headers $headers
$telegramSenders = $senderResponse.data | Where-Object { $_.channelType -eq "telegram" }

Write-Host "   Tổng số Telegram senders: $($telegramSenders.Count)" -ForegroundColor Gray

$activeSenders = $telegramSenders | Where-Object { $_.isActive -eq $true }
Write-Host "   Senders active: $($activeSenders.Count)" -ForegroundColor $(if ($activeSenders.Count -gt 0) { "Green" } else { "Red" })

if ($activeSenders.Count -eq 0) {
    Write-Host "`n   [ERROR] Không có Telegram sender nào active!" -ForegroundColor Red
    Write-Host "   Các senders hiện có:" -ForegroundColor Yellow
    foreach ($sender in $telegramSenders) {
        $status = if ($sender.isActive) { "ACTIVE" } else { "INACTIVE" }
        Write-Host "     - $($sender.name) ($status, OwnerOrgID: $($sender.ownerOrganizationId))" -ForegroundColor DarkGray
    }
} else {
    Write-Host "`n   [OK] Có $($activeSenders.Count) Telegram sender(s) active:" -ForegroundColor Green
    foreach ($sender in $activeSenders) {
        $ownerID = if ($sender.ownerOrganizationId) { $sender.ownerOrganizationId } else { "null" }
        Write-Host "     - $($sender.name) (OwnerOrgID: $ownerID)" -ForegroundColor DarkGray
    }
}

# Kiểm tra channel
Write-Host "`n[2] Kiểm tra Telegram channel..." -ForegroundColor Yellow
$channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
$telegramChannel = $channelResponse.data | Where-Object { $_.channelType -eq "telegram" } | Select-Object -First 1

if ($telegramChannel) {
    Write-Host "   Channel: $($telegramChannel.name)" -ForegroundColor Green
    Write-Host "   OwnerOrgID: $($telegramChannel.ownerOrganizationId)" -ForegroundColor Gray
    Write-Host "   SenderIDs: $($telegramChannel.senderIDs -join ', ')" -ForegroundColor Gray
    Write-Host "   IsActive: $($telegramChannel.isActive)" -ForegroundColor Gray
}

Write-Host ""
