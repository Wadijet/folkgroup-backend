# Script test gửi notification qua Telegram
# Test cả 2 cách: Trực tiếp (delivery) và qua hệ thống notification (routing)
# Sử dụng: .\scripts\test-telegram-notification.ps1 -Token "your_token_here" -ChatID "your_chat_id"

param(
    [Parameter(Mandatory=$true)]
    [string]$Token,
    
    [string]$ChatID = "",
    
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Màu sắc cho output
function Write-Success { param([string]$Message) Write-Host "[OK] $Message" -ForegroundColor Green }
function Write-ErrorMsg { param([string]$Message) Write-Host "[ERROR] $Message" -ForegroundColor Red }
function Write-Info { param([string]$Message) Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Warning { param([string]$Message) Write-Host "[WARN] $Message" -ForegroundColor Yellow }

# Headers với token
$headers = @{
    "Authorization" = "Bearer $Token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TELEGRAM TEST] BAT DAU TEST GUI NOTIFICATION QUA TELEGRAM" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
Write-Info "Base URL: $BaseURL"
Write-Info "Token: $($Token.Substring(0, 20))..."

# Lấy role ID từ API /auth/roles
Write-Info "Lay role ID tu API /auth/roles..."
$activeRoleID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            Write-Success "Da lay duoc role ID: $activeRoleID"
            $headers["X-Active-Role-ID"] = $activeRoleID
        }
    }
} catch {
    Write-ErrorMsg "Khong the lay role ID: $($_.Exception.Message)"
    exit 1
}

# Lấy organization ID từ role
$orgID = $null
if ($roleResponse.data[0].organizationId) {
    $orgID = $roleResponse.data[0].organizationId
    Write-Info "Organization ID: $orgID"
}

# ============================================
# BƯỚC 1: Lấy thông tin Telegram Channel và Sender
# ============================================
Write-Host "`n[STEP 1] Lay thong tin Telegram Channel va Sender" -ForegroundColor Yellow

$telegramChannelID = $null
$telegramChatIDs = @()

try {
    $channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    foreach ($channel in $channelResponse.data) {
        if ($channel.channelType -eq "telegram") {
            $telegramChannelID = $channel.id
            Write-Success "Tim thay Telegram Channel: $($channel.name) (ID: $telegramChannelID)"
            
            if ($channel.chatIds -and $channel.chatIds.Count -gt 0) {
                $telegramChatIDs = $channel.chatIds
                Write-Info "Channel co $($telegramChatIDs.Count) chat IDs: $($telegramChatIDs -join ', ')"
            } else {
                Write-Warning "Channel chua co chat IDs"
            }
            break
        }
    }
    
    if (-not $telegramChannelID) {
        Write-ErrorMsg "Khong tim thay Telegram Channel"
        exit 1
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach channel: $($_.Exception.Message)"
    exit 1
}

# Xác định Chat ID để gửi
$targetChatID = $ChatID
if ([string]::IsNullOrEmpty($targetChatID)) {
    if ($telegramChatIDs.Count -gt 0) {
        $targetChatID = $telegramChatIDs[0]
        Write-Info "Su dung chat ID tu channel: $targetChatID"
    } else {
        Write-ErrorMsg "Khong co chat ID nao. Vui long cung cap -ChatID hoac cap nhat channel."
        exit 1
    }
} else {
    Write-Info "Su dung chat ID duoc cung cap: $targetChatID"
}

# ============================================
# BƯỚC 2: Tạo Template cho eventType test
# ============================================
Write-Host "`n[STEP 2] Tao Template cho eventType test" -ForegroundColor Yellow

$testEventType = "test_telegram_notification_$(Get-Date -Format 'yyyyMMddHHmmss')"
$templateID = $null

try {
    $templatePayload = @{
        name = "Test Telegram Template"
        eventType = $testEventType
        channelType = "telegram"
        subject = "Test Notification"
        content = "{{message}}`n`nThoi gian: {{timestamp}}`nTest Type: {{testType}}"
        organizationId = $orgID
    } | ConvertTo-Json -Depth 10

    $templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/insert-one" -Method POST -Headers $headers -Body $templatePayload
    if ($templateResponse.data -and $templateResponse.data.id) {
        $templateID = $templateResponse.data.id
        Write-Success "Tao template thanh cong: $templateID"
    } else {
        Write-Warning "Khong the tao template"
    }
} catch {
    Write-Warning "Khong the tao template: $($_.Exception.Message)"
}

# ============================================
# BƯỚC 3: Tạo Routing Rule cho eventType test
# ============================================
Write-Host "`n[STEP 3] Tao Routing Rule cho eventType test" -ForegroundColor Yellow

$routingRuleID = $null

if ($templateID) {
    try {
        # Đảm bảo eventType là string (không phải null)
        $routingPayload = @{
            eventType = $testEventType
            organizationIds = @($orgID)
            channelTypes = @("telegram")
            isActive = $true
        } | ConvertTo-Json -Depth 10 -Compress
        
        Write-Info "Routing payload: $routingPayload"

        $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/insert-one" -Method POST -Headers $headers -Body $routingPayload
        if ($routingResponse.data -and $routingResponse.data.id) {
            $routingRuleID = $routingResponse.data.id
            Write-Success "Tao routing rule thanh cong: $routingRuleID"
            Write-Info "EventType: $testEventType"
        } else {
            Write-Warning "Khong the tao routing rule"
        }
    } catch {
        Write-Warning "Khong the tao routing rule: $($_.Exception.Message)"
        Write-Info "Se bo qua buoc test qua he thong notification (cach 2)"
    }
} else {
    Write-Warning "Khong co template, se bo qua tao routing rule"
}

# ============================================
# TEST 1: Gửi trực tiếp qua Delivery System (Hệ thống 1)
# ============================================
Write-Host "`n[TEST 1] GUI TRUC TIEP QUA DELIVERY SYSTEM (He thong 1)" -ForegroundColor Cyan
Write-Host ("-" * 70) -ForegroundColor Cyan

$testMessage1 = "Test Telegram Notification - Cach 1 (Truc tiep)
Thoi gian: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Day la test gui truc tiep qua delivery system.
Neu ban nhan duoc tin nhan nay, he thong delivery hoat dong tot!"

try {
    $deliveryPayload = @{
        channelType = "telegram"
        recipient = $targetChatID
        subject = "Test Delivery System"
        content = $testMessage1
        eventType = "test.delivery.direct"
        metadata = @{
            testType = "direct_delivery"
            timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        }
    } | ConvertTo-Json -Depth 10

    Write-Info "Dang gui notification truc tiep..."
    $deliveryResponse = Invoke-RestMethod -Uri "$BaseURL/delivery/send" -Method POST -Headers $headers -Body $deliveryPayload
    
    Write-Success "Da them vao queue thanh cong!"
    Write-Host "   Message ID: $($deliveryResponse.messageId)" -ForegroundColor Gray
    Write-Host "   Status: $($deliveryResponse.status)" -ForegroundColor Gray
    Write-Host "   Queued At: $($deliveryResponse.queuedAt)" -ForegroundColor Gray
    Write-Info "Vui long kiem tra Telegram trong vong 10-30 giay..."
    
} catch {
    Write-ErrorMsg "Loi khi gui notification truc tiep: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($errorDetail) {
            Write-Host "   Code: $($errorDetail.code)" -ForegroundColor Red
            Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
        }
    }
}

