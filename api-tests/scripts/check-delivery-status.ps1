# Script kiểm tra trạng thái delivery system
# Sử dụng: .\scripts\check-delivery-status.ps1 -Token "your_token_here"

param(
    [Parameter(Mandatory=$true)]
    [string]$Token,
    
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

Write-Host "`n[CHECK] KIEM TRA TRANG THAI DELIVERY SYSTEM" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta

# Lấy role ID
Write-Info "Lay role ID..."
$activeRoleID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            $headers["X-Active-Role-ID"] = $activeRoleID
            Write-Success "Da lay duoc role ID: $activeRoleID"
        }
    }
} catch {
    Write-ErrorMsg "Khong the lay role ID: $($_.Exception.Message)"
    exit 1
}

# ============================================
# 1. Kiểm tra Queue Items
# ============================================
Write-Host "`n[1] KIEM TRA QUEUE ITEMS" -ForegroundColor Yellow

# Lưu ý: Queue items không có API public, cần kiểm tra qua history hoặc logs
Write-Info "Queue items khong co API public, se kiem tra qua history"

# ============================================
# 2. Kiểm tra History
# ============================================
Write-Host "`n[2] KIEM TRA HISTORY" -ForegroundColor Yellow

try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Success "Tim thay $($historyResponse.data.Count) history items"
        
        # Lọc các items gần nhất
        $recentItems = $historyResponse.data | Select-Object -First 10
        
        Write-Host "`n   Cac history items gan nhat:" -ForegroundColor Gray
        foreach ($item in $recentItems) {
            $statusColor = switch ($item.status) {
                "sent" { "Green" }
                "delivered" { "Green" }
                "failed" { "Red" }
                "pending" { "Yellow" }
                "processing" { "Cyan" }
                default { "Gray" }
            }
            
            Write-Host "     - ID: $($item.id)" -ForegroundColor DarkGray
            Write-Host "       EventType: $($item.eventType)" -ForegroundColor DarkGray
            Write-Host "       ChannelType: $($item.channelType)" -ForegroundColor DarkGray
            Write-Host "       Status: $($item.status)" -ForegroundColor $statusColor
            Write-Host "       Recipient: $($item.recipient)" -ForegroundColor DarkGray
            if ($item.error) {
                Write-Host "       Error: $($item.error)" -ForegroundColor Red
            }
            if ($item.retryCount) {
                Write-Host "       RetryCount: $($item.retryCount)/$($item.maxRetries)" -ForegroundColor DarkGray
            }
            Write-Host "       CreatedAt: $($item.createdAt)" -ForegroundColor DarkGray
            Write-Host ""
        }
        
        # Thống kê
        $statusCount = @{}
        $channelCount = @{}
        foreach ($item in $historyResponse.data) {
            $status = $item.status
            if (-not $statusCount.ContainsKey($status)) {
                $statusCount[$status] = 0
            }
            $statusCount[$status]++
            
            $channel = $item.channelType
            if (-not $channelCount.ContainsKey($channel)) {
                $channelCount[$channel] = 0
            }
            $channelCount[$channel]++
        }
        
        Write-Host "   Thong ke theo status:" -ForegroundColor Gray
        foreach ($status in $statusCount.Keys | Sort-Object) {
            $count = $statusCount[$status]
            $color = if ($status -eq "sent" -or $status -eq "delivered") { "Green" } 
                     elseif ($status -eq "failed") { "Red" } 
                     else { "Yellow" }
            Write-Host "     - $status : $count" -ForegroundColor $color
        }
        
        Write-Host "`n   Thong ke theo channel:" -ForegroundColor Gray
        foreach ($channel in $channelCount.Keys | Sort-Object) {
            Write-Host "     - $channel : $($channelCount[$channel])" -ForegroundColor DarkGray
        }
        
        # Tìm các items failed
        $failedItems = $historyResponse.data | Where-Object { $_.status -eq "failed" } | Select-Object -First 5
        if ($failedItems.Count -gt 0) {
            Write-Host "`n   [WARN] Co $($failedItems.Count) items bi failed:" -ForegroundColor Yellow
            foreach ($item in $failedItems) {
                Write-Host "     - $($item.eventType) -> $($item.recipient)" -ForegroundColor DarkGray
                if ($item.error) {
                    Write-Host "       Error: $($item.error)" -ForegroundColor Red
                }
            }
        }
        
    } else {
        Write-Warning "Chua co history nao"
    }
} catch {
    Write-ErrorMsg "Loi khi lay history: $($_.Exception.Message)"
}

