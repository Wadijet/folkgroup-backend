# Script kiểm tra chi tiết input/output schema của steps
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
        $requestHeaders["X-Active-Role-ID"] = $activeRoleID
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
Write-Host "KIỂM TRA CHI TIẾT STEP SCHEMA" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Lấy role ID
$rolesResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/auth/roles"
if (-not $rolesResult.Success) {
    Write-Host "✗ Không thể lấy roles" -ForegroundColor Red
    exit 1
}
$activeRoleID = $rolesResult.Data.data[0].roleId

# Lấy danh sách steps
$stepsResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/steps/find" -ActiveRoleID $activeRoleID
if (-not $stepsResult.Success) {
    Write-Host "✗ Không thể lấy steps" -ForegroundColor Red
    exit 1
}

$steps = $stepsResult.Data.data
Write-Host "Tổng số steps: $($steps.Count)" -ForegroundColor Green
Write-Host ""

foreach ($step in $steps) {
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "STEP: $($step.name)" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "ID: $($step.id)" -ForegroundColor Gray
    Write-Host "Type: $($step.type)" -ForegroundColor Gray
    Write-Host "Status: $($step.status)" -ForegroundColor Gray
    Write-Host ""
    
    # Input Schema
    Write-Host "📥 INPUT SCHEMA:" -ForegroundColor Cyan
    if ($step.inputSchema) {
        $inputJson = $step.inputSchema | ConvertTo-Json -Depth 10
        Write-Host $inputJson -ForegroundColor White
    } else {
        Write-Host "  ✗ Chưa có InputSchema" -ForegroundColor Red
    }
    Write-Host ""
    
    # Output Schema
    Write-Host "📤 OUTPUT SCHEMA:" -ForegroundColor Cyan
    if ($step.outputSchema) {
        $outputJson = $step.outputSchema | ConvertTo-Json -Depth 10
        Write-Host $outputJson -ForegroundColor White
    } else {
        Write-Host "  ✗ Chưa có OutputSchema" -ForegroundColor Red
    }
    Write-Host ""
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "HOÀN TẤT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
