# Script test query channels trực tiếp để debug
param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResp = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
$roleID = $roleResp.data[0].roleId
$headers["X-Active-Role-ID"] = $roleID

# Lấy organization ID từ role
$orgID = $roleResp.data[0].organizationId
Write-Host "[INFO] Role Organization ID: $orgID" -ForegroundColor Cyan

# Query channels
Write-Host "`n[TEST] Query channels..." -ForegroundColor Yellow
try {
    $channelResp = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    Write-Host "[OK] Tìm thấy $($channelResp.data.Count) channel(s)" -ForegroundColor Green
    
    foreach ($channel in $channelResp.data) {
        Write-Host "`n  Channel: $($channel.name)" -ForegroundColor Cyan
        Write-Host "    - ID: $($channel._id)" -ForegroundColor Gray
        Write-Host "    - Type: $($channel.channelType)" -ForegroundColor Gray
        Write-Host "    - OwnerOrganizationID: $($channel.ownerOrganizationId)" -ForegroundColor Gray
        Write-Host "    - IsActive: $($channel.isActive)" -ForegroundColor Gray
        if ($channel.channelType -eq "telegram") {
            Write-Host "    - ChatIDs: $($channel.chatIDs.Count)" -ForegroundColor Gray
            if ($channel.chatIDs) {
                Write-Host "      ChatIDs: $($channel.chatIDs -join ', ')" -ForegroundColor DarkGray
            }
        }
        if ($channel.channelType -eq "email") {
            Write-Host "    - Recipients: $($channel.recipients.Count)" -ForegroundColor Gray
        }
        
        # Kiểm tra xem channel này có match với routing rule organization không
        $routingOrgID = "695aa015c122aac1e4cd28aa"
        if ($channel.ownerOrganizationId -eq $routingOrgID) {
            Write-Host "    [MATCH] Channel này match với routing rule organization!" -ForegroundColor Green
        } else {
            Write-Host "    [NO MATCH] Channel org ($($channel.ownerOrganizationId)) != routing org ($routingOrgID)" -ForegroundColor Yellow
        }
    }
} catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

# Query routing rules
Write-Host "`n[TEST] Query routing rules cho system_error..." -ForegroundColor Yellow
try {
    $routingResp = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find?eventType=system_error" -Method GET -Headers $headers
    Write-Host "[OK] Tìm thấy $($routingResp.data.Count) routing rule(s)" -ForegroundColor Green
    
    foreach ($routing in $routingResp.data) {
        Write-Host "`n  Routing Rule:" -ForegroundColor Cyan
        Write-Host "    - EventType: $($routing.eventType)" -ForegroundColor Gray
        Write-Host "    - OrganizationIDs: $($routing.organizationIds -join ', ')" -ForegroundColor Gray
        Write-Host "    - ChannelTypes: $($routing.channelTypes -join ', ')" -ForegroundColor Gray
        Write-Host "    - IsActive: $($routing.isActive)" -ForegroundColor Gray
    }
} catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
