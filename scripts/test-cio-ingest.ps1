# Script kiem tra API CIO Ingest - POST /api/v1/cio/ingest
# Endpoint thong nhat cho Agent sync (Version 4.00). Thay the cac route sync-upsert-one, upsert-messages da go.
# Chay: .\scripts\test-cio-ingest.ps1
# Dang nhap bang email/password qua Firebase -> lay JWT -> chay test

param(
    [string]$Email = "daomanhdung86@gmail.com",
    [string]$Password = "12345678",
    [string]$ApiKey = ""
)

$baseUrl = "http://localhost:8080/api/v1"

# Lay Firebase API Key: env -> development.env
if ($ApiKey -eq "") {
    $ApiKey = $env:FIREBASE_API_KEY
}
if ($ApiKey -eq "") {
    $envPath = Join-Path $PSScriptRoot "..\api\config\env\development.env"
    if (Test-Path $envPath) {
        Get-Content $envPath | ForEach-Object {
            if ($_ -match "^FIREBASE_API_KEY=(.+)$") {
                $ApiKey = $matches[1].Trim()
            }
        }
    }
}
if ($ApiKey -eq "") {
    Write-Host "LOI: Can FIREBASE_API_KEY. Set env FIREBASE_API_KEY hoac -ApiKey '...'" -ForegroundColor Red
    exit 1
}

Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Kiem tra API CIO Ingest (POST /cio/ingest)" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta

# 0. Dang nhap Firebase -> lay ID token -> dang nhap backend -> lay JWT
Write-Host "`nDang nhap voi email $Email..." -ForegroundColor Cyan
$firebaseBody = @{ email = $Email; password = $Password; returnSecureToken = $true } | ConvertTo-Json
try {
    $firebaseResp = Invoke-RestMethod -Uri "https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=$ApiKey" `
        -Method Post -Body $firebaseBody -ContentType "application/json" -ErrorAction Stop
    $firebaseIdToken = $firebaseResp.idToken
} catch {
    Write-Host "LOI Firebase login: $_" -ForegroundColor Red
    if ($_.ErrorDetails.Message) {
        $err = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        if ($err.error.message) { Write-Host "  $($err.error.message)" -ForegroundColor Yellow }
    }
    exit 1
}

$loginBody = @{ idToken = $firebaseIdToken; hwid = "test-cio-ingest" } | ConvertTo-Json
try {
    $loginResp = Invoke-RestMethod -Uri "$baseUrl/auth/login/firebase" -Method POST -Body $loginBody -ContentType "application/json" -ErrorAction Stop
    $bearerToken = $loginResp.data.token
    if (-not $bearerToken) {
        Write-Host "LOI: Khong nhan duoc token tu backend" -ForegroundColor Red
        exit 1
    }
    Write-Host "OK Dang nhap thanh cong" -ForegroundColor Green
} catch {
    Write-Host "LOI Backend login: $_" -ForegroundColor Red
    exit 1
}

$headers = @{
    "Authorization" = "Bearer $bearerToken"
    "Content-Type" = "application/json"
}

# 1. Lay role va organization
$activeRoleId = $null
$activeOrgId = $null
try {
    $roleResp = Invoke-RestMethod -Uri "$baseUrl/auth/roles" -Method GET -Headers $headers
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $adminRole = $roleResp.data | Where-Object { $_.roleName -eq "Administrator" } | Select-Object -First 1
        $role = if ($adminRole) { $adminRole } else { $roleResp.data[0] }
        $activeRoleId = $role.roleId
        Write-Host "`nOK Dùng role: $($role.roleName)" -ForegroundColor Green
    }
} catch {
    Write-Host "`nLỖI Không thể kết nối API (server có đang chạy?): $_" -ForegroundColor Red
    exit 1
}

if (-not $activeRoleId) {
    Write-Host "LỖI: User không có role" -ForegroundColor Red
    exit 1
}

$headers["X-Active-Role-ID"] = $activeRoleId

