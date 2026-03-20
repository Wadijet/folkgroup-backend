# Script Test Phase 3 — Learning & Rule Suggestions
# Sử dụng: .\test-phase3-learning.ps1
# Hoặc: $env:TEST_TOKEN="your_jwt"; .\test-phase3-learning.ps1

$adminToken = if ($env:TEST_TOKEN) { $env:TEST_TOKEN } else {
    "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTY2YzkwMGNiZjYyZGJhMGZjYWZkNGMiLCJ0aW1lIjoiNjk2NmM5MDAiLCJyYW5kb21OdW1iZXIiOiI1OCJ9.FflKAynO-2ArrbKWqTgRIAqIyQ13PrvHpjeB37E7MZI"
}
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

Write-Host "`n=== Phase 3: Learning & Rule Suggestions Test ===" -ForegroundColor Cyan

# 0. Health check (không cần auth)
Write-Host "`n[0] GET /system/health" -ForegroundColor Yellow
try {
    $health = Invoke-WebRequest -Uri "$baseURL/system/health" -Method GET -ErrorAction Stop
    if ($health.StatusCode -eq 200) {
        Write-Host "  PASSED - Server dang chay" -ForegroundColor Green
    }
}
catch {
    Write-Host "  FAILED - Server chua san sang. Chay: cd api; go run ./cmd/server" -ForegroundColor Red
    exit 1
}

# Setup: Lấy role ID để set active role (cần cho org context)
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "Da set X-Active-Role-ID: $roleID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Token co the het han (401). Test endpoint ton tai bang request khong token..." -ForegroundColor Yellow
    # Test endpoint tồn tại: không token → 401 (endpoint có), 404 (endpoint không có)
    try {
        $noAuth = Invoke-WebRequest -Uri "$baseURL/learning/rule-suggestions" -Method GET -ErrorAction SilentlyContinue
    } catch {
        if ($_.Exception.Response.StatusCode.value__ -eq 401) {
            Write-Host "  Endpoint /learning/rule-suggestions TON TAI (401 = yeu cau auth)" -ForegroundColor Green
        }
    }
    Write-Host "De test day du, set bien moi: `$env:TEST_TOKEN=`"<jwt_valid>`"" -ForegroundColor Gray
    exit 1
}

$passed = 0
$failed = 0

# 1. GET /learning/cases
Write-Host "`n[1] GET /learning/cases" -ForegroundColor Yellow
try {
    $resp = Invoke-WebRequest -Uri "$baseURL/learning/cases" -Method GET -Headers $headers -ErrorAction Stop
    $json = $resp.Content | ConvertFrom-Json
    if ($resp.StatusCode -eq 200 -and $json.status -eq "success") {
        Write-Host "  PASSED (HTTP $($resp.StatusCode))" -ForegroundColor Green
        Write-Host "  items: $($json.data.itemCount), total: $($json.data.total)" -ForegroundColor Gray
        $passed++
    } else {
        Write-Host "  FAILED (status=$($json.status))" -ForegroundColor Red
        $failed++
    }
}
catch {
    Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red
    $failed++
}

# 2. GET /learning/rule-suggestions (Phase 3 mới)
Write-Host "`n[2] GET /learning/rule-suggestions" -ForegroundColor Yellow
try {
    $resp = Invoke-WebRequest -Uri "$baseURL/learning/rule-suggestions" -Method GET -Headers $headers -ErrorAction Stop
    $json = $resp.Content | ConvertFrom-Json
    if ($resp.StatusCode -eq 200 -and $json.status -eq "success") {
        Write-Host "  PASSED (HTTP $($resp.StatusCode))" -ForegroundColor Green
        Write-Host "  items: $($json.data.itemCount), total: $($json.data.total)" -ForegroundColor Gray
        $passed++
    } else {
        Write-Host "  FAILED (status=$($json.status))" -ForegroundColor Red
        $failed++
    }
}
catch {
    Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red
    $failed++
}

