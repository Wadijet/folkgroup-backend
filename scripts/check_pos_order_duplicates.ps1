# Script kiểm tra duplicate pc_pos_orders và test CIO ingest (POST /cio/ingest)
# Dùng bearer token để gọi API

$baseUrl = "http://localhost:8080/api/v1"
$bearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OThjMzQ4OWNiZjYyZGJhMGYwZmQzZmMiLCJ0aW1lIjoiNjlhNjRkOWYiLCJyYW5kb21OdW1iZXIiOiIzIn0.5_BiarO_M5e-lQBfQ2fhlyW55XPTmKFexb7AT-mb0iQ"

$headers = @{
    "Authorization" = "Bearer $bearerToken"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResp = $null
try {
    $roleResp = Invoke-RestMethod -Uri "$baseUrl/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
} catch {
    Write-Host "Lỗi lấy roles: $_" -ForegroundColor Red
    exit 1
}

$activeRoleId = $null
if ($roleResp.data -and $roleResp.data.Count -gt 0) {
    $adminRole = $roleResp.data | Where-Object { $_.roleName -eq "Administrator" } | Select-Object -First 1
    $role = if ($adminRole) { $adminRole } else { $roleResp.data[0] }
    $activeRoleId = $role.roleId
    Write-Host "Dùng role: $($role.roleName) (roleId: $activeRoleId)" -ForegroundColor Green
}

if (-not $activeRoleId) {
    Write-Host "Lỗi: User không có role" -ForegroundColor Red
    exit 1
}

$headers["X-Active-Role-ID"] = $activeRoleId

# Lấy danh sách orders
$filterEncoded = [System.Uri]::EscapeDataString("{}")
$optionsEncoded = [System.Uri]::EscapeDataString('{"limit":500}')
$url = "$baseUrl/pancake-pos/order/find?filter=$filterEncoded&options=$optionsEncoded"

Write-Host "`nĐang lấy danh sách orders..." -ForegroundColor Cyan
try {
    $ordersResp = Invoke-RestMethod -Uri $url -Method GET -Headers $headers -ErrorAction Stop
} catch {
    Write-Host "Lỗi lấy orders: $_" -ForegroundColor Red
    exit 1
}

if ($ordersResp.status -ne "success" -or -not $ordersResp.data) {
    Write-Host "Response không hợp lệ: $($ordersResp | ConvertTo-Json -Depth 2)" -ForegroundColor Red
    exit 1
}

$orders = $ordersResp.data
Write-Host "Tổng số orders: $($orders.Count)" -ForegroundColor Green

# Tìm duplicate theo (orderId, ownerOrganizationId)
$grouped = @{}
foreach ($o in $orders) {
    $oid = $o.orderId
    $orgId = if ($o.ownerOrganizationId) { $o.ownerOrganizationId } else { "null" }
    $key = "$oid`_$orgId"
    if (-not $grouped[$key]) {
        $grouped[$key] = @()
    }
    $grouped[$key] += $o
}

$duplicates = $grouped.GetEnumerator() | Where-Object { $_.Value.Count -gt 1 }
if ($duplicates) {
    Write-Host "`nPHÁT HIỆN DUPLICATE:" -ForegroundColor Red
    foreach ($d in $duplicates) {
        Write-Host "  orderId=$($d.Key): $($d.Value.Count) bản ghi" -ForegroundColor Yellow
        foreach ($doc in $d.Value) {
            Write-Host "    - id: $($doc.id), createdAt: $($doc.createdAt)" -ForegroundColor Gray
        }
    }
} else {
    Write-Host "`nKhông có duplicate theo (orderId, ownerOrganizationId)" -ForegroundColor Green
}

# Test CIO ingest (POST /cio/ingest) — thay thế sync-upsert-one đã gỡ (Version 4.00)
if ($orders.Count -gt 0) {
    $sample = $orders[0]
    $orderId = $sample.orderId
    $orgId = $sample.ownerOrganizationId
    
    Write-Host "`nTest CIO ingest domain=order với orderId=$orderId..." -ForegroundColor Cyan
    $ingestBody = @{
        domain = "order"
        filter = @{ orderId = $orderId; ownerOrganizationId = $orgId }
        data   = $sample
    } | ConvertTo-Json -Depth 25
    try {
        $ingestResp = Invoke-RestMethod -Uri "$baseUrl/cio/ingest" -Method POST -Headers $headers -Body $ingestBody -ErrorAction Stop
        Write-Host "Kết quả: $($ingestResp | ConvertTo-Json -Depth 2)" -ForegroundColor Green
    } catch {
        Write-Host "Lỗi CIO ingest: $_" -ForegroundColor Red
    }
}
