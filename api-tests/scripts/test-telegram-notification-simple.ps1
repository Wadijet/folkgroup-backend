# Script test gửi thông báo Telegram đơn giản
# Sử dụng: .\scripts\test-telegram-notification-simple.ps1

param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Bearer token được cung cấp
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

# Headers với token
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Gửi thông báo Telegram qua hệ thống notification" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
Write-Host "Base URL: $BaseURL" -ForegroundColor Cyan
Write-Host "Token: $($token.Substring(0, 50))..." -ForegroundColor Cyan

# Bước 1: Lấy role ID từ API /auth/roles
Write-Host "`n[Bước 1] Lấy role ID từ API..." -ForegroundColor Yellow
$activeRoleID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            Write-Host "   [OK] Đã lấy được role ID: $activeRoleID" -ForegroundColor Green
            $headers["X-Active-Role-ID"] = $activeRoleID
        } else {
            Write-Host "   [WARN] Không tìm thấy roleId trong role đầu tiên" -ForegroundColor Yellow
        }
    } else {
        Write-Host "   [WARN] User không có role nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   [ERROR] Không thể lấy role ID: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "   [WARN] Test sẽ tiếp tục nhưng có thể bị lỗi nếu API yêu cầu X-Active-Role-ID" -ForegroundColor Yellow
}

# Bước 2: Kiểm tra routing rules và channels có sẵn
Write-Host "`n[Bước 2] Kiểm tra routing rules và channels..." -ForegroundColor Yellow

