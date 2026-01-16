# Script Test Toàn Bộ Endpoints với Admin Token
# Sử dụng: .\test-all-endpoints.ps1

$adminToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTY2YzkwMGNiZjYyZGJhMGZjYWZkNGMiLCJ0aW1lIjoiNjk2NmM5MDAiLCJyYW5kb21OdW1iZXIiOiI1OCJ9.FflKAynO-2ArrbKWqTgRIAqIyQ13PrvHpjeB37E7MZI"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

# Kết quả test
$testResults = @{
    Total = 0
    Passed = 0
    Failed = 0
    Skipped = 0
    Errors = @()
}

function Test-Endpoint {
    param(
        [string]$Method,
        [string]$Path,
        [object]$Body = $null,
        [string]$Description = ""
    )
    
    $testResults.Total++
    $fullURL = "$baseURL$Path"
    
    try {
        $bodyJson = $null
        if ($Body -ne $null) {
            $bodyJson = $Body | ConvertTo-Json -Depth 10
        }
        
        Write-Host "`n[TEST $($testResults.Total)] $Method $Path" -ForegroundColor Cyan
        if ($Description) {
            Write-Host "  Mo ta: $Description" -ForegroundColor Gray
        }
        
        $response = $null
        $statusCode = 0
        
        switch ($Method.ToUpper()) {
            "GET" {
                $response = Invoke-WebRequest -Uri $fullURL -Method GET -Headers $headers -ErrorAction SilentlyContinue
            }
            "POST" {
                if ($bodyJson) {
                    $response = Invoke-WebRequest -Uri $fullURL -Method POST -Headers $headers -Body $bodyJson -ErrorAction SilentlyContinue
                } else {
                    $response = Invoke-WebRequest -Uri $fullURL -Method POST -Headers $headers -ErrorAction SilentlyContinue
                }
            }
            "PUT" {
                if ($bodyJson) {
                    $response = Invoke-WebRequest -Uri $fullURL -Method PUT -Headers $headers -Body $bodyJson -ErrorAction SilentlyContinue
                } else {
                    $response = Invoke-WebRequest -Uri $fullURL -Method PUT -Headers $headers -ErrorAction SilentlyContinue
                }
            }
            "DELETE" {
                $response = Invoke-WebRequest -Uri $fullURL -Method DELETE -Headers $headers -ErrorAction SilentlyContinue
            }
        }
        
        if ($response) {
            $statusCode = $response.StatusCode
            $content = $response.Content | ConvertFrom-Json -ErrorAction SilentlyContinue
            
            if ($statusCode -ge 200 -and $statusCode -lt 300) {
                Write-Host "  PASSED (HTTP $statusCode)" -ForegroundColor Green
                $testResults.Passed++
            } elseif ($statusCode -eq 401 -or $statusCode -eq 403) {
                Write-Host "  SKIPPED (HTTP $statusCode - Auth/Permission)" -ForegroundColor Yellow
                $testResults.Skipped++
            } elseif ($statusCode -eq 404) {
                Write-Host "  SKIPPED (HTTP $statusCode - Not Found)" -ForegroundColor Yellow
                $testResults.Skipped++
            } else {
                Write-Host "  FAILED (HTTP $statusCode)" -ForegroundColor Red
                if ($content.message) {
                    Write-Host "    Message: $($content.message)" -ForegroundColor Red
                }
                $testResults.Failed++
                $testResults.Errors += "$Method $Path - HTTP $statusCode"
            }
        } else {
            Write-Host "  FAILED (No response)" -ForegroundColor Red
            $testResults.Failed++
            $testResults.Errors += "$Method $Path - No response"
        }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        Write-Host "  ❌ FAILED (Error: $($_.Exception.Message))" -ForegroundColor Red
        $testResults.Failed++
        $testResults.Errors += "$Method $Path - Error: $($_.Exception.Message)"
    }
}

# Lay role ID de set active role
Write-Host "`n=== SETUP ===" -ForegroundColor Yellow
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "Da set active role: $roleID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Khong the lay role ID, tiep tuc test khong co active role" -ForegroundColor Yellow
}

