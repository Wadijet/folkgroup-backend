# Test luồng Ads + AI Decision (don gian nhat)
# Luong: POST /ads/actions/propose -> decision_events_queue (executor.propose_requested, domain=ads)
#        -> Worker AIDecisionConsumer -> ProposeForAds -> GET /ads/actions/pending
#
# Yeu cau:
# - API dang chay (vd: localhost:8080)
# - MongoDB; worker AIDecisionConsumer BAT (PUT /system/worker-config hoac mac dinh)
# - Token co quyen MetaAdAccount.Update (propose) + MetaCampaign.Read (lay campaign) + MetaAdAccount.Read (pending)
#
# Su dung: .\test-ads-aidecision-flow.ps1
# Hoac: $env:TEST_ADS_CAMPAIGN_ID="..."; $env:TEST_ADS_ACCOUNT_ID="act_..."; .\test-ads-aidecision-flow.ps1

# Dang nhap: xem resolve-test-bearer-token.ps1 — mac dinh daomanhdung86@gmail.com / 12345678 + FIREBASE_API_KEY tu development.env
$ErrorActionPreference = "Stop"
$baseURL = if ($env:TEST_API_BASE_URL) { $env:TEST_API_BASE_URL } else { "http://localhost:8080/api/v1" }

. (Join-Path $PSScriptRoot 'resolve-test-bearer-token.ps1')
try {
    $adminToken = Get-ApiTestBearerToken -ApiBaseUrl $baseURL -Hwid 'test_ads_aidecision_flow'
}
catch {
    Write-Host "Loi lay JWT: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type"  = "application/json"
}

Write-Host "=== TEST: Ads -> AI Decision -> Propose (PAUSE) ===" -ForegroundColor Cyan
Write-Host "Base: $baseURL" -ForegroundColor Gray

try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "X-Active-Role-ID: $roleID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Loi lay role: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$campaignId = $env:TEST_ADS_CAMPAIGN_ID
$adAccountId = $env:TEST_ADS_ACCOUNT_ID

if (-not $campaignId -or -not $adAccountId) {
    Write-Host "Lay 1 campaign tu GET /meta/campaign/find-with-pagination (filter rong)..." -ForegroundColor Yellow
    $filterEnc = [uri]::EscapeDataString("{}")
    try {
        $campResp = Invoke-RestMethod -Uri "$baseURL/meta/campaign/find-with-pagination?page=1&limit=1&filter=$filterEnc" -Method GET -Headers $headers -ErrorAction Stop
        $row = $null
        if ($campResp.data.items -and $campResp.data.items.Count -gt 0) {
            $row = $campResp.data.items[0]
        }
        if ($row) {
            if (-not $campaignId) { $campaignId = $row.campaignId }
            if (-not $adAccountId) { $adAccountId = $row.adAccountId }
        }
    }
    catch {
        Write-Host "Khong tu dong lay campaign: $($_.Exception.Message)" -ForegroundColor Yellow
    }
}

if (-not $campaignId -or -not $adAccountId) {
    Write-Host "CAN: dat `$env:TEST_ADS_CAMPAIGN_ID va `$env:TEST_ADS_ACCOUNT_ID (tu UI hoac Mongo meta_campaigns)." -ForegroundColor Red
    exit 1
}

Write-Host "campaignId=$campaignId adAccountId=$adAccountId" -ForegroundColor Green

# 1) Propose — emit executor.propose_requested (domain=ads)
$proposeBody = @{
    actionType    = "PAUSE"
    adAccountId   = $adAccountId
    campaignId    = $campaignId
    reason        = "Test luong Ads -> AI Decision (script api-tests)"
    ruleCode      = "test_script"
} | ConvertTo-Json -Compress

Write-Host "`n[1] POST /ads/actions/propose" -ForegroundColor Magenta
$proposeResp = Invoke-RestMethod -Uri "$baseURL/ads/actions/propose" -Method POST -Headers $headers -Body $proposeBody -ErrorAction Stop
if ($proposeResp.code -ne 202 -or -not $proposeResp.data.eventId) {
    Write-Host "FAIL: can HTTP 202 + data.eventId. Response: $($proposeResp | ConvertTo-Json -Compress)" -ForegroundColor Red
    exit 1
}
$eventId = $proposeResp.data.eventId
Write-Host "  OK eventId=$eventId (hang doi executor.propose_requested)" -ForegroundColor Green

# 2) Doi worker xu ly
$waitSec = 8
Write-Host "`n[2] Doi $waitSec giay (worker AIDecisionConsumer)..." -ForegroundColor Gray
Start-Sleep -Seconds $waitSec

# 3) Kiem tra pending
Write-Host "`n[3] GET /ads/actions/pending?limit=20" -ForegroundColor Magenta
$pendingResp = Invoke-RestMethod -Uri "$baseURL/ads/actions/pending?limit=20" -Method GET -Headers $headers -ErrorAction Stop
$list = $pendingResp.data
if (-not $list) {
    Write-Host 'FAIL: pending trong — kiem tra worker aidecision_consumer; xem GET /system/worker-config' -ForegroundColor Red
    exit 1
}

$found = $false
foreach ($p in $list) {
    if ($p.payload -and $p.payload.campaignId -eq $campaignId -and $p.actionType -eq "PAUSE") {
        $found = $true
        Write-Host "  OK tim thay action pending: id=$($p.id) status=$($p.status)" -ForegroundColor Green
        break
    }
}
if (-not $found) {
    Write-Host "CANH BAO: chua thay PAUSE cho campaign nay trong pending (co the idempotency hoac delay). Danh sach: $($list.Count) ban ghi." -ForegroundColor Yellow
}
else {
    Write-Host "`n=== LUONG KHOP (Ads propose -> queue -> worker -> Propose) ===" -ForegroundColor Cyan
}
