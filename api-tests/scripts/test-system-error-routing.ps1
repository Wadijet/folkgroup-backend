# Test với eventType system_error đã có routing rule sẵn
param(
    [Parameter(Mandatory=$true)]
    [string]$Token,
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$headers = @{
    "Authorization" = "Bearer $Token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Test voi system_error (co routing rule san)" -ForegroundColor Magenta

# Lấy role ID
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $activeRoleID = $roleResponse.data[0].roleId
        $headers["X-Active-Role-ID"] = $activeRoleID
        Write-Host "[OK] Role ID: $activeRoleID" -ForegroundColor Green
    }
} catch {
    Write-Host "[ERROR] Khong the lay role ID" -ForegroundColor Red
    exit 1
}

# Kiểm tra routing rule cho system_error
Write-Host "`n[INFO] Kiem tra routing rule cho system_error..." -ForegroundColor Cyan
try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    $systemErrorRules = $routingResponse.data | Where-Object { $_.eventType -eq "system_error" }
    
    if ($systemErrorRules.Count -gt 0) {
        Write-Host "[OK] Tim thay $($systemErrorRules.Count) routing rule(s) cho system_error" -ForegroundColor Green
        foreach ($rule in $systemErrorRules) {
            Write-Host "   - ID: $($rule.id), IsActive: $($rule.isActive), ChannelTypes: $($rule.channelTypes -join ', ')" -ForegroundColor Gray
            
            # Bật nếu chưa bật
            if (-not $rule.isActive) {
                Write-Host "   [INFO] Dang bat routing rule..." -ForegroundColor Cyan
                $updatePayload = @{ isActive = $true } | ConvertTo-Json
                Invoke-RestMethod -Uri "$BaseURL/notification/routing/update-by-id/$($rule.id)" -Method PUT -Headers $headers -Body $updatePayload | Out-Null
                Write-Host "   [OK] Da bat routing rule" -ForegroundColor Green
            }
        }
    } else {
        Write-Host "[WARN] Khong tim thay routing rule cho system_error" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[ERROR] Loi khi kiem tra routing: $($_.Exception.Message)" -ForegroundColor Red
}

# Test trigger
Write-Host "`n[INFO] Trigger notification voi system_error..." -ForegroundColor Cyan
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification qua routing voi system_error"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    if ($response.queued -gt 0) {
        Write-Host "[OK] Da queue $($response.queued) notification(s)!" -ForegroundColor Green
        Write-Host "   EventType: $($response.eventType)" -ForegroundColor Gray
        Write-Host "   Message: $($response.message)" -ForegroundColor Gray
        Write-Host "`n[INFO] Vui long kiem tra Telegram trong vong 10-30 giay..." -ForegroundColor Cyan
    } else {
        Write-Host "[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "   Message: $($response.message)" -ForegroundColor Gray
    }
} catch {
    Write-Host "[ERROR] Loi: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