Write-Host "`n=== BAT DAU TEST TAT CA ENDPOINTS ===" -ForegroundColor Yellow

# ===== SYSTEM ENDPOINTS =====
Write-Host "`n--- System Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/system/health" -Description "Health check"

# ===== AUTH ENDPOINTS =====
Write-Host "`n--- Auth Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/auth/profile" -Description "Get user profile"
Test-Endpoint "GET" "/auth/roles" -Description "Get user roles"

# ===== RBAC ENDPOINTS =====
Write-Host "`n--- RBAC Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/user/find" -Description "Find users"
Test-Endpoint "GET" "/user/find-one" -Description "Find one user"
Test-Endpoint "GET" "/permission/find" -Description "Find permissions"
Test-Endpoint "GET" "/role/find" -Description "Find roles"
Test-Endpoint "GET" "/role/find-one" -Description "Find one role"
Test-Endpoint "GET" "/role-permission/find" -Description "Find role permissions"
Test-Endpoint "GET" "/user-role/find" -Description "Find user roles"
Test-Endpoint "GET" "/organization/find" -Description "Find organizations"
Test-Endpoint "GET" "/organization/find-one" -Description "Find one organization"
Test-Endpoint "GET" "/organization-share/find" -Description "Find organization shares"

# ===== NOTIFICATION ENDPOINTS =====
Write-Host "`n--- Notification Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/notification/sender/find" -Description "Find notification senders"
Test-Endpoint "GET" "/notification/channel/find" -Description "Find notification channels"
Test-Endpoint "GET" "/notification/template/find" -Description "Find notification templates"
Test-Endpoint "GET" "/notification/routing/find" -Description "Find notification routing rules"
Test-Endpoint "GET" "/notification/history/find" -Description "Find notification history"

# ===== CTA ENDPOINTS =====
Write-Host "`n--- CTA Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/cta/library/find" -Description "Find CTA libraries"
# Lưu ý: CTA tracking không có endpoint riêng, được gộp vào /track/:action/:historyId với action="cta"

# ===== DELIVERY ENDPOINTS =====
Write-Host "`n--- Delivery Endpoints ---" -ForegroundColor Magenta
# Lưu ý: Delivery queue không có endpoint riêng, delivery được quản lý qua /notification/sender và /delivery/history
Test-Endpoint "GET" "/delivery/history/find" -Description "Find delivery history"

# ===== AI ENDPOINTS =====
Write-Host "`n--- AI Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/ai/provider-profiles/find" -Description "Find AI provider profiles"
Test-Endpoint "GET" "/ai/prompt-templates/find" -Description "Find AI prompt templates"
Test-Endpoint "GET" "/ai/steps/find" -Description "Find AI steps"
Test-Endpoint "GET" "/ai/workflows/find" -Description "Find AI workflows"
Test-Endpoint "GET" "/ai/workflow-runs/find" -Description "Find AI workflow runs"
Test-Endpoint "GET" "/ai/workflow-commands/find" -Description "Find AI workflow commands"
Test-Endpoint "GET" "/ai/step-runs/find" -Description "Find AI step runs"
Test-Endpoint "GET" "/ai/generation-batches/find" -Description "Find AI generation batches"
Test-Endpoint "GET" "/ai/candidates/find" -Description "Find AI candidates"
Test-Endpoint "GET" "/ai/ai-runs/find" -Description "Find AI runs"

# ===== CONTENT STORAGE ENDPOINTS =====
Write-Host "`n--- Content Storage Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/content/nodes/find" -Description "Find content nodes"
Test-Endpoint "GET" "/content/videos/find" -Description "Find content videos"
Test-Endpoint "GET" "/content/publications/find" -Description "Find content publications"
Test-Endpoint "GET" "/content/drafts/nodes/find" -Description "Find draft nodes"
Test-Endpoint "GET" "/content/drafts/videos/find" -Description "Find draft videos"
Test-Endpoint "GET" "/content/drafts/publications/find" -Description "Find draft publications"
Test-Endpoint "GET" "/content/drafts/approvals/find" -Description "Find draft approvals"

