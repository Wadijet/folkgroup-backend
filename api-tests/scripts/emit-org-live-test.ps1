# Emit AI Decision (execute_requested) + doc GET org-live/timeline — test buffer/Mongo.
# Can: $env:API_BASE_URL = 'http://localhost:8080/api/v1'
#      $env:TEST_ADMIN_TOKEN hoac Firebase trong development.env
# Chay: .\emit-org-live-test.ps1

$ErrorActionPreference = 'Stop'
. (Join-Path $PSScriptRoot 'resolve-test-bearer-token.ps1')

$baseURL = if ($env:API_BASE_URL) { $env:API_BASE_URL.TrimEnd('/') } else { 'http://localhost:8080/api/v1' }
Write-Host "API: $baseURL" -ForegroundColor Cyan

$adminToken = Get-ApiTestBearerToken -ApiBaseUrl $baseURL
$headers = @{
    'Authorization'  = "Bearer $adminToken"
    'Content-Type'   = 'application/json'
}

$roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers
if (-not $roleResp.data -or $roleResp.data.Count -lt 1) {
    throw 'Khong co role trong /auth/roles'
}
$headers['X-Active-Role-ID'] = [string]$roleResp.data[0].roleId
Write-Host "X-Active-Role-ID: $($headers['X-Active-Role-ID'])" -ForegroundColor Green

$suffix = [guid]::NewGuid().ToString('N').Substring(0, 8)
$executeBody = @{
    sessionUid  = "sess_orglive_$suffix"
    customerUid = 'cust_orglive_test'
    cixPayload  = @{
        actionSuggestions = @('escalate_to_senior')
    }
} | ConvertTo-Json -Depth 6

Write-Host "`nPOST /ai-decision/execute ..." -ForegroundColor Magenta
try {
    $executeResp = Invoke-WebRequest -Uri "$baseURL/ai-decision/execute" -Method POST -Headers $headers -Body $executeBody -UseBasicParsing
    Write-Host "  Status: $($executeResp.StatusCode)" -ForegroundColor Green
    $j = $executeResp.Content | ConvertFrom-Json
    Write-Host "  traceId: $($j.data.traceId) eventId: $($j.data.eventId)" -ForegroundColor Gray
}
catch {
    Write-Host "  LOI execute: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.Exception.Response) {
        $r = $_.Exception.Response
        $reader = [System.IO.StreamReader]::new($r.GetResponseStream())
        Write-Host $reader.ReadToEnd() -ForegroundColor Yellow
    }
    exit 1
}

Start-Sleep -Milliseconds 500

Write-Host "`nGET /ai-decision/org-live/timeline ..." -ForegroundColor Magenta
try {
    $tlResp = Invoke-WebRequest -Uri "$baseURL/ai-decision/org-live/timeline" -Method GET -Headers $headers -UseBasicParsing
    $tl = $tlResp.Content | ConvertFrom-Json
    $ev = $tl.data.events
    $n = if ($null -eq $ev) { 0 } elseif ($ev -is [System.Array]) { $ev.Count } else { 1 }
    Write-Host "  Status: $($tlResp.StatusCode) | So su kien: $n" -ForegroundColor $(if ($n -gt 0) { 'Green' } else { 'Yellow' })
    if ($n -gt 0 -and $ev.Count -le 5) {
        $ev | ForEach-Object { Write-Host "  - phase=$($_.phase) summary=$($_.summary)" -ForegroundColor Gray }
    }
}
catch {
    Write-Host "  LOI timeline: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "`nXong." -ForegroundColor Cyan