# Kiểm tra routing rules
$availableEventTypes = @()
try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers -ErrorAction Stop
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Host "   [OK] Có $($routingResponse.data.Count) routing rule(s)" -ForegroundColor Green
        foreach ($routing in $routingResponse.data) {
            $eventType = $routing.eventType
            $availableEventTypes += $eventType
            Write-Host "      - $eventType (ChannelID: $($routing.channelId))" -ForegroundColor DarkGray
        }
    } else {
        Write-Host "   [WARN] Không có routing rules nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   [WARN] Không thể lấy routing rules: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Kiểm tra channels (đặc biệt là Telegram)
$telegramChannels = @()
try {
    $channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers -ErrorAction Stop
    if ($channelResponse.data -and $channelResponse.data.Count -gt 0) {
        Write-Host "`n   [OK] Có $($channelResponse.data.Count) channel(s)" -ForegroundColor Green
        foreach ($channel in $channelResponse.data) {
            $channelType = $channel.channelType
            $channelName = $channel.name
            Write-Host "      - $channelName (Type: $channelType)" -ForegroundColor DarkGray
            
            if ($channelType -eq "telegram") {
                $chatIDs = if ($channel.chatIDs) { $channel.chatIDs } else { @() }
                Write-Host "         ChatIDs: $($chatIDs -join ', ')" -ForegroundColor DarkGray
                if ($chatIDs.Count -gt 0) {
                    $telegramChannels += $channel
                }
            }
        }
        
        if ($telegramChannels.Count -eq 0) {
            Write-Host "   [WARN] Không có Telegram channel nào có ChatIDs" -ForegroundColor Yellow
        }
    } else {
        Write-Host "   [WARN] Không có channel nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   [WARN] Không thể lấy channels: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 3: Chọn eventType để test
Write-Host "`n[Bước 3] Chọn eventType để test..." -ForegroundColor Yellow

# Ưu tiên eventType có trong routing rules, nếu không có thì dùng eventType mặc định
$testEventType = $null
if ($availableEventTypes.Count -gt 0) {
    # Tìm eventType có routing rule cho Telegram channel
    foreach ($eventType in $availableEventTypes) {
        $testEventType = $eventType
        Write-Host "   [OK] Sử dụng eventType từ routing rules: $testEventType" -ForegroundColor Green
        break
    }
} else {
    # Dùng eventType mặc định phổ biến
    $testEventType = "system_error"
    Write-Host "   [INFO] Không có routing rules, sử dụng eventType mặc định: $testEventType" -ForegroundColor Cyan
    Write-Host "   [WARN] Cần có routing rule cho eventType này để notification được gửi đi" -ForegroundColor Yellow
}

# Bước 4: Gửi notification
Write-Host "`n[Bước 4] Gửi notification với eventType: $testEventType" -ForegroundColor Yellow

# Tạo payload phù hợp
$payloadData = @{
    timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    message = "Test notification Telegram từ script PowerShell"
    testMode = $true
}

# Thêm thông tin phù hợp với eventType
if ($testEventType -like "*error*") {
    $payloadData["errorMessage"] = "Test error notification qua Telegram"
    $payloadData["errorCode"] = "TEST_TELEGRAM_001"
} elseif ($testEventType -like "*warning*") {
    $payloadData["warningMessage"] = "Test warning notification qua Telegram"
} else {
    $payloadData["message"] = "Test notification Telegram cho eventType: $testEventType"
}

$requestBody = @{
    eventType = $testEventType
    payload = $payloadData
} | ConvertTo-Json -Depth 10

Write-Host "   Request body:" -ForegroundColor Cyan
Write-Host $requestBody -ForegroundColor DarkGray

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $requestBody -ErrorAction Stop
    
    Write-Host "`n   [SUCCESS] Response từ server:" -ForegroundColor Green
    Write-Host "      Message: $($response.message)" -ForegroundColor DarkGray
    Write-Host "      EventType: $($response.eventType)" -ForegroundColor DarkGray
    Write-Host "      Queued: $($response.queued)" -ForegroundColor DarkGray
    
    if ($response.queued -gt 0) {
        Write-Host "`n   ✅ Đã queue thành công $($response.queued) notification(s)!" -ForegroundColor Green
        Write-Host "      Notification sẽ được xử lý bởi delivery worker và gửi qua Telegram" -ForegroundColor Cyan
    } else {
        Write-Host "`n   ⚠️ Không có notification nào được queue" -ForegroundColor Yellow
        Write-Host "      Có thể do:" -ForegroundColor Yellow
        Write-Host "         - Không có routing rule cho eventType '$testEventType'" -ForegroundColor Yellow
        Write-Host "         - Không có template cho eventType và channel type" -ForegroundColor Yellow
        Write-Host "         - Channel không có ChatIDs (với Telegram)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "`n   [ERROR] Lỗi khi gửi request:" -ForegroundColor Red
    Write-Host "      $($_.Exception.Message)" -ForegroundColor Red
    
    if ($_.ErrorDetails.Message) {
        try {
            $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
            if ($errorDetail) {
                Write-Host "      Code: $($errorDetail.code)" -ForegroundColor Red
                Write-Host "      Message: $($errorDetail.message)" -ForegroundColor Red
            } else {
                Write-Host "      Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
            }
        } catch {
            Write-Host "      Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
    }
}

# Bước 5: Kiểm tra history sau 2 giây
Write-Host "`n[Bước 5] Kiểm tra notification history (sau 2 giây)..." -ForegroundColor Yellow
Start-Sleep -Seconds 2

try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers -ErrorAction Stop
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Host "   [OK] Có $($historyResponse.data.Count) notification(s) trong history" -ForegroundColor Green
        
        # Tìm notification gần nhất với eventType vừa test
        $recentNotifications = $historyResponse.data | Where-Object { $_.eventType -eq $testEventType } | Select-Object -First 3
        
        if ($recentNotifications.Count -gt 0) {
            Write-Host "`n   Notification gần nhất với eventType '$testEventType':" -ForegroundColor Cyan
            foreach ($item in $recentNotifications) {
                Write-Host "      - EventType: $($item.eventType)" -ForegroundColor DarkGray
                Write-Host "        Status: $($item.status)" -ForegroundColor DarkGray
                Write-Host "        Channel: $($item.channelType)" -ForegroundColor DarkGray
                if ($item.recipient) {
                    Write-Host "        Recipient: $($item.recipient)" -ForegroundColor DarkGray
                }
                if ($item.createdAt) {
                    Write-Host "        CreatedAt: $($item.createdAt)" -ForegroundColor DarkGray
                }
                Write-Host ""
            }
        } else {
            Write-Host "   [INFO] Chưa có notification nào với eventType '$testEventType' trong history" -ForegroundColor Cyan
        }
    } else {
        Write-Host "   [INFO] Chưa có notification nào trong history" -ForegroundColor Cyan
    }
} catch {
    Write-Host "   [WARN] Không thể lấy history: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "`n" + ("=" * 70) -ForegroundColor Magenta
Write-Host "[DONE] Hoàn thành test gửi thông báo Telegram" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
Write-Host ""