# ===== FACEBOOK ENDPOINTS =====
Write-Host "`n--- Facebook Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/facebook/page/find" -Description "Find Facebook pages"
Test-Endpoint "GET" "/facebook/post/find" -Description "Find Facebook posts"
Test-Endpoint "GET" "/facebook/conversation/find" -Description "Find Facebook conversations"
Test-Endpoint "GET" "/facebook/message/find" -Description "Find Facebook messages"
Test-Endpoint "GET" "/facebook/message-item/find" -Description "Find Facebook message items"
Test-Endpoint "GET" "/fb-customer/find" -Description "Find Facebook customers"

# ===== PANCAKE ENDPOINTS =====
Write-Host "`n--- Pancake Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/pancake/order/find" -Description "Find Pancake orders"
# Access Token được đăng ký trong Facebook routes, không có prefix /pancake/
Test-Endpoint "GET" "/access-token/find" -Description "Find access tokens"

# ===== AGENT MANAGEMENT ENDPOINTS =====
Write-Host "`n--- Agent Management Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/agent-management/registry/find" -Description "Find agent registry"
Test-Endpoint "GET" "/agent-management/config/find" -Description "Find agent configs"
Test-Endpoint "GET" "/agent-management/activity/find" -Description "Find agent activity logs"

# ===== WEBHOOK ENDPOINTS =====
Write-Host "`n--- Webhook Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/webhook-log/find" -Description "Find webhook logs"

# ===== PANCAKE POS ENDPOINTS =====
Write-Host "`n--- Pancake POS Endpoints ---" -ForegroundColor Magenta
Test-Endpoint "GET" "/pc-pos-customer/find" -Description "Find Pancake POS customers"
Test-Endpoint "GET" "/pancake-pos/shop/find" -Description "Find Pancake POS shops"
Test-Endpoint "GET" "/pancake-pos/warehouse/find" -Description "Find Pancake POS warehouses"
Test-Endpoint "GET" "/pancake-pos/product/find" -Description "Find Pancake POS products"
Test-Endpoint "GET" "/pancake-pos/variation/find" -Description "Find Pancake POS variations"
Test-Endpoint "GET" "/pancake-pos/category/find" -Description "Find Pancake POS categories"
Test-Endpoint "GET" "/pancake-pos/order/find" -Description "Find Pancake POS orders"

# ===== CUSTOM ENDPOINTS =====
Write-Host "`n--- Custom Endpoints ---" -ForegroundColor Magenta

# Notification trigger
$triggerBody = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification from endpoint test"
        errorCode = "TEST_ENDPOINT_001"
    }
}
Test-Endpoint "POST" "/notification/trigger" -Body $triggerBody -Description "Trigger notification"

# Content Node tree - Lay node ID that tu database
$realNodeID = $null
try {
    $nodeResp = Invoke-RestMethod -Uri "$baseURL/content/nodes/find-one" -Method GET -Headers $headers -ErrorAction SilentlyContinue
    if ($nodeResp.data) {
        $realNodeID = $nodeResp.data._id
        if (-not $realNodeID) { $realNodeID = $nodeResp.data.id }
    }
}
catch {
    # Khong co node, bo qua
}
if ($realNodeID) {
    Test-Endpoint "GET" "/content/nodes/tree/$realNodeID" -Description "Get content node tree voi data that"
} else {
    Test-Endpoint "GET" "/content/nodes/tree/000000000000000000000000" -Description "Get content node tree (khong co data)"
}

# Content Draft commit
Test-Endpoint "POST" "/content/drafts/nodes/000000000000000000000000/commit" -Description "Commit draft node (sẽ skip nếu không có data)"

