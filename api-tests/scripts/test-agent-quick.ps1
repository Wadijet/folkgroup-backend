# Quick test script for Agent Management
param(
    [string]$Token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVkNGEzOGNiZjYyZGJhMGZjMTQ4MDYiLCJ0aW1lIjoiNjk1ZDRhMzgiLCJyYW5kb21OdW1iZXIiOiI1NCJ9.Mx_XPGJSl3lLYsPlT_bLaAT_5HhR8PL7hA54IjJe-kA",
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$headers = @{
    "Content-Type" = "application/json"
    "Authorization" = "Bearer $Token"
}

Write-Host "Testing Agent Management System..." -ForegroundColor Cyan
Write-Host ""

# Get user roles first to get roleId for X-Active-Role-ID header
Write-Host "[0] Getting user roles..." -ForegroundColor Yellow
$roleId = $null
try {
    $rolesResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($rolesResponse.data -and $rolesResponse.data.Count -gt 0) {
        $roleId = $rolesResponse.data[0].roleId
        Write-Host "SUCCESS - Using Role ID: $roleId" -ForegroundColor Green
        $headers["X-Active-Role-ID"] = $roleId
    }
    else {
        Write-Host "WARNING - No roles found for user" -ForegroundColor Yellow
    }
}
catch {
    Write-Host "ERROR getting roles: $($_.Exception.Message)" -ForegroundColor Red
    Write-Host "Continuing without role ID (may fail)" -ForegroundColor Yellow
}
Write-Host ""

# Test 1: Get Registry List
Write-Host "[1] GET /agent-management/registry/find" -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/find" -Method GET -Headers $headers
    Write-Host "SUCCESS - Found $($response.data.Count) registries" -ForegroundColor Green
    if ($response.data.Count -gt 0) {
        Write-Host "First Registry: $($response.data[0].agentId)" -ForegroundColor Gray
    }
}
catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}
Write-Host ""

# Test 2: Create Registry
Write-Host "[2] POST /agent-management/registry/insert-one" -ForegroundColor Yellow
$testAgentId = "test-agent-$(Get-Date -Format 'yyyyMMddHHmmss')"
$createPayload = @{
    agentId = $testAgentId
    name = "Test Agent"
    description = "Test agent"
    botVersion = "1.0.0"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/insert-one" -Method POST -Headers $headers -Body $createPayload
    Write-Host "SUCCESS - Created Registry ID: $($response.data.id)" -ForegroundColor Green
    $registryId = $response.data.id
}
catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
    $registryId = $null
}
Write-Host ""

# Test 3: Get Registry by ID
if ($registryId) {
    Write-Host "[3] GET /agent-management/registry/find-by-id/$registryId" -ForegroundColor Yellow
    try {
        $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/find-by-id/$registryId" -Method GET -Headers $headers
        Write-Host "SUCCESS - Agent ID: $($response.data.agentId)" -ForegroundColor Green
    }
    catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
}

# Test 4: Create Config
if ($registryId) {
    Write-Host "[4] POST /agent-management/config/insert-one" -ForegroundColor Yellow
    $configPayload = @{
        agentId = $registryId
        version = "1.0.0"
        configData = @{
            pollingInterval = 30
            maxRetries = 3
        }
        description = "Test config"
    } | ConvertTo-Json -Depth 10
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/config/insert-one" -Method POST -Headers $headers -Body $configPayload
        Write-Host "SUCCESS - Created Config ID: $($response.data.id)" -ForegroundColor Green
    }
    catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails.Message) {
            Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
    }
    Write-Host ""
}

# Test 5: Create Command
if ($registryId) {
    Write-Host "[5] POST /agent-management/command/insert-one" -ForegroundColor Yellow
    $commandPayload = @{
        agentId = $registryId
        type = "reload_config"
        target = "bot"
        params = @{
            force = $true
        }
    } | ConvertTo-Json -Depth 10
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/command/insert-one" -Method POST -Headers $headers -Body $commandPayload
        Write-Host "SUCCESS - Created Command ID: $($response.data.id)" -ForegroundColor Green
    }
    catch {
        Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
        if ($_.ErrorDetails.Message) {
            Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
        }
    }
    Write-Host ""
}

# Test 6: Enhanced Check-In
Write-Host "[6] POST /agent-management/check-in" -ForegroundColor Yellow
$checkInPayload = @{
    agentId = $testAgentId
    status = "online"
    healthStatus = "healthy"
    systemInfo = @{
        os = "linux"
        arch = "amd64"
    }
    metrics = @{
        messagesProcessed = 1000
    }
    configVersion = "1.0.0"
    configHash = "abc123"
} | ConvertTo-Json -Depth 10

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/check-in" -Method POST -Headers $headers -Body $checkInPayload
    Write-Host "SUCCESS - Check-in completed" -ForegroundColor Green
    if ($response.data.commands) {
        Write-Host "Received $($response.data.commands.Count) commands" -ForegroundColor Cyan
    }
}
catch {
    Write-Host "ERROR: $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Details: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}
Write-Host ""

Write-Host "Test completed!" -ForegroundColor Cyan