try {
    $orgResp = Invoke-RestMethod -Uri "$baseUrl/organization" -Method GET -Headers $headers
    if ($orgResp.data -and $orgResp.data.Count -gt 0) {
        $org = $orgResp.data[0]
        $activeOrgId = $org.id
        Write-Host "OK Organization: $($org.name) (id: $activeOrgId)" -ForegroundColor Green
    }
} catch {
    Write-Host "WARN Không lấy được organization: $_" -ForegroundColor Yellow
}

if ($activeOrgId) {
    $headers["X-Active-Organization-ID"] = $activeOrgId
}

$ingestUrl = "$baseUrl/cio/ingest"
$passCount = 0
$failCount = 0

# --- Test 1: domain thiếu → 400
Write-Host "`n--- Test 1: Thiếu domain (expect 400) ---" -ForegroundColor Cyan
try {
    $body = @{ data = @{} } | ConvertTo-Json -Depth 5
    $resp = Invoke-WebRequest -Uri $ingestUrl -Method POST -Headers $headers -Body $body -ErrorAction SilentlyContinue
    Write-Host "FAIL Mong đợi 400, nhận HTTP $($resp.StatusCode)" -ForegroundColor Red
    $failCount++
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 400) {
        Write-Host "OK Nhận 400 như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 400, nhận HTTP $statusCode" -ForegroundColor Red
        $failCount++
    }
}

# --- Test 2: domain không hợp lệ → 400
Write-Host "`n--- Test 2: Domain không hợp lệ (expect 400) ---" -ForegroundColor Cyan
try {
    $body = @{ domain = "invalid_domain"; data = @{} } | ConvertTo-Json -Depth 5
    $resp = Invoke-WebRequest -Uri $ingestUrl -Method POST -Headers $headers -Body $body -ErrorAction SilentlyContinue
    Write-Host "FAIL Mong đợi 400, nhận HTTP $($resp.StatusCode)" -ForegroundColor Red
    $failCount++
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 400) {
        Write-Host "OK Nhận 400 như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 400, nhận HTTP $statusCode" -ForegroundColor Red
        $failCount++
    }
}

# --- Test 3: domain ads (stub 501)
Write-Host "`n--- Test 3: Domain ads - stub 501 ---" -ForegroundColor Cyan
try {
    $body = @{ domain = "ads"; data = @{} } | ConvertTo-Json -Depth 5
    $resp = Invoke-WebRequest -Uri $ingestUrl -Method POST -Headers $headers -Body $body -ErrorAction SilentlyContinue
    if ($resp.StatusCode -eq 501) {
        Write-Host "OK Nhận 501 Not Implemented như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 501, nhận HTTP $($resp.StatusCode)" -ForegroundColor Red
        $failCount++
    }
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 501) {
        Write-Host "OK Nhận 501 như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 501, nhận HTTP $statusCode" -ForegroundColor Red
        $failCount++
    }
}

# --- Test 4: domain crm (stub 501)
Write-Host "`n--- Test 4: Domain crm - stub 501 ---" -ForegroundColor Cyan
try {
    $body = @{ domain = "crm"; data = @{} } | ConvertTo-Json -Depth 5
    $resp = Invoke-WebRequest -Uri $ingestUrl -Method POST -Headers $headers -Body $body -ErrorAction SilentlyContinue
    if ($resp.StatusCode -eq 501) {
        Write-Host "OK Nhận 501 Not Implemented như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 501, nhận HTTP $($resp.StatusCode)" -ForegroundColor Red
        $failCount++
    }
} catch {
    $statusCode = $_.Exception.Response.StatusCode.value__
    if ($statusCode -eq 501) {
        Write-Host "OK Nhận 501 như mong đợi" -ForegroundColor Green
        $passCount++
    } else {
        Write-Host "FAIL Mong đợi 501, nhận HTTP $statusCode" -ForegroundColor Red
        $failCount++
    }
}

