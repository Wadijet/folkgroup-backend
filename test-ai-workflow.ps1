# Script test hệ thống AI Workflow
# Sử dụng bearer token của admin

$baseUrl = "http://localhost:8080"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVmN2IzOGNiZjYyZGJhMGZiMDk0Y2IiLCJ0aW1lIjoiNjk2NDU4ZGEiLCJyYW5kb21OdW1iZXIiOiI3NyJ9.GumBzdYurrNOB-hVVjCbSYp0Na8E7hdMQg8XpLO6g6k"
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TEST HỆ THỐNG AI WORKFLOW" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Hàm helper để gọi API
function Invoke-ApiRequest {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body = $null,
        [string]$ActiveRoleID = $null
    )
    
    $requestHeaders = $headers.Clone()
    if ($ActiveRoleID) {
        $requestHeaders["X-Active-Role-ID"] = $ActiveRoleID
    }
    
    $params = @{
        Method = $Method
        Uri = $Url
        Headers = $requestHeaders
    }
    
    if ($Body) {
        $params.Body = ($Body | ConvertTo-Json -Depth 10)
    }
    
    try {
        $response = Invoke-RestMethod @params
        return @{
            Success = $true
            Data = $response
            Error = $null
        }
    } catch {
        $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        return @{
            Success = $false
            Data = $null
            Error = if ($errorResponse) { $errorResponse } else { $_.Exception.Message }
        }
    }
}

# Bước 0: Lấy role ID của user
Write-Host "[BƯỚC 0] Lấy danh sách roles của user..." -ForegroundColor Yellow
$rolesResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/auth/roles"
if ($rolesResult.Success -and $rolesResult.Data.data.Count -gt 0) {
    $firstRole = $rolesResult.Data.data[0]
    $activeRoleID = $firstRole.roleId
    Write-Host "✓ Đã lấy role ID: $activeRoleID" -ForegroundColor Green
    Write-Host "  Role Name: $($firstRole.roleName)" -ForegroundColor Gray
    Write-Host "  Organization: $($firstRole.organizationName)" -ForegroundColor Gray
} else {
    Write-Host "✗ Không thể lấy roles của user" -ForegroundColor Red
    if ($rolesResult.Error) {
        Write-Host "  Lỗi: $($rolesResult.Error.message)" -ForegroundColor Red
    }
    exit 1
}
Write-Host ""

