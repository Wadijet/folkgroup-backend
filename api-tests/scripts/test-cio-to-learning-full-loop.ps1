# E2E mot vong: CIO ingest -> co action pending -> approve -> execute -> learning_cases
#
# Vi sao truoc day "dung giua chung": pipeline CIO -> ads.context_ready -> emit propose
# can aidecision_consumer + policy (rule engine / autoPropose).
# Neu khong co pending sau buoc cho, script mac dinh FALLBACK: POST /ads/actions/propose
# cung campaign/ad account vua ingest — van la 1 vong nghiep vu "co campaign tu CIO, roi Executor + Learning".
#
# Bat buoc: worker aidecision_consumer (xu ly executor.propose_requested). Domain ads = deferred -> can POST execute.
#
# Bien moi truong:
#   CIO_FULL_LOOP_WAIT_SEC       - doi sau ingest truoc khi poll pending (mac dinh 20)
#   CIO_FULL_LOOP_PENDING_POLLS  - so lan poll pending, moi lan cach CIO_FULL_LOOP_POLL_INTERVAL_SEC (mac dinh 5 lan / 4 giay)
#   CIO_FULL_LOOP_STRICT_CIO_ONLY - dat "1" de KHONG dung fallback propose — chi CIO tu dong (se fail neu pipeline khong emit)
#   TEST_ADS_ACCOUNT_ID          - ghi de ad account (propose dung dang act_...)
#   TEST_ADS_CAMPAIGN_ID         - ghi de campaign Meta (id so)
#   CIO_FULL_LOOP_USE_SAMPLE_DATA - dat "0" de khong doc mau; mac dinh doc neu co file meta_campaigns.json
#
# Du lieu mau: docs-shared/ai-context/folkform/sample-data/meta_campaigns.json (ban ghi dau tien)
#
# Chay: .\test-cio-to-learning-full-loop.ps1

$ErrorActionPreference = "Stop"
$baseURL = if ($env:TEST_API_BASE_URL) { $env:TEST_API_BASE_URL } else { "http://localhost:8080/api/v1" }
$strictCioOnly = ($env:CIO_FULL_LOOP_STRICT_CIO_ONLY -eq "1")

. (Join-Path $PSScriptRoot 'resolve-test-bearer-token.ps1')
try {
    $adminToken = Get-ApiTestBearerToken -ApiBaseUrl $baseURL -Hwid 'e2e_cio_to_learning_full'
}
catch {
    Write-Host "Loi lay JWT: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type"  = "application/json"
}

Write-Host "=== FULL LOOP: CIO -> Pending -> Approve -> Execute -> Learning ===" -ForegroundColor Cyan
if ($strictCioOnly) {
    Write-Host "Che do: STRICT CIO ONLY - khong fallback propose" -ForegroundColor Yellow
}
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

$projectRoot = (Resolve-Path (Join-Path $PSScriptRoot '..\..')).Path
$sampleCampaignsJson = Join-Path $projectRoot 'docs-shared\ai-context\folkform\sample-data\meta_campaigns.json'

$campaignMetaId = $env:TEST_ADS_CAMPAIGN_ID
$adAccountId = $env:TEST_ADS_ACCOUNT_ID
$ingestName = "E2E Full Loop Campaign"
$ingestObjective = "OUTCOME_TRAFFIC"

if ((-not $campaignMetaId -or -not $adAccountId) -and ($env:CIO_FULL_LOOP_USE_SAMPLE_DATA -ne '0') -and (Test-Path -LiteralPath $sampleCampaignsJson)) {
    $txtSample = Get-Content -LiteralPath $sampleCampaignsJson -Raw -Encoding UTF8
    # Ban ghi dau: adAccountId roi campaignId (dung voi format export meta_campaigns.json)
    if ($txtSample -match '"adAccountId"\s*:\s*"([^"]+)"\s*,\s*\r?\n\s*"campaignId"\s*:\s*"([^"]+)"') {
        if (-not $adAccountId) { $adAccountId = $matches[1] }
        if (-not $campaignMetaId) { $campaignMetaId = $matches[2] }
        Write-Host "Da doc mau: docs-shared/ai-context/folkform/sample-data/meta_campaigns.json (ban ghi dau)" -ForegroundColor Gray
        Write-Host "  campaignId=$campaignMetaId adAccountId=$adAccountId" -ForegroundColor Gray
    }
}

