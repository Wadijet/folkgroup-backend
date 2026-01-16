# Script Kiểm Tra Quyền Admin
# Sử dụng: .\check-admin-permissions.ps1

$adminToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTY2YzkwMGNiZjYyZGJhMGZjYWZkNGMiLCJ0aW1lIjoiNjk2NmM5MDAiLCJyYW5kb21OdW1iZXIiOiI1OCJ9.FflKAynO-2ArrbKWqTgRIAqIyQ13PrvHpjeB37E7MZI"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

Write-Host "`n=== KIEM TRA QUYEN ADMIN ===" -ForegroundColor Yellow

# Lay role ID
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "Role ID: $roleID" -ForegroundColor Cyan
    }
}
catch {
    Write-Host "Khong the lay role ID" -ForegroundColor Red
    exit 1
}

# Lay tat ca permissions
try {
    $allPermsResp = Invoke-RestMethod -Uri "$baseURL/permission/find" -Method GET -Headers $headers -ErrorAction Stop
    $allPermissions = $allPermsResp.data
    Write-Host "Tong so permissions trong he thong: $($allPermissions.Count)" -ForegroundColor Cyan
}
catch {
    Write-Host "Khong the lay danh sach permissions" -ForegroundColor Red
    $allPermissions = @()
}

# Lay role permissions cua admin
try {
    $permResp = Invoke-RestMethod -Uri "$baseURL/role-permission/find" -Method GET -Headers $headers -ErrorAction Stop
    $adminRolePerms = $permResp.data | Where-Object { $_.roleId -eq $roleID }
    Write-Host "So luong quyen cua Admin: $($adminRolePerms.Count)" -ForegroundColor Cyan
}
catch {
    Write-Host "Khong the lay role permissions" -ForegroundColor Red
    $adminRolePerms = @()
}

# So sanh
Write-Host "`n=== PHAN TICH ===" -ForegroundColor Yellow
if ($allPermissions.Count -gt 0) {
    $coverage = [math]::Round(($adminRolePerms.Count / $allPermissions.Count) * 100, 2)
    Write-Host "Ty le quyen: $coverage% ($($adminRolePerms.Count)/$($allPermissions.Count))" -ForegroundColor $(if ($coverage -ge 100) { "Green" } elseif ($coverage -ge 80) { "Yellow" } else { "Red" })
    
    if ($coverage -lt 100) {
        Write-Host "`n⚠️  Admin chua co du quyen!" -ForegroundColor Red
        Write-Host "Danh sach permissions thieu:" -ForegroundColor Yellow
        
        $adminPermIds = $adminRolePerms | ForEach-Object { $_.permissionId }
        $missingPerms = $allPermissions | Where-Object { $adminPermIds -notcontains $_.permissionId }
        
        foreach ($perm in $missingPerms) {
            Write-Host "  - $($perm.name) ($($perm.permissionId))" -ForegroundColor Red
        }
    } else {
        Write-Host "`n✅ Admin da co du quyen!" -ForegroundColor Green
    }
} else {
    Write-Host "Khong co permissions nao trong he thong" -ForegroundColor Yellow
}

# Goi sync permissions
Write-Host "`n=== DONG BO QUYEN ===" -ForegroundColor Yellow
try {
    $syncResp = Invoke-RestMethod -Uri "$baseURL/admin/sync-administrator-permissions" -Method POST -Headers $headers -ErrorAction Stop
    Write-Host "✅ Da dong bo quyen thanh cong!" -ForegroundColor Green
    Write-Host "Message: $($syncResp.message)" -ForegroundColor Green
}
catch {
    Write-Host "❌ Loi khi dong bo quyen: $($_.Exception.Message)" -ForegroundColor Red
}

# Kiem tra lai sau khi sync
Write-Host "`n=== KIEM TRA LAI SAU KHI SYNC ===" -ForegroundColor Yellow
try {
    $permResp2 = Invoke-RestMethod -Uri "$baseURL/role-permission/find" -Method GET -Headers $headers -ErrorAction Stop
    $adminRolePerms2 = $permResp2.data | Where-Object { $_.roleId -eq $roleID }
    Write-Host "So luong quyen sau sync: $($adminRolePerms2.Count)" -ForegroundColor Cyan
    
    if ($allPermissions.Count -gt 0) {
        $coverage2 = [math]::Round(($adminRolePerms2.Count / $allPermissions.Count) * 100, 2)
        Write-Host "Ty le quyen sau sync: $coverage2% ($($adminRolePerms2.Count)/$($allPermissions.Count))" -ForegroundColor $(if ($coverage2 -ge 100) { "Green" } elseif ($coverage2 -ge 80) { "Yellow" } else { "Red" })
    }
}
catch {
    Write-Host "Khong the kiem tra lai" -ForegroundColor Red
}

Write-Host "`n=== HOAN TAT ===" -ForegroundColor Green
