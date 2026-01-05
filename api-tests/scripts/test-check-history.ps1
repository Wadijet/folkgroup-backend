# Script kiểm tra notification history
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

Write-Host "`n[CHECK] Kiểm tra Notification History" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Đợi một chút để delivery worker xử lý
Write-Host "`nĐợi 3 giây để delivery worker xử lý..." -ForegroundColor Yellow
Start-Sleep -Seconds 3

# Kiểm tra history
try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Host "`n[OK] Có $($historyResponse.data.Count) notification(s) trong history" -ForegroundColor Green
        
        # Tìm notification gần nhất với system_error
        $recentNotifications = $historyResponse.data | Where-Object { $_.eventType -eq "system_error" } | Select-Object -First 3
        
        if ($recentNotifications.Count -gt 0) {
            Write-Host "`nNotification gần nhất với eventType 'system_error':" -ForegroundColor Cyan
            foreach ($item in $recentNotifications) {
                Write-Host "`n  - EventType: $($item.eventType)" -ForegroundColor DarkGray
                Write-Host "    Status: $($item.status)" -ForegroundColor $(if ($item.status -eq "sent") { "Green" } else { "Yellow" })
                Write-Host "    Channel: $($item.channelType)" -ForegroundColor DarkGray
                if ($item.recipient) {
                    Write-Host "    Recipient: $($item.recipient)" -ForegroundColor DarkGray
                }
                if ($item.createdAt) {
                    Write-Host "    CreatedAt: $($item.createdAt)" -ForegroundColor DarkGray
                }
                if ($item.sentAt) {
                    Write-Host "    SentAt: $($item.sentAt)" -ForegroundColor DarkGray
                }
            }
        } else {
            Write-Host "`n[INFO] Chưa có notification nào với eventType 'system_error' trong history" -ForegroundColor Cyan
            Write-Host "   (Có thể đang được xử lý hoặc chưa được gửi)" -ForegroundColor Yellow
        }
    } else {
        Write-Host "`n[INFO] Chưa có notification nào trong history" -ForegroundColor Cyan
    }
} catch {
    Write-Host "`n[ERROR] Không thể lấy history: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
