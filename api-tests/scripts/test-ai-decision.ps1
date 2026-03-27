# Script Test AI Decision API
# Su dung: .\test-ai-decision.ps1

$adminToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OWE2NTVmZDMwYzAxN2ExNjVhYzk2ZDQiLCJ0aW1lIjoiNjliYjE3YzIiLCJyYW5kb21OdW1iZXIiOiI5MSJ9.-KlAGsHjD0c5TkvdfNSdOGXam-GCORGFc03Gu-MxoKc"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

# Lay role ID de set active role (suy ra org)
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "Da set X-Active-Role-ID: $roleID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Khong the lay role ID: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

# Lay org ID cho payload
$orgID = $null
try {
    $orgResp = Invoke-RestMethod -Uri "$baseURL/organization/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($orgResp.data) {
        $orgID = $orgResp.data.organizationId
        if (-not $orgID) { $orgID = $orgResp.data._id }
        Write-Host "Org ID: $orgID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Khong the lay org ID, dung placeholder" -ForegroundColor Yellow
}
if (-not $orgID) { $orgID = "69a655f0088600c32e62f955" }

Write-Host "`n=== TEST AI DECISION API ===" -ForegroundColor Cyan

# --- 1. POST /ai-decision/events ---
Write-Host "`n[1] POST /ai-decision/events" -ForegroundColor Magenta
$eventBody = @{
    eventType   = "conversation.message_inserted"
    eventSource = "test"
    entityType  = "conversation"
    entityId    = "conv_test_001"
    orgId       = $orgID
    priority    = "high"
    lane        = "fast"
    payload     = @{
        conversationId = "conv_test_001"
        customerId     = "cust_test_001"
        channel        = "messenger"
    }
} | ConvertTo-Json -Depth 5

try {
    $eventResp = Invoke-RestMethod -Uri "$baseURL/ai-decision/events" -Method POST -Headers $headers -Body $eventBody -ErrorAction Stop
    Write-Host "  PASSED (HTTP 200)" -ForegroundColor Green
    Write-Host "  EventID: $($eventResp.data.eventId)" -ForegroundColor Gray
}
catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "  FAILED (HTTP $status): $($_.Exception.Message)" -ForegroundColor Red
}

# --- 2. POST /ai-decision/execute (202 + eventId — chi event-driven) ---
Write-Host "`n[2] POST /ai-decision/execute" -ForegroundColor Magenta
$executeBody = @{
    sessionUid  = "sess_test_001"
    customerUid = "cust_test_001"
    cixPayload  = @{
        actionSuggestions = @("escalate_to_senior")
    }
} | ConvertTo-Json -Depth 5

try {
    $executeResp = Invoke-RestMethod -Uri "$baseURL/ai-decision/execute" -Method POST -Headers $headers -Body $executeBody -ErrorAction Stop
    if ($executeResp.code -eq 202 -and $executeResp.data.eventId) {
        Write-Host "  PASSED (HTTP 202 — da xep hang)" -ForegroundColor Green
        Write-Host "  EventID: $($executeResp.data.eventId)" -ForegroundColor Gray
    }
    else {
        Write-Host "  FAILED: response khong hop le (can code 202 + data.eventId)" -ForegroundColor Red
    }
}
catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "  FAILED (HTTP $status): $($_.Exception.Message)" -ForegroundColor Red
}

# --- 3. POST /ai-decision/events - order.recompute_requested (Phase 2) ---
Write-Host "`n[3] POST /ai-decision/events (order.recompute_requested)" -ForegroundColor Magenta
$orderEventBody = @{
    eventType   = "order.recompute_requested"
    eventSource = "test"
    entityType  = "order"
    entityId    = "ord_test_001"
    orgId       = $orgID
    priority    = "normal"
    lane        = "normal"
    payload     = @{
        orderId        = "ord_test_001"
        customerId     = "cust_test_001"
        conversationId = "conv_test_001"
    }
} | ConvertTo-Json -Depth 5
try {
    $orderEvtResp = Invoke-RestMethod -Uri "$baseURL/ai-decision/events" -Method POST -Headers $headers -Body $orderEventBody -ErrorAction Stop
    Write-Host "  PASSED (HTTP 200) - Order worker se consume" -ForegroundColor Green
    Write-Host "  EventID: $($orderEvtResp.data.eventId)" -ForegroundColor Gray
}
catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "  FAILED (HTTP $status): $($_.Exception.Message)" -ForegroundColor Red
}

# --- 4. POST /ai-decision/events - ads.context_requested (Phase 2) ---
Write-Host "`n[4] POST /ai-decision/events (ads.context_requested)" -ForegroundColor Magenta
$adsEventBody = @{
    eventType   = "ads.context_requested"
    eventSource = "test"
    entityType  = "ad_account"
    entityId    = "act_test_001"
    orgId       = $orgID
    priority    = "normal"
    lane        = "batch"
    payload     = @{
        adAccountId = "act_test_001"
    }
} | ConvertTo-Json -Depth 5
try {
    $adsEvtResp = Invoke-RestMethod -Uri "$baseURL/ai-decision/events" -Method POST -Headers $headers -Body $adsEventBody -ErrorAction Stop
    Write-Host "  PASSED (HTTP 200) - Ads worker se consume" -ForegroundColor Green
    Write-Host "  EventID: $($adsEvtResp.data.eventId)" -ForegroundColor Gray
}
catch {
    $status = $_.Exception.Response.StatusCode.value__
    Write-Host "  FAILED (HTTP $status): $($_.Exception.Message)" -ForegroundColor Red
}

# --- 5. POST /ai-decision/cases/:id/close (case khong ton tai -> 404) ---
Write-Host "`n[5] POST /ai-decision/cases/:id/close" -ForegroundColor Magenta
$caseId = "dcs_test_placeholder"
try {
    $closeResp = Invoke-RestMethod -Uri "$baseURL/ai-decision/cases/$caseId/close" -Method POST -Headers $headers -ErrorAction Stop
    Write-Host "  PASSED (HTTP 200)" -ForegroundColor Green
}
catch {
    $status = $_.Exception.Response.StatusCode.value__
    if ($status -eq 404) {
        Write-Host "  PASSED (404 - case khong ton tai, dung nhu mong doi)" -ForegroundColor Green
    }
    else {
        Write-Host "  FAILED (HTTP $status): $($_.Exception.Message)" -ForegroundColor Red
    }
}

# Doi worker xu ly (Order 10s, Ads 30s)
Write-Host "`n  (Doi 5s de Order/Ads workers xu ly...)" -ForegroundColor Gray
Start-Sleep -Seconds 5

Write-Host "`n=== KET THUC TEST PHASE 1 + 2 ===" -ForegroundColor Cyan
