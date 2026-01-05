$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers
$headers["X-Active-Role-ID"] = $roleResp.data[0].roleId

# Trigger notification
$body = @{
    eventType = "system_error"
    payload = @{
        errorMessage = "Test notification"
        errorCode = "TEST_001"
    }
} | ConvertTo-Json

$resp = Invoke-RestMethod -Uri "$baseURL/notification/trigger" -Method POST -Headers $headers -Body $body

Write-Host "EventType: $($resp.eventType)"
Write-Host "Queued: $($resp.queued)"
Write-Host "Message: $($resp.message)"

if ($resp.queued -gt 0) {
    Write-Host "`n[SUCCESS] Da queue $($resp.queued) notification!" -ForegroundColor Green
} else {
    Write-Host "`n[WARN] Khong co notification nao duoc queue" -ForegroundColor Yellow
}
