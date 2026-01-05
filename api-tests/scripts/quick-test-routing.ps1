$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"
$baseURL = "http://localhost:8080/api/v1"
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] Quick Test Routing" -ForegroundColor Magenta

# Lấy role ID
$roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers
$roleID = $roleResp.data[0].roleId
$headers["X-Active-Role-ID"] = $roleID
Write-Host "[OK] Role ID: $roleID" -ForegroundColor Green

# Test với system_error
Write-Host "`n[TEST] Trigger system_error..." -ForegroundColor Cyan
$payload = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification qua routing"
        timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
    }
} | ConvertTo-Json -Depth 10

try {
    $resp = Invoke-RestMethod -Uri "$baseURL/notification/trigger" -Method POST -Headers $headers -Body $payload
    
    if ($resp.queued -gt 0) {
        Write-Host "[OK] Da queue $($resp.queued) notification!" -ForegroundColor Green
        Write-Host "   EventType: $($resp.eventType)" -ForegroundColor Gray
    } else {
        Write-Host "[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
        Write-Host "   Message: $($resp.message)" -ForegroundColor Gray
    }
} catch {
    Write-Host "[ERROR] $($_.Exception.Message)" -ForegroundColor Red
}

Write-Host ""
