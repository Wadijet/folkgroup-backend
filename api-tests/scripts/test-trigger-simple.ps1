# Script test đơn giản để trigger notification và xem logs
param(
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResp = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
$roleID = $roleResp.data[0].roleId
$headers["X-Active-Role-ID"] = $roleID
Write-Host "[OK] Role ID: $roleID" -ForegroundColor Green

# Trigger notification
Write-Host "`n[TEST] Trigger notification với system_error..." -ForegroundColor Cyan
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification"
        errorCode = "TEST_001"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    Write-Host "[RESPONSE] EventType: $($resp.eventType)" -ForegroundColor Yellow
    Write-Host "[RESPONSE] Queued: $($resp.queued)" -ForegroundColor Yellow
    Write-Host "[RESPONSE] Message: $($resp.message)" -ForegroundColor Yellow
    
    if ($resp.queued -gt 0) {
        Write-Host "`n[SUCCESS] Đã queue $($resp.queued) notification!" -ForegroundColor Green
    } else {
        Write-Host "`n[WARN] Không có notification nào được queue" -ForegroundColor Yellow
        Write-Host "Hãy kiểm tra logs của server để xem debug logs từ router" -ForegroundColor Yellow
    }
} catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        Write-Host "Chi tiết: $($_.ErrorDetails.Message)" -ForegroundColor Red
    }
}

Write-Host ""
