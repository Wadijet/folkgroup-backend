# Script test hệ thống Agent Management (Bot Management)
# Sử dụng: .\api-tests\scripts\test-agent-management.ps1 -BearerToken "your_token_here" -BaseURL "http://localhost:8080/api/v1"

param(
    [Parameter(Mandatory=$true)]
    [string]$BearerToken,
    
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TEST AGENT MANAGEMENT SYSTEM" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Headers cho tất cả requests
$headers = @{
    "Content-Type" = "application/json"
    "Authorization" = "Bearer $BearerToken"
}

# Helper function để gọi API
function Invoke-APIRequest {
    param(
        [string]$Method,
        [string]$Endpoint,
        [object]$Body = $null
    )
    
    $uri = "$BaseURL$Endpoint"
    $bodyJson = $null
    
    if ($Body) {
        $bodyJson = $Body | ConvertTo-Json -Depth 10 -Compress
    }
    
    try {
        $params = @{
            Uri = $uri
            Method = $Method
            Headers = $headers
            ContentType = "application/json"
            ErrorAction = "Stop"
        }
        
        if ($bodyJson) {
            $params.Body = $bodyJson
        }
        
        $response = Invoke-RestMethod @params
        return @{
            Success = $true
            StatusCode = 200
            Data = $response
        }
    }
    catch {
        $statusCode = $_.Exception.Response.StatusCode.value__
        $errorBody = $_.ErrorDetails.Message
        return @{
            Success = $false
            StatusCode = $statusCode
            Error = $errorBody
        }
    }
}

# Helper function để hiển thị kết quả
function Show-Result {
    param(
        [string]$TestName,
        [object]$Result
    )
    
    if ($Result.Success) {
        Write-Host "  ✅ $TestName" -ForegroundColor Green
        if ($Result.Data.data) {
            Write-Host "     Response: $(($Result.Data.data | ConvertTo-Json -Depth 2 -Compress))" -ForegroundColor Gray
        }
    }
    else {
        Write-Host "  ❌ $TestName" -ForegroundColor Red
        Write-Host "     Status: $($Result.StatusCode)" -ForegroundColor Yellow
        Write-Host "     Error: $($Result.Error)" -ForegroundColor Yellow
    }
}

# ============================================
# TEST 1: AGENT REGISTRY CRUD
# ============================================
Write-Host "[1/6] Test Agent Registry CRUD..." -ForegroundColor Yellow

# 1.1. Tạo Agent Registry
$testAgentId = "test-agent-$(Get-Date -Format 'yyyyMMddHHmmss')"
$createRegistryPayload = @{
    agentId = $testAgentId
    name = "Test Agent"
    description = "Agent test tự động"
    botVersion = "1.0.0"
}

$result = Invoke-APIRequest -Method "POST" -Endpoint "/agent-management/registry/insert-one" -Body $createRegistryPayload
Show-Result "Tạo Agent Registry" $result

if ($result.Success -and $result.Data.data.id) {
    $registryId = $result.Data.data.id
    Write-Host "     Registry ID: $registryId" -ForegroundColor Cyan
    
    # 1.2. Lấy Agent Registry theo ID
    $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/registry/find-by-id/$registryId"
    Show-Result "Lấy Agent Registry theo ID" $result
    
    # 1.3. Cập nhật Agent Registry
    $updateRegistryPayload = @{
        name = "Test Agent Updated"
        status = "online"
        healthStatus = "healthy"
    }
    $result = Invoke-APIRequest -Method "PUT" -Endpoint "/agent-management/registry/update-by-id/$registryId" -Body $updateRegistryPayload
    Show-Result "Cập nhật Agent Registry" $result
    
    # 1.4. Lấy danh sách Agent Registry
    $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/registry/find"
    Show-Result "Lấy danh sách Agent Registry" $result
}
else {
    Write-Host "  ⚠️ Không thể tạo Agent Registry, bỏ qua các test tiếp theo" -ForegroundColor Yellow
    $registryId = $null
}

Write-Host ""

# ============================================
# TEST 2: AGENT CONFIG CRUD
# ============================================
Write-Host "[2/6] Test Agent Config CRUD..." -ForegroundColor Yellow

if ($registryId) {
    # 2.1. Tạo Agent Config
    $createConfigPayload = @{
        agentId = $registryId
        version = "1.0.0"
        configData = @{
            pollingInterval = 30
            maxRetries = 3
            timeout = 60
            jobs = @(
                @{
                    name = "job1"
                    enabled = $true
                    schedule = "0 */5 * * * *"
                }
            )
        }
        description = "Config test tự động"
        changeLog = "Tạo config mới cho test"
    }
    
    $result = Invoke-APIRequest -Method "POST" -Endpoint "/agent-management/config/insert-one" -Body $createConfigPayload
    Show-Result "Tạo Agent Config" $result
    
    if ($result.Success -and $result.Data.data.id) {
        $configId = $result.Data.data.id
        Write-Host "     Config ID: $configId" -ForegroundColor Cyan
        
        # 2.2. Lấy Agent Config theo ID
        $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/config/find-by-id/$configId"
        Show-Result "Lấy Agent Config theo ID" $result
        
        # 2.3. Cập nhật Agent Config
        $updateConfigPayload = @{
            description = "Config đã được cập nhật"
            isActive = $true
        }
        $result = Invoke-APIRequest -Method "PUT" -Endpoint "/agent-management/config/update-by-id/$configId" -Body $updateConfigPayload
        Show-Result "Cập nhật Agent Config" $result
        
        # 2.4. Lấy danh sách Agent Config
        $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/config/find"
        Show-Result "Lấy danh sách Agent Config" $result
    }
}
else {
    Write-Host "  ⚠️ Bỏ qua test Agent Config (cần Registry ID)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================
# TEST 3: AGENT COMMAND CRUD
# ============================================
Write-Host "[3/6] Test Agent Command CRUD..." -ForegroundColor Yellow

if ($registryId) {
    # 3.1. Tạo Agent Command
    $createCommandPayload = @{
        agentId = $registryId
        type = "reload_config"
        target = "bot"
        params = @{
            force = $true
        }
    }
    
    $result = Invoke-APIRequest -Method "POST" -Endpoint "/agent-management/command/insert-one" -Body $createCommandPayload
    Show-Result "Tạo Agent Command" $result
    
    if ($result.Success -and $result.Data.data.id) {
        $commandId = $result.Data.data.id
        Write-Host "     Command ID: $commandId" -ForegroundColor Cyan
        
        # 3.2. Lấy Agent Command theo ID
        $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/command/find-by-id/$commandId"
        Show-Result "Lấy Agent Command theo ID" $result
        
        # 3.3. Cập nhật Agent Command (bot báo cáo kết quả)
        $updateCommandPayload = @{
            status = "completed"
            result = @{
                message = "Config reloaded successfully"
                timestamp = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
            }
        }
        $result = Invoke-APIRequest -Method "PUT" -Endpoint "/agent-management/command/update-by-id/$commandId" -Body $updateCommandPayload
        Show-Result "Cập nhật Agent Command" $result
        
        # 3.4. Lấy danh sách Agent Command
        $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/command/find"
        Show-Result "Lấy danh sách Agent Command" $result
    }
}
else {
    Write-Host "  ⚠️ Bỏ qua test Agent Command (cần Registry ID)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================
# TEST 4: AGENT STATUS (READ-ONLY)
# ============================================
Write-Host "[4/6] Test Agent Status (Read-Only)..." -ForegroundColor Yellow

if ($registryId) {
    # 4.1. Lấy danh sách Agent Status
    $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/status/find"
    Show-Result "Lấy danh sách Agent Status" $result
    
    # 4.2. Lấy Agent Status theo ID (nếu có)
    $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/status/find-by-id/$registryId"
    Show-Result "Lấy Agent Status theo ID" $result
}
else {
    Write-Host "  ⚠️ Bỏ qua test Agent Status (cần Registry ID)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================
# TEST 5: AGENT ACTIVITY LOG (READ-ONLY)
# ============================================
Write-Host "[5/6] Test Agent Activity Log (Read-Only)..." -ForegroundColor Yellow

if ($registryId) {
    # 5.1. Lấy danh sách Agent Activity Log
    $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/activity/find"
    Show-Result "Lấy danh sách Agent Activity Log" $result
    
    # 5.2. Lấy Agent Activity Log theo ID (nếu có)
    if ($result.Success -and $result.Data.data -and $result.Data.data.Count -gt 0) {
        $firstActivityId = $result.Data.data[0].id
        $result = Invoke-APIRequest -Method "GET" -Endpoint "/agent-management/activity/find-by-id/$firstActivityId"
        Show-Result "Lấy Agent Activity Log theo ID" $result
    }
}
else {
    Write-Host "  ⚠️ Bỏ qua test Agent Activity Log (cần Registry ID)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================
# TEST 6: ENHANCED CHECK-IN
# ============================================
Write-Host "[6/6] Test Enhanced Check-In..." -ForegroundColor Yellow

if ($testAgentId) {
    # 6.1. Enhanced Check-In từ bot
    $checkInPayload = @{
        agentId = $testAgentId
        status = "online"
        healthStatus = "healthy"
        systemInfo = @{
            os = "linux"
            arch = "amd64"
            goVersion = "1.21.0"
            uptime = 3600
            cpu = @{
                usage = 25.5
                cores = 4
            }
            memory = @{
                total = 8192
                used = 4096
                free = 4096
            }
            disk = @{
                total = 100000
                used = 50000
                free = 50000
            }
        }
        metrics = @{
            messagesProcessed = 1000
            errors = 5
            avgResponseTime = 150
        }
        jobStatus = @(
            @{
                name = "job1"
                status = "running"
                lastRun = (Get-Date).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
                nextRun = (Get-Date).AddMinutes(5).ToUniversalTime().ToString("yyyy-MM-ddTHH:mm:ssZ")
            }
        )
        configVersion = "1.0.0"
        configHash = "abc123def456"
    }
    
    $result = Invoke-APIRequest -Method "POST" -Endpoint "/agent-management/check-in" -Body $checkInPayload
    Show-Result "Enhanced Check-In" $result
    
    if ($result.Success) {
        Write-Host "     Check-in thành công, server có thể trả về commands và config updates" -ForegroundColor Cyan
        if ($result.Data.data.commands) {
            Write-Host "     Commands: $(($result.Data.data.commands | ConvertTo-Json -Compress))" -ForegroundColor Gray
        }
        if ($result.Data.data.configUpdates) {
            Write-Host "     Config Updates: $(($result.Data.data.configUpdates | ConvertTo-Json -Compress))" -ForegroundColor Gray
        }
    }
}
else {
    Write-Host "  ⚠️ Bỏ qua test Enhanced Check-In (cần Agent ID)" -ForegroundColor Yellow
}

Write-Host ""

# ============================================
# TỔNG KẾT
# ============================================
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  HOÀN TẤT TEST" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Lưu ý:" -ForegroundColor Yellow
Write-Host "  - Các test đã được thực hiện với Bearer Token của admin user" -ForegroundColor Gray
Write-Host "  - Nếu có lỗi permission, kiểm tra lại quyền của user" -ForegroundColor Gray
Write-Host "  - Agent Registry ID: $registryId" -ForegroundColor Gray
Write-Host "  - Test Agent ID: $testAgentId" -ForegroundColor Gray
Write-Host ""
