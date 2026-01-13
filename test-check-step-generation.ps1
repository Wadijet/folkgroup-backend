# Script kiểm tra chi tiết STEP_GENERATION step
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
Write-Host "KIỂM TRA STEP_GENERATION" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Lấy role ID
$rolesResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/auth/roles"
if (-not $rolesResult.Success) {
    Write-Host "✗ Không thể lấy roles" -ForegroundColor Red
    exit 1
}
$activeRoleID = $rolesResult.Data.data[0].roleId

# Lấy danh sách steps và tìm STEP_GENERATION
$stepsResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/steps/find" -ActiveRoleID $activeRoleID
if (-not $stepsResult.Success) {
    Write-Host "✗ Không thể lấy steps" -ForegroundColor Red
    exit 1
}

$stepGen = $stepsResult.Data.data | Where-Object { $_.type -eq "STEP_GENERATION" } | Select-Object -First 1

if ($stepGen) {
    Write-Host "✓ Tìm thấy STEP_GENERATION Step" -ForegroundColor Green
    Write-Host "  ID: $($stepGen.id)" -ForegroundColor Gray
    Write-Host "  Name: $($stepGen.name)" -ForegroundColor Gray
    Write-Host "  Type: $($stepGen.type)" -ForegroundColor Gray
    Write-Host "  TargetLevel: $($stepGen.targetLevel)" -ForegroundColor Gray
    Write-Host "  ParentLevel: $($stepGen.parentLevel)" -ForegroundColor Gray
    Write-Host ""
    
    Write-Host "📥 INPUT SCHEMA:" -ForegroundColor Cyan
    Write-Host ($stepGen.inputSchema | ConvertTo-Json -Depth 10) -ForegroundColor White
    Write-Host ""
    
    Write-Host "📤 OUTPUT SCHEMA:" -ForegroundColor Cyan
    Write-Host ($stepGen.outputSchema | ConvertTo-Json -Depth 10) -ForegroundColor White
    Write-Host ""
    
    # Tóm tắt
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "TÓM TẮT INPUT:" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow
    $inputProps = $stepGen.inputSchema.properties
    Write-Host "Required: $($stepGen.inputSchema.required -join ', ')" -ForegroundColor Gray
    Write-Host "Properties:" -ForegroundColor Gray
    foreach ($prop in $inputProps.PSObject.Properties) {
        Write-Host "  - $($prop.Name): $($prop.Value.type)" -ForegroundColor Gray
    }
    Write-Host ""
    
    Write-Host "========================================" -ForegroundColor Yellow
    Write-Host "TÓM TẮT OUTPUT:" -ForegroundColor Yellow
    Write-Host "========================================" -ForegroundColor Yellow
    $outputProps = $stepGen.outputSchema.properties
    Write-Host "Required: $($stepGen.outputSchema.required -join ', ')" -ForegroundColor Gray
    Write-Host "Properties:" -ForegroundColor Gray
    foreach ($prop in $outputProps.PSObject.Properties) {
        Write-Host "  - $($prop.Name): $($prop.Value.type)" -ForegroundColor Gray
    }
} else {
    Write-Host "✗ Không tìm thấy STEP_GENERATION step" -ForegroundColor Red
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "HOÀN TẤT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