# Test 1: Lấy danh sách workflows (GET /api/v1/ai/workflows/find)
Write-Host "[TEST 1] Lấy danh sách workflows..." -ForegroundColor Yellow
$result = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find" -ActiveRoleID $activeRoleID
if ($result.Success) {
    Write-Host "✓ Thành công! Số lượng workflows: $($result.Data.data.Count)" -ForegroundColor Green
    if ($result.Data.data.Count -gt 0) {
        Write-Host "  Workflows hiện có:" -ForegroundColor Gray
        $result.Data.data | ForEach-Object {
            Write-Host "    - ID: $($_.id), Name: $($_.name), Status: $($_.status)" -ForegroundColor Gray
        }
    }
} else {
    Write-Host "✗ Lỗi: $($result.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Test 2: Kiểm tra có steps nào không (cần để tạo workflow)
Write-Host "[TEST 2] Kiểm tra danh sách steps..." -ForegroundColor Yellow
$stepsResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/steps/find" -ActiveRoleID $activeRoleID
if ($stepsResult.Success) {
    Write-Host "✓ Thành công! Số lượng steps: $($stepsResult.Data.data.Count)" -ForegroundColor Green
    $steps = $stepsResult.Data.data
    if ($steps.Count -eq 0) {
        Write-Host "  ⚠ Không có steps nào. Cần tạo step trước khi tạo workflow." -ForegroundColor Yellow
        Write-Host ""
        
        # Tạo một step mẫu
        Write-Host "[TEST 2.1] Tạo step mẫu..." -ForegroundColor Yellow
        $stepData = @{
            name = "Test Step - Generate Content"
            description = "Step test để generate content"
            type = "GENERATE"
            inputSchema = @{
                type = "object"
                properties = @{
                    content = @{
                        type = "string"
                        description = "Nội dung cần generate"
                    }
                }
            }
            outputSchema = @{
                type = "object"
                properties = @{
                    result = @{
                        type = "string"
                    }
                }
            }
            status = "active"
        }
        
        $createStepResult = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $stepData -ActiveRoleID $activeRoleID
        if ($createStepResult.Success) {
            $stepId = $createStepResult.Data.data.id
            Write-Host "✓ Đã tạo step với ID: $stepId" -ForegroundColor Green
            $steps = @($createStepResult.Data.data)
        } else {
            Write-Host "✗ Lỗi khi tạo step: $($createStepResult.Error.message)" -ForegroundColor Red
            Write-Host "  Không thể tiếp tục test workflow vì cần stepId." -ForegroundColor Red
            exit 1
        }
    } else {
        Write-Host "  Steps hiện có:" -ForegroundColor Gray
        $steps | ForEach-Object {
            Write-Host "    - ID: $($_.id), Name: $($_.name), Type: $($_.type)" -ForegroundColor Gray
        }
    }
    $firstStepId = $steps[0].id
} else {
    Write-Host "✗ Lỗi: $($stepsResult.Error.message)" -ForegroundColor Red
    Write-Host "  Không thể tiếp tục test workflow vì cần stepId." -ForegroundColor Red
    exit 1
}
Write-Host ""

# Test 3: Tạo workflow mới (POST /api/v1/ai/workflows)
Write-Host "[TEST 3] Tạo workflow mới..." -ForegroundColor Yellow
$workflowData = @{
    name = "Test Workflow - $(Get-Date -Format 'yyyyMMdd-HHmmss')"
    description = "Workflow test được tạo tự động"
    version = "1.0.0"
    steps = @(
        @{
            stepId = $firstStepId
            order = 0
            policy = @{
                retryCount = 3
                timeout = 60
                onFailure = "stop"
                onSuccess = "continue"
                parallel = $false
            }
        }
    )
    rootRefType = "pillar"
    targetLevel = "L1"
    defaultPolicy = @{
        retryCount = 2
        timeout = 30
        onFailure = "continue"
        onSuccess = "continue"
        parallel = $false
    }
    status = "active"
    metadata = @{
        test = $true
        createdBy = "test-script"
    }
}

$createResult = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/workflows/insert-one" -Body $workflowData -ActiveRoleID $activeRoleID
if ($createResult.Success) {
    $workflowId = $createResult.Data.data.id
    Write-Host "✓ Đã tạo workflow với ID: $workflowId" -ForegroundColor Green
    Write-Host "  Name: $($createResult.Data.data.name)" -ForegroundColor Gray
    Write-Host "  Version: $($createResult.Data.data.version)" -ForegroundColor Gray
    Write-Host "  Status: $($createResult.Data.data.status)" -ForegroundColor Gray
    Write-Host "  Steps: $($createResult.Data.data.steps.Count)" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi khi tạo workflow: $($createResult.Error.message)" -ForegroundColor Red
    if ($createResult.Error.details) {
        Write-Host "  Chi tiết: $($createResult.Error.details)" -ForegroundColor Red
    }
    exit 1
}
Write-Host ""

# Test 4: Lấy workflow theo ID (GET /api/v1/ai/workflows/find-by-id/:id)
Write-Host "[TEST 4] Lấy workflow theo ID..." -ForegroundColor Yellow
$getResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find-by-id/$workflowId" -ActiveRoleID $activeRoleID
if ($getResult.Success) {
    Write-Host "✓ Thành công!" -ForegroundColor Green
    $workflow = $getResult.Data.data
    Write-Host "  ID: $($workflow.id)" -ForegroundColor Gray
    Write-Host "  Name: $($workflow.name)" -ForegroundColor Gray
    Write-Host "  Description: $($workflow.description)" -ForegroundColor Gray
    Write-Host "  Version: $($workflow.version)" -ForegroundColor Gray
    Write-Host "  RootRefType: $($workflow.rootRefType)" -ForegroundColor Gray
    Write-Host "  TargetLevel: $($workflow.targetLevel)" -ForegroundColor Gray
    Write-Host "  Status: $($workflow.status)" -ForegroundColor Gray
    Write-Host "  Steps: $($workflow.steps.Count)" -ForegroundColor Gray
    Write-Host "  CreatedAt: $($workflow.createdAt)" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi: $($getResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Test 5: Cập nhật workflow (PUT /api/v1/ai/workflows/:id)
Write-Host "[TEST 5] Cập nhật workflow..." -ForegroundColor Yellow
$updateData = @{
    description = "Workflow đã được cập nhật - $(Get-Date -Format 'yyyyMMdd-HHmmss')"
    version = "1.0.1"
    metadata = @{
        test = $true
        updatedBy = "test-script"
        updatedAt = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
}

$updateResult = Invoke-ApiRequest -Method "PUT" -Url "$baseUrl/api/v1/ai/workflows/update-by-id/$workflowId" -Body $updateData -ActiveRoleID $activeRoleID
if ($updateResult.Success) {
    Write-Host "✓ Đã cập nhật workflow thành công!" -ForegroundColor Green
    Write-Host "  Description mới: $($updateResult.Data.data.description)" -ForegroundColor Gray
    Write-Host "  Version mới: $($updateResult.Data.data.version)" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi khi cập nhật: $($updateResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Test 6: Lấy lại danh sách workflows sau khi tạo
Write-Host "[TEST 6] Lấy lại danh sách workflows..." -ForegroundColor Yellow
$listResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find" -ActiveRoleID $activeRoleID
if ($listResult.Success) {
    Write-Host "✓ Thành công! Tổng số workflows: $($listResult.Data.data.Count)" -ForegroundColor Green
    $workflowInList = $listResult.Data.data | Where-Object { $_.id -eq $workflowId }
    if ($workflowInList) {
        Write-Host "  ✓ Workflow vừa tạo có trong danh sách" -ForegroundColor Green
    } else {
        Write-Host "  ⚠ Workflow vừa tạo không có trong danh sách" -ForegroundColor Yellow
    }
} else {
    Write-Host "✗ Lỗi: $($listResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Test 7: Xóa workflow (DELETE /api/v1/ai/workflows/delete-by-id/:id)
Write-Host "[TEST 7] Xóa workflow..." -ForegroundColor Yellow
$deleteResult = Invoke-ApiRequest -Method "DELETE" -Url "$baseUrl/api/v1/ai/workflows/delete-by-id/$workflowId" -ActiveRoleID $activeRoleID
if ($deleteResult.Success) {
    Write-Host "✓ Đã xóa workflow thành công!" -ForegroundColor Green
} else {
    Write-Host "✗ Lỗi khi xóa: $($deleteResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Test 8: Xác nhận workflow đã bị xóa
Write-Host "[TEST 8] Xác nhận workflow đã bị xóa..." -ForegroundColor Yellow
$verifyResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find-by-id/$workflowId" -ActiveRoleID $activeRoleID
if (-not $verifyResult.Success) {
    Write-Host "✓ Xác nhận: Workflow không còn tồn tại (đã bị xóa)" -ForegroundColor Green
} else {
    Write-Host "⚠ Cảnh báo: Workflow vẫn còn tồn tại sau khi xóa" -ForegroundColor Yellow
}
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "HOÀN TẤT TEST AI WORKFLOW" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
