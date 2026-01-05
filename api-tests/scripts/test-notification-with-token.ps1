# Script test gửi thông báo với token được cung cấp
# Sử dụng: .\scripts\test-notification-with-token.ps1

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

Write-Host "`n[TEST] Gửi thông báo qua hệ thống notification" -ForegroundColor Magenta
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
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
    Write-Host "[WARN] Test sẽ tiếp tục nhưng có thể bị lỗi nếu API yêu cầu X-Active-Role-ID" -ForegroundColor Yellow
}

# Bước 2: Kiểm tra routing rules có sẵn
Write-Host "`n[Bước 2] Kiểm tra routing rules có sẵn..." -ForegroundColor Yellow
$availableEventTypes = @()
try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($routingResponse.data.Count) routing rule(s)" -ForegroundColor Green
        Write-Host "   Các eventType có sẵn:" -ForegroundColor Gray
        foreach ($routing in $routingResponse.data) {
            $eventType = $routing.eventType
            $availableEventTypes += $eventType
            $orgId = if ($routing.ownerOrganizationId) { $routing.ownerOrganizationId } else { "null (system)" }
            Write-Host "     - $eventType (OrgID: $orgId, Priority: $($routing.priority))" -ForegroundColor DarkGray
        }
    } else {
        Write-Host "[WARN] Không có routing rules nào. Cần tạo routing rule để trigger notification." -ForegroundColor Yellow
    }
} catch {
    Write-Host "[WARN] Không thể lấy routing rules: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Kiểm tra channels
Write-Host "`n   Kiểm tra channels có sẵn..." -ForegroundColor Cyan
try {
    $channelResponse = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    if ($channelResponse.data -and $channelResponse.data.Count -gt 0) {
        Write-Host "   [OK] Có $($channelResponse.data.Count) channel(s)" -ForegroundColor Green
        foreach ($channel in $channelResponse.data) {
            Write-Host "     - $($channel.name) (Type: $($channel.channelType), Recipients: $($channel.recipients.Count))" -ForegroundColor DarkGray
        }
    } else {
        Write-Host "   [WARN] Không có channel nào" -ForegroundColor Yellow
    }
} catch {
    Write-Host "   [WARN] Không thể lấy channels: $($_.Exception.Message)" -ForegroundColor Yellow
}

# Bước 3: Test gửi thông báo với các eventType khác nhau
Write-Host "`n[Bước 3] Gửi thông báo với các eventType khác nhau" -ForegroundColor Yellow

# Danh sách eventType để test (ưu tiên các eventType có trong routing rules)
$testEventTypes = @("system_error", "system_warning", "database_error", "api_error", "security_alert")
if ($availableEventTypes.Count -gt 0) {
    # Thêm các eventType từ routing rules vào đầu danh sách
    $testEventTypes = $availableEventTypes[0..([Math]::Min(3, $availableEventTypes.Count-1))] + $testEventTypes | Select-Object -Unique
}

$successCount = 0
$totalQueued = 0

foreach ($eventType in $testEventTypes) {
    Write-Host "`n   Test với eventType: $eventType" -ForegroundColor Cyan
    try {
        # Tạo payload phù hợp với từng eventType
        $payloadData = @{
            timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
            testMessage = "Test notification từ script PowerShell"
        }
        
        switch ($eventType) {
            { $_ -like "*error*" } {
                $payloadData["errorMessage"] = "Test error notification"
                $payloadData["errorCode"] = "TEST_001"
            }
            { $_ -like "*warning*" } {
                $payloadData["warningMessage"] = "Test warning notification"
            }
            { $_ -like "*alert*" } {
                $payloadData["alertMessage"] = "Test security alert"
                $payloadData["severity"] = "high"
            }
            default {
                $payloadData["message"] = "Test notification cho $eventType"
            }
        }
        
        $payload = @{
            eventType = $eventType
            payload = $payloadData
        } | ConvertTo-Json -Depth 10

        $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
        
        if ($response.queued -gt 0) {
            Write-Host "   [SUCCESS] Đã queue $($response.queued) notification!" -ForegroundColor Green
            $successCount++
            $totalQueued += $response.queued
        } else {
            Write-Host "   [WARN] Không có notification nào được queue" -ForegroundColor Yellow
            Write-Host "      Message: $($response.message)" -ForegroundColor DarkGray
        }
    } catch {
        Write-Host "   [ERROR] Lỗi: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails.Message) {
            $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
            if ($errorDetail) {
                Write-Host "      Message: $($errorDetail.message)" -ForegroundColor Red
            }
        }
    }
    
    # Nghỉ một chút giữa các request
    Start-Sleep -Milliseconds 300
}

Write-Host "`n   Tổng kết:" -ForegroundColor Yellow
Write-Host "      - Số eventType đã test: $($testEventTypes.Count)" -ForegroundColor Gray
Write-Host "      - Số eventType thành công: $successCount" -ForegroundColor Gray
Write-Host "      - Tổng số notification đã queue: $totalQueued" -ForegroundColor Gray

# Bước 4: Kiểm tra history sau khi gửi
Write-Host "`n[Bước 4] Kiểm tra notification history..." -ForegroundColor Yellow
Start-Sleep -Seconds 2
try {
    $historyResponse = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    if ($historyResponse.data -and $historyResponse.data.Count -gt 0) {
        Write-Host "[OK] Có $($historyResponse.data.Count) notification(s) trong history" -ForegroundColor Green
        
        # Hiển thị 3 notification gần nhất
        $maxShow = [Math]::Min(3, $historyResponse.data.Count)
        Write-Host "`n   $maxShow notification gần nhất:" -ForegroundColor Gray
        for ($i = 0; $i -lt $maxShow; $i++) {
            $item = $historyResponse.data[$i]
            Write-Host "     [$($i+1)] EventType: $($item.eventType) | Status: $($item.status) | Channel: $($item.channelType)" -ForegroundColor DarkGray
            if ($item.recipient) {
                Write-Host "         Recipient: $($item.recipient)" -ForegroundColor DarkGray
            }
            if ($item.createdAt) {
                Write-Host "         CreatedAt: $($item.createdAt)" -ForegroundColor DarkGray
            }
        }
    } else {
        Write-Host "[INFO] Chưa có notification nào trong history" -ForegroundColor Cyan
    }
} catch {
    Write-Host "[WARN] Không thể lấy history: $($_.Exception.Message)" -ForegroundColor Yellow
}

Write-Host "`n" + ("=" * 60) -ForegroundColor Magenta
Write-Host "[DONE] Hoàn thành test gửi thông báo" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Host ""
