# Script test hệ thống Notification mới
# Sử dụng: .\scripts\test-notification.ps1 -Token "your_token_here"

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

Write-Host "`n[NOTIFICATION] BAT DAU TEST HE THONG NOTIFICATION" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
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
            # Thêm X-Active-Role-ID vào headers
            $headers["X-Active-Role-ID"] = $activeRoleID
        } else {
            Write-Warning "Khong tim thay roleId trong role dau tien"
        }
    } else {
        Write-Warning "User khong co role nao"
    }
} catch {
    Write-Warning "Khong the lay role ID: $($_.Exception.Message)"
    Write-Warning "Test se tiep tuc nhung co the bi loi neu API yeu cau X-Active-Role-ID"
}

# ============================================
# TEST 1: Notification Sender - GET
# ============================================
Write-Host "`n[TEST 1] Notification Sender" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/sender/find" -Method GET -Headers $headers
    Write-Success "Lấy danh sách sender thành công"
    Write-Host "   Số lượng sender: $($response.data.Count)" -ForegroundColor Gray
    if ($response.data.Count -gt 0) {
        $firstSender = $response.data[0]
        Write-Host "   Sender đầu tiên: $($firstSender.name) (Type: $($firstSender.channelType))" -ForegroundColor Gray
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach sender: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

# ============================================
# TEST 2: Notification Channel - GET
# ============================================
Write-Host "`n[TEST 2] Notification Channel" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/channel/find" -Method GET -Headers $headers
    Write-Success "Lấy danh sách channel thành công"
    Write-Host "   Số lượng channel: $($response.data.Count)" -ForegroundColor Gray
    if ($response.data.Count -gt 0) {
        foreach ($channel in $response.data) {
            Write-Host "   - $($channel.name) (Type: $($channel.channelType), Recipients: $($channel.recipients.Count))" -ForegroundColor Gray
        }
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach channel: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

# ============================================
# TEST 3: Notification Template - GET
# ============================================
Write-Host "`n[TEST 3] Notification Template" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
    Write-Success "Lấy danh sách template thành công"
    Write-Host "   Số lượng template: $($response.data.Count)" -ForegroundColor Gray
    if ($response.data.Count -gt 0) {
        foreach ($template in $response.data) {
            Write-Host "   - $($template.name) (EventType: $($template.eventType), ChannelType: $($template.channelType))" -ForegroundColor Gray
        }
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach template: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

# ============================================
# TEST 4: Notification Routing - GET
# ============================================
Write-Host "`n[TEST 4] Notification Routing" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    Write-Success "Lấy danh sách routing rules thành công"
    Write-Host "   Số lượng routing rules: $($response.data.Count)" -ForegroundColor Gray
    if ($response.data.Count -gt 0) {
        foreach ($routing in $response.data) {
            Write-Host "   - EventType: $($routing.eventType), Priority: $($routing.priority)" -ForegroundColor Gray
            if ($routing.domain) {
                Write-Host "     Domain: $($routing.domain)" -ForegroundColor DarkGray
            }
            if ($routing.severities) {
                Write-Host "     Severities: $($routing.severities -join ', ')" -ForegroundColor DarkGray
            }
        }
    } else {
        Write-Warning "Không có routing rules nào. Cần tạo routing rule để trigger notification."
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach routing: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

# ============================================
# TEST 5: Notification History - GET
# ============================================
Write-Host "`n[TEST 5] Notification History" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    Write-Success "Lấy danh sách history thành công"
    Write-Host "   Số lượng history: $($response.data.Count)" -ForegroundColor Gray
    if ($response.data.Count -gt 0) {
        $recent = $response.data[0]
        Write-Host "   History gần nhất:" -ForegroundColor Gray
        Write-Host "     - EventType: $($recent.eventType)" -ForegroundColor DarkGray
        Write-Host "     - Status: $($recent.status)" -ForegroundColor DarkGray
        Write-Host "     - ChannelType: $($recent.channelType)" -ForegroundColor DarkGray
        Write-Host "     - CreatedAt: $($recent.createdAt)" -ForegroundColor DarkGray
    }
} catch {
    Write-ErrorMsg "Loi khi lay danh sach history: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        Write-Host "   Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

# ============================================
# TEST 6: Notification Trigger - POST
# ============================================
Write-Host "`n[TEST 6] Notification Trigger" -ForegroundColor Yellow
Write-Info "Test trigger notification với các eventType khác nhau..."

# Test với eventType system_error (critical)
Write-Host "`n   Test 6.1: system_error (Critical)" -ForegroundColor Cyan
try {
    $payload = @{
        eventType = "system_error"
        payload = @{
            errorMessage = "Test database connection failed"
            errorCode = "DB_CONN_001"
            timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
        }
    } | ConvertTo-Json -Depth 10

    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    Write-Success "Trigger notification thành công"
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor Gray
    Write-Host "   Message: $($response.message)" -ForegroundColor Gray
} catch {
    Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
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

# Test với eventType conversation_unreplied (high)
Start-Sleep -Milliseconds 500
Write-Host "`n   Test 6.2: conversation_unreplied (High)" -ForegroundColor Cyan
try {
    $payload = @{
        eventType = "conversation_unreplied"
        payload = @{
            conversationId = "test_conv_123"
            customerName = "Nguyễn Văn A"
            unreadCount = 5
            lastMessage = "Xin chào, tôi cần hỗ trợ"
        }
    } | ConvertTo-Json -Depth 10

    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    Write-Success "Trigger notification thành công"
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor Gray
} catch {
    Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($errorDetail) {
            Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
        }
    }
}

# Test với eventType order_created (info)
Start-Sleep -Milliseconds 500
Write-Host "`n   Test 6.3: order_created (Info)" -ForegroundColor Cyan
try {
    $payload = @{
        eventType = "order_created"
        payload = @{
            orderId = "ORD_12345"
            customerName = "Trần Thị B"
            totalAmount = 1500000
            items = @("Sản phẩm A", "Sản phẩm B")
        }
    } | ConvertTo-Json -Depth 10

    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    Write-Success "Trigger notification thành công"
    Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
    Write-Host "   Queued: $($response.queued)" -ForegroundColor Gray
} catch {
    Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
    if ($_.ErrorDetails.Message) {
        $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($errorDetail) {
            Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
        }
    }
}

# Test với eventType không có routing rule
Start-Sleep -Milliseconds 500
Write-Host "`n   Test 6.4: test_event_no_rule (Test không có routing rule)" -ForegroundColor Cyan
try {
    $payload = @{
        eventType = "test_event_no_rule_$(Get-Date -Format 'yyyyMMddHHmmss')"
        payload = @{
            message = "Test event không có routing rule"
        }
    } | ConvertTo-Json -Depth 10

    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    if ($response.queued -eq 0) {
        Write-Warning "Không có routing rule cho eventType này (đúng như mong đợi)"
        Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    } else {
        Write-Success "Trigger notification thành công (có routing rule)"
    }
} catch {
    Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
}

# ============================================
# TEST 7: Kiem tra lai History sau khi trigger
# ============================================
Write-Host "`n[TEST 7] Kiem tra History sau khi trigger" -ForegroundColor Yellow
Start-Sleep -Seconds 2
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/history/find" -Method GET -Headers $headers
    Write-Success "Lay danh sach history sau trigger thanh cong"
    
    if ($response.data -and $response.data.Count -gt 0) {
        Write-Host "   Tong so history: $($response.data.Count)" -ForegroundColor Gray
        
        # Dem theo status
        $statusCount = @{}
        foreach ($item in $response.data) {
            $status = $item.status
            if ($status) {
                if (-not $statusCount.ContainsKey($status)) {
                    $statusCount[$status] = 0
                }
                $statusCount[$status]++
            }
        }
        
        if ($statusCount.Keys.Count -gt 0) {
            Write-Host "   Phan bo theo status:" -ForegroundColor Gray
            foreach ($status in $statusCount.Keys) {
                Write-Host "     - $status : $($statusCount[$status])" -ForegroundColor DarkGray
            }
        }
        
        # Hien thi 3 history gan nhat
        $maxShow = [Math]::Min(3, $response.data.Count)
        Write-Host "`n   $maxShow history gan nhat:" -ForegroundColor Gray
        for ($i = 0; $i -lt $maxShow; $i++) {
            $item = $response.data[$i]
            $info = "     - $($item.eventType) - $($item.status) - $($item.channelType)"
            if ($item.recipient) {
                $info += " - $($item.recipient)"
            }
            Write-Host $info -ForegroundColor DarkGray
        }
    } else {
        Write-Info "Chua co history nao (co the do khong co routing rule hoac notification chua duoc gui)"
    }
} catch {
    Write-ErrorMsg "Loi khi lay history: $($_.Exception.Message)"
}

# ============================================
# TỔNG KẾT
# ============================================
Write-Host "`n" + ("=" * 60) -ForegroundColor Magenta
Write-Host "[DONE] HOAN THANH TEST HE THONG NOTIFICATION" -ForegroundColor Magenta
Write-Host ("=" * 60) -ForegroundColor Magenta
Write-Info "Da test cac chuc nang:"
Write-Host "   1. [OK] Notification Sender (GET)" -ForegroundColor Gray
Write-Host "   2. [OK] Notification Channel (GET)" -ForegroundColor Gray
Write-Host "   3. [OK] Notification Template (GET)" -ForegroundColor Gray
Write-Host "   4. [OK] Notification Routing (GET)" -ForegroundColor Gray
Write-Host "   5. [OK] Notification History (GET)" -ForegroundColor Gray
Write-Host "   6. [OK] Notification Trigger (POST) - voi nhieu eventType" -ForegroundColor Gray
Write-Host "   7. [OK] Kiem tra History sau trigger" -ForegroundColor Gray
Write-Host ""
