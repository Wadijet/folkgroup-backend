# Script kiểm tra API dashboard với bearer token admin
# Chạy: .\scripts\verify_api_dashboard.ps1

$baseUrl = "http://localhost:8080/api/v1"
$bearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OThjMzQ4OWNiZjYyZGJhMGYwZmQzZmMiLCJ0aW1lIjoiNjk5ZWNiNzQiLCJyYW5kb21OdW1iZXIiOiIzMiJ9.zIbeU31HBZg2P9lWgY4QB_PpqsmrvY7pUpGVcR5XNWE"

$headers = @{
    "Authorization" = "Bearer $bearerToken"
    "Content-Type" = "application/json"
}

Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Kiểm tra API Dashboard" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

# 1. Lấy roles để có X-Active-Role-ID
$activeRoleId = $null
try {
    $roleResp = Invoke-RestMethod -Uri "$baseUrl/auth/roles" -Method GET -Headers $headers
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $adminRole = $roleResp.data | Where-Object { $_.roleName -eq "Administrator" } | Select-Object -First 1
        $role = if ($adminRole) { $adminRole } else { $roleResp.data[0] }
        $activeRoleId = $role.roleId
        Write-Host "`nOK Dùng role: $($role.roleName) (roleId: $activeRoleId)" -ForegroundColor Green
    }
} catch {
    Write-Host "`nLỖI Không thể kết nối API (server có đang chạy?): $_" -ForegroundColor Red
    exit 1
}

if (-not $activeRoleId) {
    Write-Host "LỖI: User không có role" -ForegroundColor Red
    exit 1
}

$headers["X-Active-Role-ID"] = $activeRoleId

# 2. Lấy organization để có X-Active-Organization-ID
$activeOrgId = $null
try {
    $orgResp = Invoke-RestMethod -Uri "$baseUrl/organization" -Method GET -Headers $headers
    if ($orgResp.data -and $orgResp.data.Count -gt 0) {
        $org = $orgResp.data[0]
        $activeOrgId = $org.id
        Write-Host "OK Organization: $($org.name) (id: $activeOrgId)" -ForegroundColor Green
    }
} catch {
    Write-Host "WARN Không lấy được organization: $_" -ForegroundColor Yellow
}

if ($activeOrgId) {
    $headers["X-Active-Organization-ID"] = $activeOrgId
}

# 3. Gọi GET /dashboard/customers/period-movements-from-snapshots (thay thế /customers + /customers/trend)
Write-Host "`n--- GET /dashboard/customers/period-movements-from-snapshots ---" -ForegroundColor Cyan
try {
    $customersResp = Invoke-RestMethod -Uri "$baseUrl/dashboard/customers/period-movements-from-snapshots?period=month&limit=20&offset=0" -Method GET -Headers $headers
    if ($customersResp.status -eq "success") {
        $snap = $customersResp.data.currentSnapshot
        if ($snap) {
            Write-Host "OK KPI: totalCustomers=$($snap.summary.totalCustomers), newInPeriod=$($snap.summary.newCustomersInPeriod)" -ForegroundColor Green
            Write-Host "  snapshotSource: $($snap.snapshotSource), periodKey: $($snap.snapshotPeriodKey)" -ForegroundColor Gray
            if ($snap.customers -and $snap.customers.Count -gt 0) {
                Write-Host "  Số khách trong bảng: $($snap.customers.Count)" -ForegroundColor Green
            }
        }
        $trendData = $customersResp.data.trendData
        if ($trendData -and $trendData.Count -gt 0) {
            Write-Host "  trendData items: $($trendData.Count)" -ForegroundColor Gray
        }
    } else {
        Write-Host "WARN Response: $($customersResp | ConvertTo-Json -Depth 2)" -ForegroundColor Yellow
    }
} catch {
    Write-Host "LỖI: $_" -ForegroundColor Red
}

Write-Host "`n========================================" -ForegroundColor Magenta
Write-Host "Hoàn thành kiểm tra API" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