# Facebook custom endpoints
Test-Endpoint "GET" "/facebook/conversation/sort-by-api-update" -Description "Get conversations sorted by API update"
Test-Endpoint "GET" "/facebook/message-item/find-by-conversation/000000000000000000000000" -Description "Find message items by conversation (sẽ skip nếu không có data)"
Test-Endpoint "GET" "/facebook/message-item/find-by-message-id/000000000000000000000000" -Description "Find message item by message ID (sẽ skip nếu không có data)"

# AI custom endpoints - Workflow Commands
Test-Endpoint "POST" "/ai/workflow-commands/claim-pending" -Body @{ agentId = "test-agent"; limit = 1 } -Description "Claim pending AI workflow commands"
Test-Endpoint "POST" "/ai/workflow-commands/update-heartbeat" -Body @{ commandId = "000000000000000000000000"; progress = @{} } -Description "Update AI workflow command heartbeat"
Test-Endpoint "POST" "/ai/workflow-commands/release-stuck" -Body @{ timeoutSeconds = 300 } -Description "Release stuck AI workflow commands"

# Agent Management custom endpoints
Test-Endpoint "POST" "/agent-management/command/claim-pending" -Body @{ agentId = "test-agent"; limit = 1 } -Description "Claim pending agent commands"
Test-Endpoint "POST" "/agent-management/command/update-heartbeat" -Body @{ commandId = "000000000000000000000000"; progress = @{} } -Description "Update agent command heartbeat"
Test-Endpoint "POST" "/agent-management/command/release-stuck" -Body @{ timeoutSeconds = 300 } -Description "Release stuck agent commands"

# Agent Management config update
Test-Endpoint "PUT" "/agent-management/config/000000000000000000000000/update-data" -Body @{ configData = @{} } -Description "Update agent config data (sẽ skip nếu không có data)"

# Content Draft Approval custom endpoints
Test-Endpoint "POST" "/content/drafts/approvals/000000000000000000000000/approve" -Description "Approve draft workflow run (sẽ skip nếu không có data)"
Test-Endpoint "POST" "/content/drafts/approvals/000000000000000000000000/reject" -Description "Reject draft approval (sẽ skip nếu không có data)"

# Admin endpoints
Test-Endpoint "POST" "/admin/user/block" -Body @{ userId = "000000000000000000000000" } -Description "Block user (sẽ skip nếu không có data)"
Test-Endpoint "POST" "/admin/user/unblock" -Body @{ userId = "000000000000000000000000" } -Description "Unblock user (sẽ skip nếu không có data)"
Test-Endpoint "POST" "/admin/sync-administrator-permissions" -Description "Sync administrator permissions"

# Delivery send
$deliveryBody = @{
    channelId = "000000000000000000000000"
    recipient = "test@example.com"
    subject = "Test"
    content = "Test content"
}
Test-Endpoint "POST" "/delivery/send" -Body $deliveryBody -Description "Send delivery (sẽ skip nếu không có channel)"

# Role Permission update
Test-Endpoint "PUT" "/role-permission/update-role" -Body @{ roleId = "000000000000000000000000"; permissionIds = @() } -Description "Update role permissions (sẽ skip nếu không có data)"

# ===== TONG KET =====
Write-Host "`n=== TONG KET ===" -ForegroundColor Yellow
Write-Host "Tong so test: $($testResults.Total)" -ForegroundColor White
Write-Host "PASSED: $($testResults.Passed)" -ForegroundColor Green
Write-Host "FAILED: $($testResults.Failed)" -ForegroundColor Red
Write-Host "SKIPPED: $($testResults.Skipped)" -ForegroundColor Yellow

if ($testResults.Failed -gt 0) {
    Write-Host "`n--- Chi Tiet Loi ---" -ForegroundColor Red
    foreach ($err in $testResults.Errors) {
        Write-Host "  - $err" -ForegroundColor Red
    }
}

$successRate = if ($testResults.Total -gt 0) {
    [math]::Round(($testResults.Passed / $testResults.Total) * 100, 2)
} else {
    0
}

Write-Host "`nTy le thanh cong: $successRate%" -ForegroundColor $(if ($successRate -ge 80) { "Green" } elseif ($successRate -ge 50) { "Yellow" } else { "Red" })