# 3. GET /learning/rule-suggestions với query params
Write-Host "`n[3] GET /learning/rule-suggestions?status=pending&limit=10" -ForegroundColor Yellow
try {
    $resp = Invoke-WebRequest -Uri "$baseURL/learning/rule-suggestions?status=pending&limit=10" -Method GET -Headers $headers -ErrorAction Stop
    $json = $resp.Content | ConvertFrom-Json
    if ($resp.StatusCode -eq 200 -and $json.status -eq "success") {
        Write-Host "  PASSED (HTTP $($resp.StatusCode))" -ForegroundColor Green
        $passed++
    } else {
        Write-Host "  FAILED" -ForegroundColor Red
        $failed++
    }
}
catch {
    Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red
    $failed++
}

# 4. PATCH /learning/rule-suggestions/:id (cập nhật status — cần suggestionId thật)
# Bỏ qua nếu không có rule suggestion nào
$firstSuggestionId = $null
try {
    $listResp = Invoke-RestMethod -Uri "$baseURL/learning/rule-suggestions?limit=1" -Method GET -Headers $headers -ErrorAction Stop
    if ($listResp.data -and $listResp.data.items -and $listResp.data.items.Count -gt 0) {
        $firstSuggestionId = $listResp.data.items[0].suggestionId
    }
}
catch { }
if ($firstSuggestionId) {
    Write-Host "`n[4] PATCH /learning/rule-suggestions/$firstSuggestionId" -ForegroundColor Yellow
    $patchBody = @{ status = "reviewed"; reviewedBy = "test-script" } | ConvertTo-Json
    try {
        $resp = Invoke-WebRequest -Uri "$baseURL/learning/rule-suggestions/$firstSuggestionId" -Method PATCH -Headers $headers -Body $patchBody -ErrorAction Stop
        $json = $resp.Content | ConvertFrom-Json
        if ($resp.StatusCode -eq 200 -and $json.status -eq "success") {
            Write-Host "  PASSED (HTTP $($resp.StatusCode))" -ForegroundColor Green
            $passed++
        } else { Write-Host "  FAILED" -ForegroundColor Red; $failed++ }
    }
    catch { Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red; $failed++ }
} else {
    Write-Host "`n[4] PATCH /learning/rule-suggestions/:id - SKIP (khong co suggestion)" -ForegroundColor Gray
}

# 5. POST /learning/cases (tạo learning case mẫu)
Write-Host "`n[5] POST /learning/cases" -ForegroundColor Yellow
$caseBody = @{
    caseId = "test_phase3_$(Get-Date -Format 'yyyyMMddHHmmss')"
    caseType = "ads"
    caseCategory = "rule_review"
    domain = "ads"
    targetType = "campaign"
    targetId = "test_camp_001"
    goalCode = "pause_campaign"
    result = "success"
    sourceRefType = "manual"
    sourceRefId = "test"
} | ConvertTo-Json
try {
    $resp = Invoke-WebRequest -Uri "$baseURL/learning/cases" -Method POST -Headers $headers -Body $caseBody -ErrorAction Stop
    $json = $resp.Content | ConvertFrom-Json
    if ($resp.StatusCode -eq 201 -and $json.status -eq "success") {
        Write-Host "  PASSED (HTTP $($resp.StatusCode))" -ForegroundColor Green
        $passed++
    } else {
        Write-Host "  FAILED (status=$($json.status))" -ForegroundColor Red
        $failed++
    }
}
catch {
    Write-Host "  FAILED: $($_.Exception.Message)" -ForegroundColor Red
    $failed++
}

# Tổng kết
Write-Host "`n=== TONG KET Phase 3 ===" -ForegroundColor Cyan
Write-Host "PASSED: $passed" -ForegroundColor Green
Write-Host "FAILED: $failed" -ForegroundColor $(if ($failed -gt 0) { "Red" } else { "Green" })
if ($failed -eq 0) {
    Write-Host "`nPhase 3 test PASSED!" -ForegroundColor Green
} else {
    exit 1
}
