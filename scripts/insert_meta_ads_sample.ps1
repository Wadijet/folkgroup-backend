# Script chèn dữ liệu mẫu Meta Ads vào DB qua API
# Yêu cầu: Server đang chạy, bearer token admin hợp lệ
# Chạy: .\scripts\insert_meta_ads_sample.ps1

$baseUrl = "http://localhost:8080/api/v1"
$bearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OWE2NTVmZDMwYzAxN2ExNjVhYzk2ZDQiLCJ0aW1lIjoiNjlhODMwZWYiLCJyYW5kb21OdW1iZXIiOiI0MSJ9.1h3jxYF1veOoxpXo-Puh0BAs-4fgbY_bjFNqNiW96FU"
$orgId = "69a655f0088600c32e62f955"

# Bước 0: Lấy role từ /auth/roles
$baseHeaders = @{
    "Authorization" = "Bearer $bearerToken"
    "Content-Type"  = "application/json"
}
$activeRoleId = $null
try {
    $roleResp = Invoke-RestMethod -Uri "$baseUrl/auth/roles" -Method GET -Headers $baseHeaders
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $adminRole = $roleResp.data | Where-Object { $_.roleName -eq "Administrator" } | Select-Object -First 1
        $role = if ($adminRole) { $adminRole } else { $roleResp.data[0] }
        $activeRoleId = $role.roleId
        Write-Host "Dung role: $($role.roleName) (roleId: $activeRoleId)" -ForegroundColor Green
    }
} catch {
    Write-Host "Loi lay roles: $_" -ForegroundColor Red
    exit 1
}
if (-not $activeRoleId) {
    Write-Host "Loi: User khong co role" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization"       = "Bearer $bearerToken"
    "Content-Type"        = "application/json"
    "X-Active-Role-ID"    = $activeRoleId
}

# Danh sach Meta Ad Accounts mau (~10)
$adAccounts = @(
    @{ adAccountId = "act_123456789012345"; name = "Folkgroup Ad Account - VN"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012346"; name = "Folkgroup Ad Account - Test"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012347"; name = "Brand A - Vietnam"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012348"; name = "Brand B - SEA"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012349"; name = "E-commerce Store 1"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012350"; name = "E-commerce Store 2"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012351"; name = "Agency Client Alpha"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012352"; name = "Agency Client Beta"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012353"; name = "App Install Campaigns"; ownerOrganizationId = $orgId },
    @{ adAccountId = "act_123456789012354"; name = "Lead Gen - B2B"; ownerOrganizationId = $orgId }
)

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Chen du lieu mau Meta Ad Accounts" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

$successCount = 0
foreach ($acc in $adAccounts) {
    $body = $acc | ConvertTo-Json
    Write-Host "Dang tao: $($acc.name) ($($acc.adAccountId))..." -ForegroundColor Cyan
    try {
        $response = Invoke-RestMethod -Uri "$baseUrl/meta/ad-account/insert-one" -Method POST -Headers $headers -Body $body -ErrorAction Stop
        if ($response.status -eq "success") {
            Write-Host "  OK: Da tao thanh cong" -ForegroundColor Green
            $successCount++
        } else {
            Write-Host "  WARN: Response khong thanh cong" -ForegroundColor Yellow
        }
    } catch {
        if ($_.Exception.Response.StatusCode -eq 409 -or $_.ErrorDetails.Message -match "duplicate") {
            Write-Host "  SKIP: Ad account da ton tai (duplicate)" -ForegroundColor Yellow
        } else {
            Write-Host "  ERROR: $($_.Exception.Message)" -ForegroundColor Red
            if ($_.ErrorDetails.Message) { Write-Host "  $($_.ErrorDetails.Message)" -ForegroundColor Red }
        }
    }
    Start-Sleep -Milliseconds 300
}

Write-Host ""
Write-Host "Hoan thanh: $successCount ad account(s) da tao." -ForegroundColor Green
Write-Host "Luu y: meta_campaigns, meta_adsets, meta_ads, meta_ad_insights duoc sync tu Meta API." -ForegroundColor Cyan
Write-Host "       Du lieu mau nam trong docs-shared/ai-context/folkform/sample-data/" -ForegroundColor Cyan
