# Script Test Endpoints với Data Thật
# Sử dụng: .\test-with-real-data.ps1

$adminToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTY2YzkwMGNiZjYyZGJhMGZjYWZkNGMiLCJ0aW1lIjoiNjk2NmM5MDAiLCJyYW5kb21OdW1iZXIiOiI1OCJ9.FflKAynO-2ArrbKWqTgRIAqIyQ13PrvHpjeB37E7MZI"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

# Doc test data
$testData = @{}
if (Test-Path "test-data.json") {
    $testData = Get-Content "test-data.json" | ConvertFrom-Json
}

# Lay role ID
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
    }
}
catch {
    Write-Host "Khong the lay role ID" -ForegroundColor Red
}

# Lay organization ID
try {
    $orgResp = Invoke-RestMethod -Uri "$baseURL/organization/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($orgResp.data) {
        $orgID = $orgResp.data.organizationId
        if (-not $orgID) { $orgID = $orgResp.data._id }
    }
}
catch {
    Write-Host "Khong the lay organization ID" -ForegroundColor Red
}

# Lay user ID
try {
    $userResp = Invoke-RestMethod -Uri "$baseURL/user/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($userResp.data) {
        $userID = $userResp.data.userId
        if (-not $userID) { $userID = $userResp.data._id }
    }
}
catch {
    Write-Host "Khong the lay user ID" -ForegroundColor Red
}

# Lay role ID de test update permissions
try {
    $roleResp2 = Invoke-RestMethod -Uri "$baseURL/role/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp2.data) {
        $testRoleID = $roleResp2.data.roleId
        if (-not $testRoleID) { $testRoleID = $roleResp2.data._id }
    }
}
catch {
    Write-Host "Khong the lay role ID" -ForegroundColor Red
}

# Lay channel ID
try {
    $channelResp = Invoke-RestMethod -Uri "$baseURL/notification/channel/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($channelResp.data) {
        $channelID = $channelResp.data._id
        if (-not $channelID) { $channelID = $channelResp.data.id }
    }
}
catch {
    Write-Host "Khong the lay channel ID" -ForegroundColor Yellow
}

# Su dung nodeID tu test-data hoac lay tu database
$nodeID = $testData.nodeID
if (-not $nodeID) {
    try {
        $nodeResp = Invoke-RestMethod -Uri "$baseURL/content/nodes/find-one" -Method GET -Headers $headers -ErrorAction Stop
        if ($nodeResp.data) {
            $nodeID = $nodeResp.data._id
            if (-not $nodeID) { $nodeID = $nodeResp.data.id }
        }
    }
    catch {
        Write-Host "Khong the lay node ID" -ForegroundColor Yellow
    }
}

Write-Host "`n=== TEST VOI DATA THAT ===" -ForegroundColor Yellow
Write-Host "Node ID: $nodeID" -ForegroundColor Cyan
Write-Host "User ID: $userID" -ForegroundColor Cyan
Write-Host "Role ID: $testRoleID" -ForegroundColor Cyan
Write-Host "Channel ID: $channelID" -ForegroundColor Cyan

$testResults = @{
    Total = 0
    Passed = 0
    Failed = 0
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
        Write-Host "  FAILED (Error: $($_.Exception.Message))" -ForegroundColor Red
        $testResults.Failed++
        $testResults.Errors += "$Method $Path - Error: $($_.Exception.Message)"
    }
}

# Test voi data that
if ($nodeID) {
    Test-Endpoint "GET" "/content/nodes/tree/$nodeID" -Description "Get content node tree voi data that"
}

# Test update heartbeat voi command ID that (neu co)
if ($testData.commandID) {
    $heartbeatBody = @{
        commandId = $testData.commandID
        progress = @{
            step = "testing"
            percentage = 50
        }
    }
    Test-Endpoint "POST" "/ai/workflow-commands/update-heartbeat?agentId=test-agent" -Body $heartbeatBody -Description "Update heartbeat voi command ID that"
}

# Test delivery send voi channel ID that
if ($channelID) {
    $deliveryBody = @{
        channelId = $channelID
        recipient = "test@example.com"
        subject = "Test Delivery"
        content = "Test content"
    }
    Test-Endpoint "POST" "/delivery/send" -Body $deliveryBody -Description "Send delivery voi channel ID that"
}

# Test block/unblock user voi user ID that
if ($userID) {
    $blockBody = @{ userId = $userID }
    Test-Endpoint "POST" "/admin/user/block" -Body $blockBody -Description "Block user voi user ID that"
    
    $unblockBody = @{ userId = $userID }
    Test-Endpoint "POST" "/admin/user/unblock" -Body $unblockBody -Description "Unblock user voi user ID that"
}

# Test update role permissions voi role ID that
if ($testRoleID) {
    $updateRoleBody = @{
        roleId = $testRoleID
        permissionIds = @()
    }
    Test-Endpoint "PUT" "/role-permission/update-role" -Body $updateRoleBody -Description "Update role permissions voi role ID that"
}

# Tong ket
Write-Host "`n=== TONG KET ===" -ForegroundColor Yellow
Write-Host "Tong so test: $($testResults.Total)" -ForegroundColor White
Write-Host "PASSED: $($testResults.Passed)" -ForegroundColor Green
Write-Host "FAILED: $($testResults.Failed)" -ForegroundColor Red

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
