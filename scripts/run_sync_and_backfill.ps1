# Script chạy sync và backfill CRM.
# Cần: $env:TOKEN (Bearer token admin)
# Option: $env:BASE_URL (mặc định http://localhost:8080)
# Option: $env:OWNER_ORG_ID (override org; nếu không có sẽ dùng org từ role)
#
# Chạy: $env:TOKEN="your_admin_jwt"; .\scripts\run_sync_and_backfill.ps1
#
# Luồng: 1) GET /auth/roles lấy role → 2) Thêm X-Active-Role-ID vào header → 3) Gọi sync + backfill

$baseUrl = if ($env:BASE_URL) { $env:BASE_URL } else { "http://localhost:8080" }
$apiBase = "$baseUrl/api/v1"
$token = $env:TOKEN
$ownerOrgId = $env:OWNER_ORG_ID

if (-not $token) {
    Write-Host "Lỗi: Cần set TOKEN (admin). Ví dụ: `$env:TOKEN=`"eyJhbGc...`"" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Bước 1: Lấy role từ /auth/roles — BẮT BUỘC vì route sync/backfill yêu cầu X-Active-Role-ID
Write-Host "`n[Bước 0] Lấy role từ GET /auth/roles..." -ForegroundColor Cyan
$activeRoleId = $null
try {
    $roleResp = Invoke-RestMethod -Uri "$apiBase/auth/roles" -Method GET -Headers $headers
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        # Ưu tiên role Administrator, không thì dùng role đầu tiên
        $adminRole = $roleResp.data | Where-Object { $_.roleName -eq "Administrator" } | Select-Object -First 1
        $role = if ($adminRole) { $adminRole } else { $roleResp.data[0] }
        $activeRoleId = $role.roleId
        Write-Host "  Dùng role: $($role.roleName) (roleId: $activeRoleId)" -ForegroundColor Green
        $headers["X-Active-Role-ID"] = $activeRoleId
    }
} catch {
    Write-Host "  Lỗi lấy roles: $_" -ForegroundColor Red
    exit 1
}
if (-not $activeRoleId) {
    Write-Host "Lỗi: User không có role nào hoặc response không hợp lệ" -ForegroundColor Red
    exit 1
}

# ownerOrganizationId: override nếu có env; không thì backend dùng org từ role
if (-not $ownerOrgId) {
    $ownerOrgId = go run scripts/get_first_org_id.go 2>$null
    if ($ownerOrgId) { Write-Host "  Dùng ownerOrganizationId override: $ownerOrgId" -ForegroundColor Gray }
}

Write-Host "`n=== 1. POST /api/v1/customers/sync ===" -ForegroundColor Cyan
$syncBody = if ($ownerOrgId) { @{ ownerOrganizationId = $ownerOrgId } | ConvertTo-Json } else { "{}" }
try {
    $syncResp = Invoke-RestMethod -Uri "$apiBase/customers/sync" -Method Post -Headers $headers -Body $syncBody
    Write-Host "Sync: $($syncResp | ConvertTo-Json -Compress)" -ForegroundColor Green
} catch {
    Write-Host "Sync lỗi: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== 2. POST /api/v1/customers/backfill-activity ===" -ForegroundColor Cyan
$backfillBody = if ($ownerOrgId) { @{ ownerOrganizationId = $ownerOrgId } | ConvertTo-Json } else { "{}" }
try {
    $backfillResp = Invoke-RestMethod -Uri "$apiBase/customers/backfill-activity" -Method Post -Headers $headers -Body $backfillBody
    Write-Host "Backfill: $($backfillResp | ConvertTo-Json -Compress)" -ForegroundColor Green
} catch {
    Write-Host "Backfill lỗi: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`nHoàn tất." -ForegroundColor Green
