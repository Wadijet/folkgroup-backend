# E2E: Bot/CIO -> DB (meta_campaigns) -> OnDataChanged -> (legacy: meta_campaign.*; luồng chính: recompute -> campaign_intel_recomputed)
#       -> AI Decision -> ads.context_requested -> (cung consumer AID) ads.context_ready
#       -> (env) executor.propose_requested (domain=ads) -> AI Decision -> Propose -> Executor (pending)
#       -> Learning: khi action dong (OnActionClosed) — khong tu dong trong script
#
# BAT TRUOC KHI CHAY SERVER: worker aidecision_consumer (xu ly ads.context_* + executor.propose_requested)
#
# Su dung: .\test-cio-ads-ai-decision-e2e.ps1
# Dang nhap: uu tien TEST_ADMIN_TOKEN; khong thi email/password Firebase + api/config/env/development.env (FIREBASE_API_KEY)
# Mac dinh email: daomanhdung86@gmail.com / 12345678 — ghi de bang TEST_EMAIL, TEST_PASSWORD

$ErrorActionPreference = "Stop"
$baseURL = if ($env:TEST_API_BASE_URL) { $env:TEST_API_BASE_URL } else { "http://localhost:8080/api/v1" }

. (Join-Path $PSScriptRoot 'resolve-test-bearer-token.ps1')
try {
    $adminToken = Get-ApiTestBearerToken -ApiBaseUrl $baseURL -Hwid 'e2e_cio_ads_ai_decision'
}
catch {
    Write-Host "Loi lay JWT: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type"  = "application/json"
}

# Cung pattern voi test-ads-aidecision-flow.ps1: base URL, role, X-Active-Role-ID, buoc [1][2][3], pending
Write-Host "=== E2E CIO -> Ads Intelligence -> AI Decision -> Propose ===" -ForegroundColor Cyan
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

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
$campaignMetaId = "test_cio_pipeline_$suffix"
$adAccountId = if ($env:TEST_ADS_ACCOUNT_ID) { $env:TEST_ADS_ACCOUNT_ID } else { "act_test_pipeline_$suffix" }

# Body giong Meta Marketing API (metaData) — CIO ingest -> HandleSyncUpsertOne -> Upsert meta_campaigns -> OnDataChanged
$ingestBody = @{
    domain = "meta_campaign"
    data   = @{
        metaData = @{
            id         = $campaignMetaId
            account_id = $adAccountId
            name       = "E2E Campaign Test"
            status     = "PAUSED"
            objective  = "OUTCOME_TRAFFIC"
        }
    }
} | ConvertTo-Json -Depth 6

Write-Host "`n[1] POST /cio/ingest domain=meta_campaign; campaignId=$campaignMetaId" -ForegroundColor Magenta
try {
    $ing = Invoke-RestMethod -Uri "$baseURL/cio/ingest" -Method POST -Headers $headers -Body $ingestBody -ErrorAction Stop
    Write-Host "  OK ingest: $($ing.message)" -ForegroundColor Green
}
catch {
    Write-Host "  FAIL: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$waitSec = 20
Write-Host "`n[2] Doi $waitSec giay cho AIDecisionConsumer, AdsContextWorker va executor.propose_requested..." -ForegroundColor Gray
Start-Sleep -Seconds $waitSec

Write-Host "`n[3] GET /ads/actions/pending?limit=30" -ForegroundColor Magenta
try {
    $pend = Invoke-RestMethod -Uri "$baseURL/ads/actions/pending?limit=30" -Method GET -Headers $headers -ErrorAction Stop
    $list = $pend.data
    if (-not $list) {
        Write-Host 'FAIL: pending trong — kiem tra worker aidecision_consumer; endpoint GET /system/worker-config' -ForegroundColor Red
    }
    else {
        $hit = $false
        foreach ($p in $list) {
            if ($p.payload -and $p.payload.campaignId -eq $campaignMetaId) {
                $hit = $true
                Write-Host "  OK tim thay action pending: actionType=$($p.actionType) id=$($p.id) status=$($p.status)" -ForegroundColor Green
                break
            }
        }
        if (-not $hit) {
            Write-Host "CANH BAO: chua thay campaignId=$campaignMetaId trong pending. So ban ghi pending: $($list.Count)." -ForegroundColor Yellow
            Write-Host '  Kiem tra: campaign co alertFlags + rule engine khop; ads_meta_config (autoPropose)' -ForegroundColor Gray
            Write-Host '  Worker: aidecision_consumer; xem log server' -ForegroundColor Gray
        }
        else {
            Write-Host "`n=== LUONG KHOP: CIO ingest -> queue -> Ads context -> AI Decision -> Propose -> pending ===" -ForegroundColor Cyan
        }
    }
}
catch {
    Write-Host "  FAIL: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host "`n=== Ket thuc E2E ===" -ForegroundColor Cyan
