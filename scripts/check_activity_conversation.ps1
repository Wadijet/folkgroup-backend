# Script kiem tra activity ve lich su chat (conversation)
$baseUrl = "http://localhost:8080/api/v1"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OThjMzQ4OWNiZjYyZGJhMGYwZmQzZmMiLCJ0aW1lIjoiNjk5ZjU1MmMiLCJyYW5kb21OdW1iZXIiOiIxNiJ9.iJglvkv-JziiRF_hCzaNMGFLDG-hW_rEDXBeJSSuk6I"
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

Write-Host "=== Lay roles ===" -ForegroundColor Cyan
try {
    $roles = Invoke-RestMethod -Uri "$baseUrl/auth/roles" -Method GET -Headers $headers
    $roleId = $roles.data[0].roleId
    Write-Host "RoleId: $roleId" -ForegroundColor Green
    $headers["X-Active-Role-ID"] = $roleId
} catch {
    Write-Host "Loi lay roles: $_" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== Dashboard customers (lay unifiedId) ===" -ForegroundColor Cyan
try {
    $cust = Invoke-RestMethod -Uri "$baseUrl/dashboard/customers?period=month&limit=10" -Method GET -Headers $headers
} catch {
    Write-Host "Loi lay customers: $_" -ForegroundColor Red
    exit 1
}

# Dashboard tra ve: data.summary, data.customers, ...
$customers = $cust.data.customers
if (-not $customers) { $customers = @() }
$custCount = if ($customers -is [array]) { $customers.Count } else { @($customers).Count }
Write-Host "So customers: $custCount" -ForegroundColor Yellow
if ($custCount -eq 0) {
    Write-Host "Khong co customers trong dashboard, se dung fallback unifiedId" -ForegroundColor Yellow
}
if (-not $customers) {
    Write-Host "Khong tim thay customers. Response:" -ForegroundColor Red
    $cust | ConvertTo-Json -Depth 5
    exit 1
}

$u = $null
if ($custCount -gt 0) {
    if ($customers[0].customerId) { $u = $customers[0].customerId }  # CustomerItem dung customerId (= unifiedId)
    elseif ($customers[0].unifiedId) { $u = $customers[0].unifiedId }
    elseif ($customers[0].unified_id) { $u = $customers[0].unified_id }
    else { $u = $customers[0]._id }
}
# Fallback: customer co conversation activity (tu check_activity_conversation_data.go)
$convTestId = "51a0c5f8-144b-47fe-ad63-8f0e0e50006f"
if (-not $u -or $u -eq "") { $u = $convTestId; Write-Host "Dung fallback unifiedId" -ForegroundColor Yellow }
Write-Host "UnifiedId: $u" -ForegroundColor Green

Write-Host "`n=== Full profile (activityHistory) cua customer $u ===" -ForegroundColor Cyan
try {
    $profile = Invoke-RestMethod -Uri "$baseUrl/customers/$u/profile" -Method GET -Headers $headers
} catch {
    Write-Host "Loi lay profile: $_" -ForegroundColor Red
    if ($_.Exception.Response) {
        $reader = [System.IO.StreamReader]::new($_.Exception.Response.GetResponseStream())
        Write-Host $reader.ReadToEnd()
    }
    exit 1
}

# Response co the data.activityHistory hoac activityHistory
$acts = $profile.data.activityHistory
if (-not $acts) { $acts = $profile.activityHistory }
if (-not $acts) { $acts = @() }

Write-Host "Tong so activity: $($acts.Count)" -ForegroundColor Yellow
$acts | ForEach-Object { 
    Write-Host "  - domain=$($_.domain) type=$($_.activityType) source=$($_.source) label=$($_.displayLabel)" 
}

$convActs = $acts | Where-Object { $_.domain -eq "conversation" }
Write-Host "`nSo activity CONVERSATION (chat): $($convActs.Count)" -ForegroundColor $(if ($convActs.Count -gt 0) { "Green" } else { "Red" })

# Neu customer dau khong co conversation, thu them 1 customer co conversation de so sanh
if ($convActs.Count -eq 0 -and $u -ne "51a0c5f8-144b-47fe-ad63-8f0e0e50006f") {
    Write-Host "`n--- So sanh: Customer CO conversation (51a0c5f8-...) ---" -ForegroundColor Cyan
    try {
        $p2 = Invoke-RestMethod -Uri "$baseUrl/customers/51a0c5f8-144b-47fe-ad63-8f0e0e50006f/profile" -Method GET -Headers $headers
        $a2 = $p2.data.activityHistory; if (-not $a2) { $a2 = @() }
        $c2 = $a2 | Where-Object { $_.domain -eq "conversation" }
        Write-Host "  Tong: $($a2.Count) | Conversation: $($c2.Count)" -ForegroundColor Green
        $c2 | Select-Object -First 3 | ForEach-Object { Write-Host "    - $($_.displayLabel)" }
    } catch { Write-Host "  Loi: $_" -ForegroundColor Red }
}

# Thong ke theo domain
Write-Host "`n=== Thong ke activity theo domain ===" -ForegroundColor Cyan
$acts | Group-Object -Property domain | ForEach-Object { Write-Host "  $($_.Name): $($_.Count)" }
