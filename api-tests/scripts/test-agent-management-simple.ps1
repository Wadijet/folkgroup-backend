# Script test đơn giản cho Agent Management System
# Sử dụng: .\api-tests\scripts\test-agent-management-simple.ps1

param(
    [string]$BearerToken = "",
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Nếu không có token, yêu cầu nhập
if ([string]::IsNullOrEmpty($BearerToken)) {
    Write-Host "Nhập Bearer Token của admin user:" -ForegroundColor Yellow
    $BearerToken = Read-Host -AsSecureString
    $BearerToken = [Runtime.InteropServices.Marshal]::PtrToStringAuto([Runtime.InteropServices.Marshal]::SecureStringToBSTR($BearerToken))
}

if ([string]::IsNullOrEmpty($BearerToken)) {
    Write-Host "❌ Bearer Token không được để trống!" -ForegroundColor Red
    exit 1
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TEST AGENT MANAGEMENT SYSTEM" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "Base URL: $BaseURL" -ForegroundColor Gray
Write-Host ""

# Headers
$headers = @{
    "Content-Type" = "application/json"
    "Authorization" = "Bearer $BearerToken"
}

# Test 1: Lấy danh sách Agent Registry
Write-Host "[1] Test: Lấy danh sách Agent Registry..." -ForegroundColor Yellow
try {
    $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/find" -Method GET -Headers $headers
    Write-Host "  ✅ Thành công - Tìm thấy $($response.data.Count) registry" -ForegroundColor Green
    if ($response.data.Count -gt 0) {
        $registryId = $response.data[0].id
        Write-Host "     Registry ID đầu tiên: $registryId" -ForegroundColor Cyan
    }
}
catch {
    Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""

# Test 2: Tạo Agent Registry mới
Write-Host "[2] Test: Tạo Agent Registry mới..." -ForegroundColor Yellow
$testAgentId = "test-agent-$(Get-Date -Format 'yyyyMMddHHmmss')"
$createPayload = @{
    agentId = $testAgentId
    name = "Test Agent"
    description = "Agent test tự động"
    botVersion = "1.0.0"
} | ConvertTo-Json

try {
    $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/insert-one" -Method POST -Headers $headers -Body $createPayload
    Write-Host "  ✅ Thành công - Registry ID: $($response.data.id)" -ForegroundColor Green
    $registryId = $response.data.id
}
catch {
    Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
    $registryId = $null
}

Write-Host ""

# Test 3: Lấy Agent Registry theo ID
if ($registryId) {
    Write-Host "[3] Test: Lấy Agent Registry theo ID..." -ForegroundColor Yellow
    try {
        $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/registry/find-by-id/$registryId" -Method GET -Headers $headers
        Write-Host "  ✅ Thành công - Agent ID: $($response.data.agentId)" -ForegroundColor Green
    }
    catch {
        Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
}

# Test 4: Tạo Agent Config
if ($registryId) {
    Write-Host "[4] Test: Tạo Agent Config..." -ForegroundColor Yellow
    $configPayload = @{
        agentId = $registryId
        version = "1.0.0"
        configData = @{
            pollingInterval = 30
            maxRetries = 3
        }
        description = "Config test"
    } | ConvertTo-Json -Depth 10
    
    try {
        $response = Invoke-RestMethod -Uri "$BaseURL/agent-management/config/insert-one" -Method POST -Headers $headers -Body $configPayload
        Write-Host "  ✅ Thành công - Config ID: $($response.data.id)" -ForegroundColor Green
        $configId = $response.data.id
    }
    catch {
        Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
}

# Test 5: Tạo Agent Command
if ($registryId) {
    Write-Host "[5] Test: Tạo Agent Command..." -ForegroundColor Yellow
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
        Write-Host "  ✅ Thành công - Command ID: $($response.data.id)" -ForegroundColor Green
        $commandId = $response.data.id
    }
    catch {
        Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
    }
    Write-Host ""
}

# Test 6: Enhanced Check-In
Write-Host "[6] Test: Enhanced Check-In..." -ForegroundColor Yellow
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
    Write-Host "  ✅ Thành công - Check-in hoàn tất" -ForegroundColor Green
    if ($response.data.commands) {
        Write-Host "     Nhận được $($response.data.commands.Count) commands" -ForegroundColor Cyan
    }
}
catch {
    Write-Host "  ❌ Lỗi: $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