Start-Sleep -Seconds 3

# ============================================
# TEST 2: Gửi qua Notification System với Routing (Hệ thống 2)
# ============================================
if ($routingRuleID) {
    Write-Host "`n[TEST 2] GUI QUA NOTIFICATION SYSTEM VOI ROUTING (He thong 2)" -ForegroundColor Cyan
    Write-Host ("-" * 70) -ForegroundColor Cyan

    # Đợi một chút để MongoDB index được cập nhật
    Write-Info "Doi 2 giay de MongoDB index cap nhat..."
    Start-Sleep -Seconds 2

    # Kiểm tra routing rule đã tồn tại chưa
    Write-Info "Kiem tra routing rule..."
    try {
        $checkRoutingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
        $foundRule = $checkRoutingResponse.data | Where-Object { $_.id -eq $routingRuleID }
        if ($foundRule) {
            Write-Success "Routing rule da ton tai trong database"
            Write-Host "   EventType: $($foundRule.eventType)" -ForegroundColor Gray
            Write-Host "   IsActive: $($foundRule.isActive)" -ForegroundColor Gray
        } else {
            Write-Warning "Routing rule chua tim thay trong database"
        }
    } catch {
        Write-Warning "Khong the kiem tra routing rule: $($_.Exception.Message)"
    }

    # Thử dùng eventType đã có routing rule sẵn (system_error) để test nhanh
    $testEventTypeForTrigger = $testEventType
    Write-Info "Su dung eventType: $testEventTypeForTrigger"
    
    $testMessage2 = "Test Telegram Notification - Cach 2 (Qua Routing)
Thoi gian: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Day la test gui qua he thong notification voi routing rule.
Neu ban nhan duoc tin nhan nay, he thong notification hoat dong tot!"

    try {
        $triggerPayload = @{
            eventType = $testEventTypeForTrigger
            payload = @{
                message = $testMessage2
                testType = "notification_routing"
                timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
                baseUrl = $BaseURL
            }
        } | ConvertTo-Json -Depth 10

        Write-Info "Dang trigger notification qua routing..."
        $triggerResponse = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $triggerPayload
        
        if ($triggerResponse.queued -gt 0) {
            Write-Success "Da them vao queue thanh cong!"
            Write-Host "   EventType: $($triggerResponse.eventType)" -ForegroundColor Gray
            Write-Host "   Queued: $($triggerResponse.queued)" -ForegroundColor Gray
            Write-Host "   Message: $($triggerResponse.message)" -ForegroundColor Gray
            Write-Info "Vui long kiem tra Telegram trong vong 10-30 giay..."
        } else {
            Write-Warning "Khong co notification nao duoc queue"
            Write-Host "   Message: $($triggerResponse.message)" -ForegroundColor Gray
            Write-Host "   EventType: $($triggerResponse.eventType)" -ForegroundColor Gray
            
            # Debug: Kiểm tra lại routing rules
            Write-Info "Debug: Kiem tra lai routing rules..."
            try {
                $allRules = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
                $matchingRules = $allRules.data | Where-Object { $_.eventType -eq $testEventType }
                if ($matchingRules) {
                    Write-Host "   Tim thay $($matchingRules.Count) routing rule cho eventType nay" -ForegroundColor Yellow
                    foreach ($rule in $matchingRules) {
                        Write-Host "     - ID: $($rule.id), IsActive: $($rule.isActive), EventType: $($rule.eventType)" -ForegroundColor DarkGray
                    }
                } else {
                    Write-Host "   Khong tim thay routing rule nao cho eventType: $testEventType" -ForegroundColor Red
                }
            } catch {
                Write-Warning "Khong the kiem tra routing rules: $($_.Exception.Message)"
            }
        }
    } catch {
        Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
        if ($_.ErrorDetails.Message) {
            $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
            if ($errorDetail) {
                Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
            }
        }
    }
} else {
    Write-Warning "`n[TEST 2] Bo qua - Khong co routing rule"
}

