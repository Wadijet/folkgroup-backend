# Script kiểm tra tất cả notification history
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

Write-Host "`n[CHECK] Tất cả Notification History" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Host "`n[OK] Tổng số: $($historyResponse.data.Count) notification(s)" -ForegroundColor Green
        
        # Hiển thị 5 notification gần nhất
        $recentNotifications = $historyResponse.data | Select-Object -First 5
        
        Write-Host "`n5 notification gần nhất:" -ForegroundColor Cyan
        for ($i = 0; $i -lt $recentNotifications.Count; $i++) {
            $item = $recentNotifications[$i]
            $statusColor = switch ($item.status) {
                "sent" { "Green" }
                "failed" { "Red" }
                "pending" { "Yellow" }
                default { "Gray" }
            }
            
            Write-Host "`n[$($i+1)] EventType: $($item.eventType)" -ForegroundColor DarkGray
            Write-Host "    Status: $($item.status)" -ForegroundColor $statusColor
            Write-Host "    Channel: $($item.channelType)" -ForegroundColor DarkGray
            if ($item.recipient) {
                Write-Host "    Recipient: $($item.recipient)" -ForegroundColor DarkGray
            }
            if ($item.createdAt) {
                try {
                    $createdTime = [DateTimeOffset]::FromUnixTimeSeconds($item.createdAt).LocalDateTime.ToString("yyyy-MM-dd HH:mm:ss")
                    Write-Host "    CreatedAt: $createdTime" -ForegroundColor DarkGray
                } catch {
                    Write-Host "    CreatedAt: $($item.createdAt)" -ForegroundColor DarkGray
                }
            }
            if ($item.sentAt) {
                try {
                    $sentTime = [DateTimeOffset]::FromUnixTimeSeconds($item.sentAt).LocalDateTime.ToString("yyyy-MM-dd HH:mm:ss")
                    Write-Host "    SentAt: $sentTime" -ForegroundColor DarkGray
                } catch {
                    Write-Host "    SentAt: $($item.sentAt)" -ForegroundColor DarkGray
                }
            }
        }
    } else {
        Write-Host "`n[INFO] Chưa có notification nào trong history" -ForegroundColor Cyan
    }
} catch {
    Write-Host "`n[ERROR] Không thể lấy history: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
