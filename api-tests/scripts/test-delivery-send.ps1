# Script test gửi thông báo trực tiếp qua endpoint /delivery/send
# Endpoint này gửi trực tiếp không cần routing rules (Hệ thống 1)
# Sử dụng: .\scripts\test-delivery-send.ps1

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

Write-Host "`n[TEST] Gửi thông báo trực tiếp qua /delivery/send" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Host "Base URL: $BaseURL" -ForegroundColor Cyan
Write-Host "Token: $($token.Substring(0, 30))..." -ForegroundColor Cyan

# Bước 1: Lấy role ID từ API /auth/roles
Write-Host "`n[Bước 1] Lấy role ID từ API /auth/roles..." -ForegroundColor Yellow
$activeRoleID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            Write-Host "[OK] Đã lấy được role ID: $activeRoleID" -ForegroundColor Green
            # Thêm X-Active-Role-ID vào headers
            $headers["X-Active-Role-ID"] = $activeRoleID
        } else {
            Write-Host "[WARN] Không tìm thấy roleId trong role đầu tiên" -ForegroundColor Yellow
        }
    } else {
        Write-Host "[WARN] User không có role nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[ERROR] Không thể lấy role ID: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "[ERROR] Không thể tiếp tục test vì cần role ID" -ForegroundColor Red
    exit 1
}

# Bước 2: Kiểm tra senders có sẵn
Write-Host "`n[Bước 2] Kiểm tra senders có sẵn..." -ForegroundColor Yellow
try {
    $senderResponse = Invoke-RestMethod -Uri "$BaseURL/notification/sender/find" -Method GET -Headers $headers
    if ($senderResponse.data -and $senderResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($senderResponse.data.Count) sender(s)" -ForegroundColor Green
        foreach ($sender in $senderResponse.data) {
            $status = if ($sender.isActive) { "Active" } else { "Inactive" }
            Write-Host "   - $($sender.name) (Type: $($sender.channelType), Status: $status)" -ForegroundColor DarkGray
        }
    } else {
        Write-Host "[WARN] Không có sender nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[WARN] Không thể lấy senders: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 3: Test gửi thông báo qua email (nếu có sender active)
Write-Host "`n[Bước 3] Test gửi thông báo qua email..." -ForegroundColor Yellow
$hasActiveEmailSender = $false
if ($senderResponse.data) {
    foreach ($sender in $senderResponse.data) {
        if ($sender.channelType -eq "email" -and $sender.isActive) {
            $hasActiveEmailSender = $true
            break
        }
    }
}

if ($hasActiveEmailSender) {
    try {
        $emailContent = "Day la noi dung thong bao test duoc gui qua he thong notification.`n`nThoi gian: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')`n`nNoi dung test nay duoc gui truc tiep qua endpoint /delivery/send."
        $payload = @{
            channelType = "email"
            recipient = "test@example.com"
            subject = "Test Notification tu Script"
            content = $emailContent
            eventType = "test.delivery.direct"
            metadata = @{
                source = "test-script"
                timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
            }
            ctas = @(
                @{
                    label = "Xem chi tiet"
                    action = "https://example.com/details"
                    originalUrl = "https://example.com/details"
                    style = "primary"
                }
            )
        } | ConvertTo-Json -Depth 10 -Compress

        $response = Invoke-RestMethod -Uri "$BaseURL/delivery/send" -Method POST -Headers $headers -Body $payload
        
        Write-Host "`n[OK] Gửi thông báo email thành công!" -ForegroundColor Green
        Write-Host "   MessageID: $($response.messageId)" -ForegroundColor Gray
        Write-Host "   Status: $($response.status)" -ForegroundColor Gray
        Write-Host "   QueuedAt: $($response.queuedAt)" -ForegroundColor Gray
        Write-Host "`n[SUCCESS] Notification đã được thêm vào queue!" -ForegroundColor Green
    } catch {
        Write-Host "`n[ERROR] Lỗi khi gửi thông báo email: $($_.Exception.Message)" -ForegroundColor Red
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
} else {
    Write-Host "[SKIP] Bỏ qua test email vì không có email sender active" -ForegroundColor Yellow
}

# Bước 4: Test gửi thông báo qua telegram (nếu có chat ID)
Write-Host "`n[Bước 4] Test gửi thông báo qua telegram..." -ForegroundColor Yellow
Write-Host "   (Cần có telegram sender và chat ID hợp lệ)" -ForegroundColor Cyan
$hasActiveTelegramSender = $false
if ($senderResponse.data) {
    foreach ($sender in $senderResponse.data) {
        if ($sender.channelType -eq "telegram" -and $sender.isActive) {
            $hasActiveTelegramSender = $true
            break
        }
    }
}

if ($hasActiveTelegramSender) {
    try {
        $telegramContent = "Test Notification tu Script PowerShell`n`nDay la thong bao test duoc gui truc tiep qua endpoint /delivery/send.`n`nThoi gian: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')"
        $payloadObj = @{
            channelType = "telegram"
            recipient = "-5139196836"  # Chat ID từ history trước đó
            content = $telegramContent
            eventType = "test.delivery.direct"
            metadata = @{
                source = "test-script"
                timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
            }
            ctas = @(
                @{
                    label = "Xem chi tiet"
                    action = "https://example.com/details"
                    originalUrl = "https://example.com/details"
                    style = "primary"
                }
            )
        }
        
        # Convert to JSON với encoding UTF8
        $payload = $payloadObj | ConvertTo-Json -Depth 10 -Compress
        $payloadBytes = [System.Text.Encoding]::UTF8.GetBytes($payload)

        $response = Invoke-RestMethod -Uri "$BaseURL/delivery/send" -Method POST -Headers $headers -Body $payloadBytes
    
        Write-Host "`n[OK] Gửi thông báo telegram thành công!" -ForegroundColor Green
        Write-Host "   MessageID: $($response.messageId)" -ForegroundColor Gray
        Write-Host "   Status: $($response.status)" -ForegroundColor Gray
        Write-Host "   QueuedAt: $($response.queuedAt)" -ForegroundColor Gray
        Write-Host "`n[SUCCESS] Notification đã được thêm vào queue!" -ForegroundColor Green
    } catch {
        Write-Host "`n[WARN] Không thể gửi thông báo telegram: $($_.Exception.Message)" -ForegroundColor Yellow
        if ($_.ErrorDetails.Message) {
            $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
            if ($errorDetail) {
                Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Yellow
            } else {
                Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Yellow
            }
        }
    }
} else {
    Write-Host "[SKIP] Bỏ qua test telegram vì không có telegram sender active" -ForegroundColor Yellow
}

# Bước 5: Kiểm tra history sau khi gửi
Write-Host "`n[Bước 5] Kiểm tra notification history..." -ForegroundColor Yellow
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
            if ($item.subject) {
                Write-Host "         Subject: $($item.subject)" -ForegroundColor DarkGray
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
Write-Host "[DONE] Hoàn thành test gửi thông báo trực tiếp" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Host ""
