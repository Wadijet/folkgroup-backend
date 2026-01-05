# Script bật tất cả routing rules
param(
    [Parameter(Mandatory=$true)]
    [string]$Token,
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$headers = @{
    "Authorization" = "Bearer $Token"
    "Content-Type" = "application/json"
}

Write-Host "`n[ACTIVATE] BAT TAT CA ROUTING RULES" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta

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

# Lấy tất cả routing rules
Write-Host "`n[INFO] Lay tat ca routing rules..." -ForegroundColor Cyan
try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Host "[OK] Tim thay $($routingResponse.data.Count) routing rules" -ForegroundColor Green
        
        # Tìm các rules chưa active
        $inactiveRules = $routingResponse.data | Where-Object { -not $_.isActive }
        
        if ($inactiveRules.Count -gt 0) {
            Write-Host "[INFO] Tim thay $($inactiveRules.Count) routing rules chua active" -ForegroundColor Yellow
            
            foreach ($rule in $inactiveRules) {
                Write-Host "   - EventType: $($rule.eventType), ID: $($rule.id)" -ForegroundColor Gray
                
                try {
                    $updatePayload = @{ isActive = $true } | ConvertTo-Json
                    $updateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/update-by-id/$($rule.id)" -Method PUT -Headers $headers -Body $updatePayload
                    Write-Host "     [OK] Da bat" -ForegroundColor Green
                } catch {
                    Write-Host "     [ERROR] Loi: $($_.Exception.Message)" -ForegroundColor Red
                }
            }
        } else {
            Write-Host "[INFO] Tat ca routing rules da duoc bat" -ForegroundColor Green
        }
        
        # Hiển thị thống kê
        Write-Host "`n[INFO] Thong ke:" -ForegroundColor Cyan
        $activeCount = ($routingResponse.data | Where-Object { $_.isActive }).Count
        $inactiveCount = ($routingResponse.data | Where-Object { -not $_.isActive }).Count
        Write-Host "   - Active: $activeCount" -ForegroundColor Green
        Write-Host "   - Inactive: $inactiveCount" -ForegroundColor $(if ($inactiveCount -gt 0) { "Yellow" } else { "Green" })
        
    } else {
        Write-Host "[WARN] Khong co routing rule nao" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[ERROR] Loi: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