$suffix = [DateTimeOffset]::UtcNow.ToUnixTimeSeconds()
if (-not $campaignMetaId) { $campaignMetaId = "test_cio_full_$suffix" }
if (-not $adAccountId) { $adAccountId = "act_test_full_$suffix" }

# Propose / API ads: uu tien adAccountId dang act_
if ($adAccountId -notmatch '^act_') {
    $adAccountId = 'act_' + $adAccountId
}
# CIO metaData.account_id: Meta dung id so, khong prefix act_
$accountIdForIngest = $adAccountId -replace '^act_', ''

$ingestBody = @{
    domain = "meta_campaign"
    data   = @{
        metaData = @{
            id         = $campaignMetaId
            account_id = $accountIdForIngest
            name       = $ingestName
            status     = "PAUSED"
            objective  = $ingestObjective
        }
    }
} | ConvertTo-Json -Depth 6

Write-Host "`n[1] POST /cio/ingest - upsert meta_campaigns" -ForegroundColor Magenta
try {
    $ing = Invoke-RestMethod -Uri "$baseURL/cio/ingest" -Method POST -Headers $headers -Body $ingestBody -ErrorAction Stop
    Write-Host "  OK: $($ing.message)" -ForegroundColor Green
}
catch {
    Write-Host "  FAIL: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

$waitAfterIngest = 20
if ($env:CIO_FULL_LOOP_WAIT_SEC) { $waitAfterIngest = [int]$env:CIO_FULL_LOOP_WAIT_SEC }
Write-Host "`n[2] Doi $waitAfterIngest giay sau ingest..." -ForegroundColor Gray
Start-Sleep -Seconds $waitAfterIngest

$pollCount = 5
$pollSec = 4
if ($env:CIO_FULL_LOOP_PENDING_POLLS) { $pollCount = [int]$env:CIO_FULL_LOOP_PENDING_POLLS }
if ($env:CIO_FULL_LOOP_POLL_INTERVAL_SEC) { $pollSec = [int]$env:CIO_FULL_LOOP_POLL_INTERVAL_SEC }

# Tim action theo campaign. -ActiveOnly: chi pending + queued — tranh ban failed cu lam dung luong.
# -RuleCode: chi khop payload.ruleCode (sau propose fallback voi ruleCode moi moi lan chay).
function Find-AdsActionRowForCampaign {
    param(
        [string]$CampId,
        [string]$RuleCode = $null,
        [switch]$ActiveOnly
    )
    function Test-ActionRowMatch {
        param($row)
        if (-not $row.payload -or $row.payload.campaignId -ne $CampId) { return $false }
        if ($RuleCode -and [string]$row.payload.ruleCode -ne $RuleCode) { return $false }
        return $true
    }
    try {
        $pr = Invoke-RestMethod -Uri "$baseURL/ads/actions/pending?limit=50" -Method GET -Headers $headers -ErrorAction Stop
        $lst = $pr.data
        if ($lst) {
            foreach ($x in $lst) {
                if (Test-ActionRowMatch $x) {
                    return @{ Id = [string]$x.id; Status = [string]$x.status }
                }
            }
        }
    }
    catch { }
    # ActiveOnly: pending + queued (ListPending chi pending; auto-approve chi con queued)
    $statuses = if ($ActiveOnly) { @('pending', 'queued') } else { @('queued', 'executed', 'failed', 'rejected') }
    foreach ($st in $statuses) {
        try {
            $fu = ('{0}/ads/actions/find?domain=ads&status={1}&limit=50' -f $baseURL, $st)
            $fr = Invoke-RestMethod -Uri $fu -Method GET -Headers $headers -ErrorAction Stop
            $lst2 = $fr.data
            if (-not $lst2) { continue }
            foreach ($x in $lst2) {
                if (Test-ActionRowMatch $x) {
                    return @{ Id = [string]$x.id; Status = [string]$x.status }
                }
            }
        }
        catch { }
    }
    return $null
}

# ruleCode moi moi lan — idempotencyKey khac — luon tao action moi khi fallback propose
$runRuleCode = 'full_loop_' + [DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds().ToString()

Write-Host "`n[3] Poll action ads (chi pending/queued) campaignId=$campaignMetaId - toi da $pollCount lan, moi $pollSec giay" -ForegroundColor Magenta
$actionRow = $null
for ($i = 0; $i -lt $pollCount; $i++) {
    $actionRow = Find-AdsActionRowForCampaign -CampId $campaignMetaId -ActiveOnly
    if ($actionRow) {
        Write-Host "  OK id=$($actionRow.Id) status=$($actionRow.Status) - lan poll $($i + 1)" -ForegroundColor Green
        break
    }
    if ($i -lt $pollCount - 1) {
        Start-Sleep -Seconds $pollSec
    }
}

if (-not $actionRow -and -not $strictCioOnly) {
    Write-Host "`n[3b] Fallback POST /ads/actions/propose - ruleCode=$runRuleCode - worker ai_decision_consumer" -ForegroundColor Yellow
    $proposeBody = @{
        actionType  = "PAUSE"
        adAccountId = $adAccountId
        campaignId  = $campaignMetaId
        reason      = "Full loop E2E: fallback sau CIO, moi lan chay ruleCode khac"
        ruleCode    = $runRuleCode
    } | ConvertTo-Json -Compress
    try {
        $prop = Invoke-RestMethod -Uri "$baseURL/ads/actions/propose" -Method POST -Headers $headers -Body $proposeBody -ErrorAction Stop
        if ($prop.code -ne 202 -or -not $prop.data.eventId) {
            Write-Host "FAIL propose: can code 202 va data.eventId. Phan hoi: $($prop | ConvertTo-Json -Compress)" -ForegroundColor Red
            exit 1
        }
        Write-Host "  OK eventId=$($prop.data.eventId) - doi worker..." -ForegroundColor Green
    }
    catch {
        Write-Host "FAIL propose: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
    # Consumer co the cham; tang poll neu can: CIO_FULL_LOOP_AFTER_PROPOSE_POLLS / CIO_FULL_LOOP_AFTER_PROPOSE_SEC
    $afterProposePolls = 15
    $afterProposeSec = 5
    if ($env:CIO_FULL_LOOP_AFTER_PROPOSE_POLLS) { $afterProposePolls = [int]$env:CIO_FULL_LOOP_AFTER_PROPOSE_POLLS }
    if ($env:CIO_FULL_LOOP_AFTER_PROPOSE_SEC) { $afterProposeSec = [int]$env:CIO_FULL_LOOP_AFTER_PROPOSE_SEC }
    for ($i = 0; $i -lt $afterProposePolls; $i++) {
        Start-Sleep -Seconds $afterProposeSec
        $actionRow = Find-AdsActionRowForCampaign -CampId $campaignMetaId -RuleCode $runRuleCode -ActiveOnly
        if ($actionRow) {
            Write-Host "  OK id=$($actionRow.Id) status=$($actionRow.Status) sau propose - lan $($i + 1)" -ForegroundColor Green
            break
        }
    }
}

if (-not $actionRow) {
    Write-Host "FAIL: van khong tim thay action ads cho campaign nay." -ForegroundColor Red
    Write-Host "  Worker ten API: ai_decision_consumer - GET /system/worker-config - active phai true" -ForegroundColor Gray
    Write-Host "  Khong dat WORKER_ACTIVE_AI_DECISION_CONSUMER=0 hoac false - se tat consumer" -ForegroundColor Gray
    if ($strictCioOnly) {
        Write-Host "  Strict CIO: bat aidecision_consumer (queue + rule engine)" -ForegroundColor Gray
    }
    exit 1
}

$actionId = $actionRow.Id
$actionStatus = $actionRow.Status
$approveBody = @{ actionId = $actionId } | ConvertTo-Json

if ($actionStatus -eq 'executed' -or $actionStatus -eq 'failed' -or $actionStatus -eq 'rejected') {
    Write-Host "`n[4][5] Bo qua approve/execute - action da o trang thai $actionStatus" -ForegroundColor Yellow
}
elseif ($actionStatus -eq 'queued') {
    Write-Host "`n[4] Bo qua approve - ResolveImmediate da auto-approve, action dang queued" -ForegroundColor Yellow
    Write-Host "`n[5] POST /ads/actions/execute" -ForegroundColor Magenta
    try {
        $ex = Invoke-RestMethod -Uri "$baseURL/ads/actions/execute" -Method POST -Headers $headers -Body $approveBody -ErrorAction Stop
        Write-Host "  OK sau execute: status=$($ex.data.status)" -ForegroundColor Green
    }
    catch {
        # Campaign test gia Meta se 500 — engine van ghi failed + OnActionClosed; tiep tuc poll learning
        Write-Host "  CANH BAO execute HTTP loi: $($_.Exception.Message) - van thu poll learning (action co the da failed)" -ForegroundColor Yellow
    }
}
else {
    Write-Host "`n[4] POST /ads/actions/approve" -ForegroundColor Magenta
    try {
        $ap = Invoke-RestMethod -Uri "$baseURL/ads/actions/approve" -Method POST -Headers $headers -Body $approveBody -ErrorAction Stop
        Write-Host "  OK sau approve: status=$($ap.data.status)" -ForegroundColor Green
    }
    catch {
        Write-Host "FAIL approve: $($_.Exception.Message)" -ForegroundColor Red
        exit 1
    }
    if ($ap.data.status -eq 'queued') {
        Write-Host "`n[5] POST /ads/actions/execute" -ForegroundColor Magenta
        try {
            $ex = Invoke-RestMethod -Uri "$baseURL/ads/actions/execute" -Method POST -Headers $headers -Body $approveBody -ErrorAction Stop
            Write-Host "  OK sau execute: status=$($ex.data.status)" -ForegroundColor Green
        }
        catch {
            Write-Host "  CANH BAO execute HTTP loi: $($_.Exception.Message) - van thu poll learning" -ForegroundColor Yellow
        }
    }
    else {
        Write-Host "`n[5] Bo qua execute - status sau approve khong phai queued" -ForegroundColor Yellow
    }
}

Start-Sleep -Seconds 2

Write-Host "`n[6] GET /learning/cases - sourceRefId = actionId" -ForegroundColor Magenta
$foundLc = $false
$maxTry = 12
$interval = 2
for ($i = 0; $i -lt $maxTry; $i++) {
    try {
        $enc = [uri]::EscapeDataString($actionId)
        $lcUrl = ('{0}/learning/cases?domain=ads&sourceRefId={1}&limit=5&sortField=closedAt&sortOrder=-1' -f $baseURL, $enc)
        $lcResp = Invoke-RestMethod -Uri $lcUrl -Method GET -Headers $headers -ErrorAction Stop
        $items = $lcResp.data.items
        if ($items -and $items.Count -gt 0) {
            $foundLc = $true
            $first = $items[0]
            Write-Host "  OK learning case: id=$($first.id) result=$($first.result) sourceRefId=$($first.sourceRefId)" -ForegroundColor Green
            break
        }
    }
    catch {
        Write-Host "  Poll learning loi: $($_.Exception.Message)" -ForegroundColor Yellow
    }
    if ($i -lt $maxTry - 1) {
        Start-Sleep -Seconds $interval
    }
}

if (-not $foundLc) {
    Write-Host "FAIL: chua thay learning case. Kiem tra AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE va decision case closure." -ForegroundColor Red
    Write-Host "  Goi y debug: AI_DECISION_LEARNING_SKIP_INCOMPLETE_CLOSURE = 0" -ForegroundColor Gray
    exit 1
}

Write-Host ""
Write-Host "=== FULL LOOP HOAN TAT ===" -ForegroundColor Cyan