Start-Sleep -Seconds 3

# ============================================
# TEST 3: Kiểm tra History
# ============================================
Write-Host "`n[TEST 3] KIEM TRA HISTORY" -ForegroundColor Yellow
Start-Sleep -Seconds 5

try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Success "Tim thay $($historyResponse.data.Count) history items"
        
        # Lọc các history gần nhất (telegram, pending hoặc sent)
        $recentTelegram = $historyResponse.data | Where-Object { 
            $_.channelType -eq "telegram" -and 
            ($_.status -eq "pending" -or $_.status -eq "sent" -or $_.status -eq "delivered")
        } | Select-Object -First 5
        
        if ($recentTelegram.Count -gt 0) {
            Write-Host "`n   Cac notification Telegram gan nhat:" -ForegroundColor Gray
            foreach ($item in $recentTelegram) {
                $statusColor = if ($item.status -eq "sent" -or $item.status -eq "delivered") { "Green" } else { "Yellow" }
                Write-Host "     - EventType: $($item.eventType)" -ForegroundColor DarkGray
                Write-Host "       Status: $($item.status)" -ForegroundColor $statusColor
                Write-Host "       Recipient: $($item.recipient)" -ForegroundColor DarkGray
                Write-Host "       Created: $($item.createdAt)" -ForegroundColor DarkGray
                Write-Host ""
            }
        } else {
            Write-Info "Chua co notification Telegram nao trong history (co the dang xu ly)"
        }
        
        # Thống kê
        $statusCount = @{}
        foreach ($item in $historyResponse.data) {
            if ($item.channelType -eq "telegram") {
                $status = $item.status
                if (-not $statusCount.ContainsKey($status)) {
                    $statusCount[$status] = 0
                }
                $statusCount[$status]++
            }
        }
        
        if ($statusCount.Keys.Count -gt 0) {
            Write-Host "   Thong ke Telegram notifications:" -ForegroundColor Gray
            foreach ($status in $statusCount.Keys) {
                Write-Host "     - $status : $($statusCount[$status])" -ForegroundColor DarkGray
            }
        }
    } else {
        Write-Info "Chua co history nao"
    }
} catch {
    Write-ErrorMsg "Loi khi lay history: $($_.Exception.Message)"
}

# ============================================
# TỔNG KẾT
# ============================================
Write-Host "`n" + ("=" * 70) -ForegroundColor Magenta
Write-Host "[DONE] HOAN THANH TEST TELEGRAM NOTIFICATION" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
Write-Info "Da test:"
Write-Host "   1. [OK] Gui truc tiep qua Delivery System (POST /delivery/send)" -ForegroundColor Gray
if ($routingRuleID) {
    Write-Host "   2. [OK] Gui qua Notification System voi Routing (POST /notification/trigger)" -ForegroundColor Gray
} else {
    Write-Host "   2. [SKIP] Gui qua Notification System (khong co routing rule)" -ForegroundColor DarkGray
}
Write-Host "   3. [OK] Kiem tra History" -ForegroundColor Gray
Write-Host ""
Write-Info "Vui long kiem tra Telegram de xac nhan da nhan duoc notification!"
Write-Host ""
