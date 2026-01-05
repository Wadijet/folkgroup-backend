# Script kiểm tra System Organization và templates
$BaseURL = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVhYTBiOGNiZjYyZGJhMGZjNzJhMzciLCJ0aW1lIjoiNjk1YWEyZDciLCJyYW5kb21OdW1iZXIiOiI5NyJ9.FRiCnIz5I98YaMf2rrWBEjibjMvucTuJ-KYo9Mcy1a0"

$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

# Lấy role ID
$roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
if ($roleResponse.data -and $roleResponse.data.Count -gt 0 -and $roleResponse.data[0].roleId) {
    $headers["X-Active-Role-ID"] = $roleResponse.data[0].roleId
}

Write-Host "`n[CHECK] Kiểm tra System Organization và Templates" -ForegroundColor Magenta
Write-Host "============================================================" -ForegroundColor Magenta

# Lấy System Organization
Write-Host "`n[1] Tìm System Organization..." -ForegroundColor Yellow
$orgResponse = Invoke-RestMethod -Uri "$BaseURL/organization/find" -Method GET -Headers $headers
$systemOrg = $orgResponse.data | Where-Object { $_.level -eq -1 -and $_.code -eq "SYSTEM" } | Select-Object -First 1

if ($systemOrg) {
    $systemOrgID = if ($systemOrg._id) { $systemOrg._id } else { $systemOrg.id }
    Write-Host "   [OK] System Organization ID: $systemOrgID" -ForegroundColor Green
    Write-Host "   Name: $($systemOrg.name)" -ForegroundColor Gray
    Write-Host "   Code: $($systemOrg.code)" -ForegroundColor Gray
    Write-Host "   Level: $($systemOrg.level)" -ForegroundColor Gray
} else {
    Write-Host "   [ERROR] Không tìm thấy System Organization!" -ForegroundColor Red
    exit 1
}

# Kiểm tra templates với systemOrgID
Write-Host "`n[2] Kiểm tra templates Telegram với System Organization ID..." -ForegroundColor Yellow
$templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
$telegramTemplates = $templateResponse.data | Where-Object { $_.channelType -eq "telegram" }

Write-Host "   Tổng số templates Telegram: $($telegramTemplates.Count)" -ForegroundColor Gray

$systemTemplates = $telegramTemplates | Where-Object { $_.ownerOrganizationId -eq $systemOrgID }
Write-Host "   Templates thuộc System Organization: $($systemTemplates.Count)" -ForegroundColor $(if ($systemTemplates.Count -gt 0) { "Green" } else { "Yellow" })

if ($systemTemplates.Count -gt 0) {
    Write-Host "`n   Các templates system:" -ForegroundColor Cyan
    foreach ($tpl in $systemTemplates | Select-Object -First 5) {
        Write-Host "     - $($tpl.eventType) (OwnerOrgID: $($tpl.ownerOrganizationId))" -ForegroundColor DarkGray
    }
} else {
    Write-Host "`n   [WARN] Không có template nào thuộc System Organization!" -ForegroundColor Yellow
    Write-Host "   Các templates hiện có:" -ForegroundColor Yellow
    foreach ($tpl in $telegramTemplates | Select-Object -First 5) {
        $ownerID = if ($tpl.ownerOrganizationId) { $tpl.ownerOrganizationId } else { "null" }
        Write-Host "     - $($tpl.eventType) (OwnerOrgID: $ownerID)" -ForegroundColor DarkGray
    }
}

# Test với eventType system_error
Write-Host "`n[3] Test tìm template cho system_error..." -ForegroundColor Yellow
$testEventType = "system_error"
$testTemplate = $telegramTemplates | Where-Object { 
    $_.eventType -eq $testEventType -and 
    $_.ownerOrganizationId -eq $systemOrgID 
} | Select-Object -First 1

if ($testTemplate) {
    Write-Host "   [OK] Tìm thấy template system_error với System Organization!" -ForegroundColor Green
} else {
    Write-Host "   [WARN] Không tìm thấy template system_error với System Organization" -ForegroundColor Yellow
    $anyTemplate = $telegramTemplates | Where-Object { $_.eventType -eq $testEventType } | Select-Object -First 1
    if ($anyTemplate) {
        Write-Host "   Template system_error có OwnerOrgID: $($anyTemplate.ownerOrganizationId)" -ForegroundColor Yellow
        Write-Host "   SystemOrgID: $systemOrgID" -ForegroundColor Yellow
    }
}

Write-Host ""
