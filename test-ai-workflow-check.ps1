# Script kiểm tra workflow có OwnerOrganizationID trong DB
$baseUrl = "http://localhost:8080"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVmN2IzOGNiZjYyZGJhMGZiMDk0Y2IiLCJ0aW1lIjoiNjk2NDU4ZGEiLCJyYW5kb21OdW1iZXIiOiI3NyJ9.GumBzdYurrNOB-hVVjCbSYp0Na8E7hdMQg8XpLO6g6k"
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

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

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "KIỂM TRA WORKFLOW TRONG DB" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Lấy role ID
$rolesResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/auth/roles"
if (-not $rolesResult.Success) {
    Write-Host "✗ Không thể lấy roles" -ForegroundColor Red
    exit 1
}
$activeRoleID = $rolesResult.Data.data[0].roleId
Write-Host "Role ID: $activeRoleID" -ForegroundColor Gray
Write-Host ""

# Lấy danh sách steps
$stepsResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/steps/find" -ActiveRoleID $activeRoleID
if (-not $stepsResult.Success -or $stepsResult.Data.data.Count -eq 0) {
    Write-Host "✗ Không có steps nào" -ForegroundColor Red
    exit 1
}
$stepId = $stepsResult.Data.data[0].id
Write-Host "Step ID: $stepId" -ForegroundColor Gray
Write-Host ""

# Tạo workflow
Write-Host "[1] Tạo workflow..." -ForegroundColor Yellow
$workflowData = @{
    name = "Test Workflow - Check DB - $(Get-Date -Format 'yyyyMMdd-HHmmss')"
    description = "Workflow để kiểm tra OwnerOrganizationID"
    version = "1.0.0"
    steps = @(
        @{
            stepId = $stepId
            order = 0
        }
    )
    rootRefType = "layer"
    targetLevel = "L1"
    status = "active"
}

$createResult = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/workflows/insert-one" -Body $workflowData -ActiveRoleID $activeRoleID
if (-not $createResult.Success) {
    Write-Host "✗ Lỗi khi tạo workflow: $($createResult.Error.message)" -ForegroundColor Red
    exit 1
}

$workflowId = $createResult.Data.data.id
Write-Host "✓ Đã tạo workflow với ID: $workflowId" -ForegroundColor Green
Write-Host "  OwnerOrganizationID: $($createResult.Data.data.ownerOrganizationId)" -ForegroundColor Gray
Write-Host ""

# Đợi một chút để đảm bảo đã lưu vào DB
Start-Sleep -Seconds 1

# Kiểm tra workflow trong danh sách
Write-Host "[2] Kiểm tra workflow trong danh sách..." -ForegroundColor Yellow
$listResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find" -ActiveRoleID $activeRoleID
if ($listResult.Success) {
    Write-Host "✓ Tổng số workflows: $($listResult.Data.data.Count)" -ForegroundColor Green
    $workflowInList = $listResult.Data.data | Where-Object { $_.id -eq $workflowId }
    if ($workflowInList) {
        Write-Host "✓ Workflow có trong danh sách!" -ForegroundColor Green
        Write-Host "  ID: $($workflowInList.id)" -ForegroundColor Gray
        Write-Host "  Name: $($workflowInList.name)" -ForegroundColor Gray
        Write-Host "  OwnerOrganizationID: $($workflowInList.ownerOrganizationId)" -ForegroundColor Gray
    } else {
        Write-Host "✗ Workflow KHÔNG có trong danh sách!" -ForegroundColor Red
        Write-Host "  Có thể do:" -ForegroundColor Yellow
        Write-Host "    - OwnerOrganizationID không được set khi tạo" -ForegroundColor Yellow
        Write-Host "    - Filter organization không match" -ForegroundColor Yellow
        Write-Host "    - Permission không đủ" -ForegroundColor Yellow
    }
} else {
    Write-Host "✗ Lỗi khi lấy danh sách: $($listResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

# Kiểm tra workflow theo ID
Write-Host "[3] Kiểm tra workflow theo ID..." -ForegroundColor Yellow
$getResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/workflows/find-by-id/$workflowId" -ActiveRoleID $activeRoleID
if ($getResult.Success) {
    Write-Host "✓ Có thể lấy workflow theo ID!" -ForegroundColor Green
    Write-Host "  OwnerOrganizationID: $($getResult.Data.data.ownerOrganizationId)" -ForegroundColor Gray
} else {
    Write-Host "✗ Không thể lấy workflow theo ID: $($getResult.Error.message)" -ForegroundColor Red
}
Write-Host ""

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "HOÀN TẤT KIỂM TRA" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "Lưu ý: Workflow này sẽ KHÔNG bị xóa để bạn có thể kiểm tra trong DB" -ForegroundColor Yellow
