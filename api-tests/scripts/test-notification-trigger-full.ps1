# Script test gửi thông báo qua /notification/trigger với kiểm tra đầy đủ
# Sử dụng: .\scripts\test-notification-trigger-full.ps1

param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Token được cung cấp
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

# Headers với token
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Gửi thông báo qua /notification/trigger (Hệ thống 2)" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Host "Base URL: $BaseURL" -ForegroundColor Cyan
Write-Host "Token: $($token.Substring(0, 30))..." -ForegroundColor Cyan

# Bước 1: Lấy role ID và organization ID
Write-Host "`n[Bước 1] Lấy role ID và organization ID từ API /auth/roles..." -ForegroundColor Yellow
$activeRoleID = $null
$activeOrganizationID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            Write-Host "[OK] Đã lấy được role ID: $activeRoleID" -ForegroundColor Green
            $headers["X-Active-Role-ID"] = $activeRoleID
        }
        if ($firstRole.organizationId) {
            $activeOrganizationID = $firstRole.organizationId
            Write-Host "[OK] Active Organization ID: $activeOrganizationID" -ForegroundColor Green
        }
    }
} catch {
    Write-Host "[ERROR] Không thể lấy role ID: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Bước 2: Kiểm tra channels chi tiết
Write-Host "`n[Bước 2] Kiểm tra channels chi tiết..." -ForegroundColor Yellow
$telegramChannel = $null
try {
    $channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    if ($channelResponse.data -and $channelResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($channelResponse.data.Count) channel(s)" -ForegroundColor Green
        foreach ($channel in $channelResponse.data) {
            if ($channel.channelType -eq "telegram") {
                $telegramChannel = $channel
                Write-Host "   [TELEGRAM] $($channel.name)" -ForegroundColor Cyan
                Write-Host "      - ChannelID: $($channel._id)" -ForegroundColor DarkGray
                Write-Host "      - Recipients: $($channel.recipients.Count)" -ForegroundColor DarkGray
                if ($channel.recipients) {
                    Write-Host "      - Recipient list: $($channel.recipients -join ', ')" -ForegroundColor DarkGray
                }
                Write-Host "      - ChatIDs: $($channel.chatIDs.Count)" -ForegroundColor DarkGray
                if ($channel.chatIDs) {
                    Write-Host "      - ChatID list: $($channel.chatIDs -join ', ')" -ForegroundColor DarkGray
                }
                Write-Host "      - OwnerOrganizationID: $($channel.ownerOrganizationId)" -ForegroundColor DarkGray
            } else {
                Write-Host "   [$($channel.channelType)] $($channel.name) (Recipients: $($channel.recipients.Count))" -ForegroundColor Gray
            }
        }
    }
} catch {
    Write-Host "[WARN] Không thể lấy channels: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 3: Kiểm tra routing rules chi tiết
Write-Host "`n[Bước 3] Kiểm tra routing rules chi tiết..." -ForegroundColor Yellow
$matchingRoutes = @()
try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($routingResponse.data.Count) routing rule(s)" -ForegroundColor Green
        foreach ($routing in $routingResponse.data) {
            $orgId = if ($routing.ownerOrganizationId) { $routing.ownerOrganizationId } else { "null (system)" }
            $channelId = if ($routing.channelId) { $routing.channelId } else { "null" }
            $orgIDs = if ($routing.organizationIds) { $routing.organizationIds -join ', ' } else { "empty" }
            Write-Host "   - EventType: $($routing.eventType)" -ForegroundColor DarkGray
            Write-Host "     ChannelID: $channelId" -ForegroundColor DarkGray
            Write-Host "     OrganizationIDs: $orgIDs" -ForegroundColor DarkGray
            Write-Host "     ChannelTypes: $($routing.channelTypes -join ', ')" -ForegroundColor DarkGray
            Write-Host "     IsActive: $($routing.isActive)" -ForegroundColor DarkGray
            
            # Kiểm tra xem routing này có match với telegram channel không
            $channelOrgId = if ($telegramChannel.ownerOrganizationId) { $telegramChannel.ownerOrganizationId } else { $null }
            if ($telegramChannel -and $routing.organizationIds -and $channelOrgId) {
                if ($routing.organizationIds -contains $channelOrgId) {
                    $matchingRoutes += $routing
                    Write-Host "     [MATCH] Routing này match với Telegram channel organization!" -ForegroundColor Green
                    if ($activeOrganizationID -and $routing.organizationIds -contains $activeOrganizationID) {
                        Write-Host "     [MATCH] Routing này cũng match với active organization của user!" -ForegroundColor Green
                    } else {
                        Write-Host "     [WARN] Routing không match với active organization của user ($activeOrganizationID)" -ForegroundColor Yellow
                    }
                } else {
                    Write-Host "     [NO MATCH] Channel org ($channelOrgId) không có trong OrganizationIDs" -ForegroundColor Yellow
                }
            }
        }
    }
} catch {
    Write-Host "[WARN] Không thể lấy routing rules: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 4: Kiểm tra templates
Write-Host "`n[Bước 4] Kiểm tra templates..." -ForegroundColor Yellow
try {
    $templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
    if ($templateResponse.data -and $templateResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($templateResponse.data.Count) template(s)" -ForegroundColor Green
        $telegramTemplates = $templateResponse.data | Where-Object { $_.channelType -eq "telegram" }
        Write-Host "   - Telegram templates: $($telegramTemplates.Count)" -ForegroundColor Gray
        foreach ($template in $telegramTemplates) {
            Write-Host "     - $($template.name) (EventType: $($template.eventType))" -ForegroundColor DarkGray
        }
    }
} catch {
    Write-Host "[WARN] Không thể lấy templates: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 5: Test trigger với eventType có routing rule
Write-Host "`n[Bước 5] Test trigger notification với eventType: system_error" -ForegroundColor Yellow
if ($matchingRoutes.Count -eq 0 -and $routingResponse.data) {
    # Nếu không có routing match, dùng eventType đầu tiên có routing rule
    $testEventType = $routingResponse.data[0].eventType
    Write-Host "   (Không có routing match với telegram channel, dùng eventType: $testEventType)" -ForegroundColor Cyan
} else {
    $testEventType = "system_error"
}

try {
    $payload = @{
        eventType = $testEventType
        payload = @{
            errorMessage = "Test notification qua hệ thống notification trigger"
            errorCode = "TEST_TRIGGER_001"
            timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
            description = "Đây là thông báo test từ script PowerShell qua endpoint /notification/trigger"
        }
    } | ConvertTo-Json -Depth 10

    Write-Host "   Payload:" -ForegroundColor Cyan
    Write-Host ($payload | ConvertFrom-Json | ConvertTo-Json -Depth 10) -ForegroundColor Gray

    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    Write-Host "`n[OK] Gửi thông báo thành công!" -ForegroundColor Green
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Số lượng notification đã queue: $($response.queued)" -ForegroundColor Gray
    Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    
    if ($response.queued -gt 0) {
        Write-Host "`n[SUCCESS] Đã queue $($response.queued) notification thành công!" -ForegroundColor Green
    } else {
        Write-Host "`n[WARN] Không có notification nào được queue" -ForegroundColor Yellow
        Write-Host "   Có thể do:" -ForegroundColor Yellow
        Write-Host "   - Routing rule không match với organization của user" -ForegroundColor Yellow
        Write-Host "   - Channel không có recipients/chatIDs" -ForegroundColor Yellow
        Write-Host "   - Không có template phù hợp" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n[ERROR] Lỗi khi gửi thông báo: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($errorDetail) {
            Write-Host "   Code: $($errorDetail.code)" -ForegroundColor Red
            Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
        } else {
            Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
    }
}

# Bước 6: Kiểm tra history sau khi trigger
Write-Host "`n[Bước 6] Kiểm tra notification history..." -ForegroundColor Yellow
Start-Sleep -Seconds 3
try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($historyResponse.data.Count) notification(s) trong history" -ForegroundColor Green
        
        # Hiển thị 5 notification gần nhất
        $maxShow = [Math]::Min(5, $historyResponse.data.Count)
        Write-Host "`n   $maxShow notification gần nhất:" -ForegroundColor Gray
        for ($i = 0; $i -lt $maxShow; $i++) {
            $item = $historyResponse.data[$i]
            Write-Host "     [$($i+1)] EventType: $($item.eventType) | Status: $($item.status) | Channel: $($item.channelType)" -ForegroundColor DarkGray
            if ($item.recipient) {
                Write-Host "         Recipient: $($item.recipient)" -ForegroundColor DarkGray
            }
            if ($item.createdAt) {
                $date = [DateTimeOffset]::FromUnixTimeMilliseconds($item.createdAt).LocalDateTime
                Write-Host "         CreatedAt: $($date.ToString('yyyy-MM-dd HH:mm:ss'))" -ForegroundColor DarkGray
            }
        }
    } else {
        Write-Host "[INFO] Chưa có notification nào trong history" -ForegroundColor Cyan
    }
} catch {
    Write-Host "[WARN] Không thể lấy history: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "`n" + ("=" * 60) -ForegroundColor Magenta
Write-Host "[DONE] Hoàn thành test notification trigger" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Host ""