# --- Test 5: domain order (can order mau tu DB)
Write-Host "`n--- Test 5: Domain order (sync-upsert) ---" -ForegroundColor Cyan
$orderFilterEncoded = [System.Uri]::EscapeDataString("{}")
$optionsEncoded = [System.Uri]::EscapeDataString("{`"limit`":1}")
$ordersUrl = "$baseUrl/pancake-pos/order/find?filter=$orderFilterEncoded&options=$optionsEncoded"
try {
    $ordersResp = Invoke-RestMethod -Uri $ordersUrl -Method GET -Headers $headers -ErrorAction Stop
    if ($ordersResp.status -eq "success" -and $ordersResp.data -and $ordersResp.data.Count -gt 0) {
        $sample = $ordersResp.data[0]
        $orderId = $sample.orderId
        $orgId = $sample.ownerOrganizationId
        $ingestBody = @{
            domain = "order"
            filter = @{ orderId = $orderId; ownerOrganizationId = $orgId }
            data   = $sample
        } | ConvertTo-Json -Depth 25
        try {
            $ingestResp = Invoke-RestMethod -Uri $ingestUrl -Method POST -Headers $headers -Body $ingestBody -ErrorAction Stop
            if ($ingestResp.status -eq "success") {
                Write-Host "OK Order ingest thành công (orderId=$orderId)" -ForegroundColor Green
                if ($ingestResp.skipped) {
                    Write-Host "  (skipped - dữ liệu không thay đổi)" -ForegroundColor Gray
                }
                $passCount++
            } else {
                Write-Host "FAIL Response: $($ingestResp | ConvertTo-Json -Depth 2)" -ForegroundColor Red
                $failCount++
            }
        } catch {
            Write-Host "FAIL Lỗi ingest order: $_" -ForegroundColor Red
            $failCount++
        }
    } else {
        Write-Host "SKIP Khong co order mau trong DB, bo qua test order" -ForegroundColor Yellow
    }
} catch {
    Write-Host "SKIP Không lấy được orders: $_" -ForegroundColor Yellow
}

# --- Test 6: domain interaction_conversation (can conversation mau)
Write-Host "`n--- Test 6: Domain interaction_conversation ---" -ForegroundColor Cyan
$convFilterEncoded = [System.Uri]::EscapeDataString("{}")
$convOptionsEncoded = [System.Uri]::EscapeDataString("{`"limit`":1}")
$convUrl = "$baseUrl/facebook/conversation/find?filter=$convFilterEncoded&options=$convOptionsEncoded"
try {
    $convResp = Invoke-RestMethod -Uri $convUrl -Method GET -Headers $headers -ErrorAction Stop
    if ($convResp.status -eq "success" -and $convResp.data -and $convResp.data.Count -gt 0) {
        $conv = $convResp.data[0]
        $convId = $conv.conversationId
        if (-not $convId) { $convId = $conv.id }
        $ingestBody = @{
            domain = "interaction_conversation"
            filter = @{ conversationId = $convId }
            data   = $conv
        } | ConvertTo-Json -Depth 25
        try {
            $ingestResp = Invoke-RestMethod -Uri $ingestUrl -Method POST -Headers $headers -Body $ingestBody -ErrorAction Stop
            if ($ingestResp.status -eq "success") {
                Write-Host "OK Conversation ingest thành công (conversationId=$convId)" -ForegroundColor Green
                $passCount++
            } else {
                Write-Host "FAIL Response: $($ingestResp | ConvertTo-Json -Depth 2)" -ForegroundColor Red
                $failCount++
            }
        } catch {
            Write-Host "FAIL Lỗi ingest conversation: $_" -ForegroundColor Red
            $failCount++
        }
    } else {
        Write-Host "SKIP Khong co conversation mau, bo qua" -ForegroundColor Yellow
    }
} catch {
    Write-Host "SKIP Không lấy được conversations: $_" -ForegroundColor Yellow
}

# --- Tong ket
$resultColor = if ($failCount -eq 0) { "Green" } else { "Yellow" }
Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
$msg = "Ket qua: " + $passCount + " PASS, " + $failCount + " FAIL"
Write-Host $msg -ForegroundColor $resultColor
Write-Host "========================================" -ForegroundColor Magenta