# ============================================
# 3. Kiểm tra Telegram Sender
# ============================================
Write-Host "`n[3] KIEM TRA TELEGRAM SENDER" -ForegroundColor Yellow

try {
    $senderResponse = Invoke-RestMethod -Uri "$BaseURL/notification/sender/find" -Method GET -Headers $headers
    
    $telegramSenders = $senderResponse.data | Where-Object { $_.channelType -eq "telegram" }
    
    if ($telegramSenders.Count -gt 0) {
        Write-Success "Tim thay $($telegramSenders.Count) Telegram sender(s)"
        
        foreach ($sender in $telegramSenders) {
            Write-Host "   - Name: $($sender.name)" -ForegroundColor Gray
            Write-Host "     ID: $($sender.id)" -ForegroundColor DarkGray
            Write-Host "     IsActive: $($sender.isActive)" -ForegroundColor $(if ($sender.isActive) { "Green" } else { "Red" })
            if ($sender.botToken) {
                $tokenPreview = if ($sender.botToken.Length -gt 10) { 
                    $sender.botToken.Substring(0, 10) + "..." 
                } else { 
                    "***" 
                }
                Write-Host "     BotToken: $tokenPreview" -ForegroundColor DarkGray
            } else {
                Write-Warning "     BotToken: KHONG CO" -ForegroundColor Red
            }
            if ($sender.botUsername) {
                Write-Host "     BotUsername: $($sender.botUsername)" -ForegroundColor DarkGray
            }
            Write-Host ""
        }
    } else {
        Write-ErrorMsg "Khong tim thay Telegram sender nao"
    }
} catch {
    Write-ErrorMsg "Loi khi lay sender: $($_.Exception.Message)"
}

# ============================================
# 4. Kiểm tra Telegram Channel
# ============================================
Write-Host "`n[4] KIEM TRA TELEGRAM CHANNEL" -ForegroundColor Yellow

try {
    $channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    
    $telegramChannels = $channelResponse.data | Where-Object { $_.channelType -eq "telegram" }
    
    if ($telegramChannels.Count -gt 0) {
        Write-Success "Tim thay $($telegramChannels.Count) Telegram channel(s)"
        
        foreach ($channel in $telegramChannels) {
            Write-Host "   - Name: $($channel.name)" -ForegroundColor Gray
            Write-Host "     ID: $($channel.id)" -ForegroundColor DarkGray
            Write-Host "     IsActive: $($channel.isActive)" -ForegroundColor $(if ($channel.isActive) { "Green" } else { "Red" })
            if ($channel.chatIds -and $channel.chatIds.Count -gt 0) {
                Write-Host "     ChatIDs: $($channel.chatIds -join ', ')" -ForegroundColor DarkGray
            } else {
                Write-Warning "     ChatIDs: KHONG CO" -ForegroundColor Red
            }
            Write-Host ""
        }
    } else {
        Write-ErrorMsg "Khong tim thay Telegram channel nao"
    }
} catch {
    Write-ErrorMsg "Loi khi lay channel: $($_.Exception.Message)"
}

# ============================================
# 5. Kiểm tra Logs (nếu có)
# ============================================
Write-Host "`n[5] HUONG DAN KIEM TRA LOGS" -ForegroundColor Yellow
Write-Info "De kiem tra logs chi tiet, vui long xem file log:"
Write-Host "   - Windows: api\logs\app.log" -ForegroundColor Gray
Write-Host "   - Hoac tim cac file log moi nhat trong api\logs\" -ForegroundColor Gray
Write-Host "   - Tim kiem: 'DELIVERY' hoac 'TELEGRAM'" -ForegroundColor Gray

# ============================================
# TỔNG KẾT
# ============================================
Write-Host "`n" + ("=" * 70) -ForegroundColor Magenta
Write-Host "[DONE] HOAN THANH KIEM TRA" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
Write-Info "Neu notification chua duoc gui, kiem tra:"
Write-Host "   1. Telegram sender co BotToken va IsActive = true" -ForegroundColor Gray
Write-Host "   2. Telegram channel co ChatIDs va IsActive = true" -ForegroundColor Gray
Write-Host "   3. History items co status 'failed' (xem error message)" -ForegroundColor Gray
Write-Host "   4. Delivery processor co dang chay (xem logs)" -ForegroundColor Gray
Write-Host ""
