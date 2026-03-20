# Script tổng thể chạy test - Tất cả trong một
# Sử dụng: .\api-tests\test.ps1
# Hoặc: .\api-tests\test.ps1 -SkipServer (nếu server đã chạy sẵn)
# Hoặc: .\api-tests\test.ps1 -UnitOnly (chỉ chạy unit tests trong api, không cần server)

param(
    [switch]$SkipServer = $false,   # Bỏ qua khởi động server nếu đã chạy
    [switch]$UnitOnly = $false      # Chỉ chạy unit tests trong api (nhanh, không cần server/DB)
)

$ErrorActionPreference = "Continue"

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  TEST SUITE - QUY TRINH DAY DU" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Đảm bảo chạy từ thư mục gốc của project
$scriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$projectRoot = Split-Path -Parent $scriptDir
Set-Location $projectRoot

# ============================================
# CHẾ ĐỘ UNIT ONLY - Chạy unit tests trong api rồi thoát
# ============================================
if ($UnitOnly) {
    Write-Host "[UNIT] Chay unit tests trong api (khong can server)..." -ForegroundColor Yellow
    $unitOutput = go test ./api/... -short -count=1 -v 2>&1
    $unitExit = $LASTEXITCODE
    Write-Host $unitOutput
    Write-Host ""
    if ($unitExit -eq 0) {
        Write-Host "[OK] Unit tests PASSED" -ForegroundColor Green
    } else {
        Write-Host "[FAIL] Unit tests FAILED" -ForegroundColor Red
    }
    exit $unitExit
}

# Kiểm tra file config
if (-not (Test-Path "$projectRoot\api\config\env\development.env")) {
    Write-Host "[ERROR] Khong tim thay file config: $projectRoot\api\config\env\development.env" -ForegroundColor Red
    Write-Host "[INFO] Dam bao ban dang chay script tu thu muc goc cua project" -ForegroundColor Yellow
    exit 1
}

# Load env từ development.env (FIREBASE_API_KEY, v.v.) và set TEST_EMAIL/TEST_PASSWORD nếu chưa có
$envPath = "$projectRoot\api\config\env\development.env"
Get-Content $envPath | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
        $key = $matches[1].Trim()
        $val = $matches[2].Trim()
        if (-not [string]::IsNullOrEmpty($val) -and -not [Environment]::GetEnvironmentVariable($key, "Process")) {
            [Environment]::SetEnvironmentVariable($key, $val, "Process")
        }
    }
}
# Nếu chưa có TEST_FIREBASE_ID_TOKEN, dùng email/password (ưu tiên env, fallback mặc định)
if (-not $env:TEST_FIREBASE_ID_TOKEN) {
    if (-not $env:TEST_EMAIL) { $env:TEST_EMAIL = "daomanhdung86@gmail.com" }
    if (-not $env:TEST_PASSWORD) { $env:TEST_PASSWORD = "12345678" }
    Write-Host "[INFO] Dung login email: $env:TEST_EMAIL (FIREBASE_API_KEY tu development.env)" -ForegroundColor Gray
}

# ============================================
# BƯỚC 1: KIỂM TRA VÀ KHỞI ĐỘNG SERVER
# ============================================
Write-Host "[1/4] Kiem tra server..." -ForegroundColor Yellow

$serverRunning = $false
$serverProcess = $null

# Base URL: ưu tiên TEST_BASE_URL, fallback từ ADDRESS trong env
$baseURL = $env:TEST_BASE_URL
if (-not $baseURL) {
    $port = ($env:ADDRESS -replace '^:', '')  # bỏ dấu : nếu có
    if (-not $port) { $port = "8080" }
    $baseURL = "http://localhost:$port"
}
$healthURL = "$baseURL/api/v1/system/health"

try {
    $response = Invoke-WebRequest -Uri $healthURL -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
    if ($response.StatusCode -eq 200) {
        $serverRunning = $true
        Write-Host "  [OK] Server dang chay ($healthURL)" -ForegroundColor Green
    }
} catch {
    # Thử /health nếu /api/v1/system/health fail
    try {
        $altURL = "$baseURL/health"
        $response = Invoke-WebRequest -Uri $altURL -UseBasicParsing -TimeoutSec 5 -ErrorAction Stop
        if ($response.StatusCode -eq 200) {
            $serverRunning = $true
            Write-Host "  [OK] Server dang chay ($altURL)" -ForegroundColor Green
        }
    } catch {}
    if (-not $serverRunning) {
        Write-Host "  [INFO] Health check that bai: $healthURL" -ForegroundColor Gray
        Write-Host "  [INFO] Loi: $($_.Exception.Message)" -ForegroundColor Gray
        Write-Host "  [INFO] Kiem tra server co chay tren port dung khong (ADDRESS=$env:ADDRESS)" -ForegroundColor Gray
    }
}

# Khởi động server nếu cần
if (-not $serverRunning -and -not $SkipServer) {
    Write-Host "[2/4] Khoi dong server..." -ForegroundColor Yellow
    
    # Dừng server cũ nếu có
    try {
        $oldProcesses = Get-NetTCPConnection -LocalPort 8080 -ErrorAction SilentlyContinue | 
            Select-Object -ExpandProperty OwningProcess -Unique
        if ($oldProcesses) {
            Write-Host "  Dung server cu..." -ForegroundColor Gray
            foreach ($processId in $oldProcesses) {
                Stop-Process -Id $processId -Force -ErrorAction SilentlyContinue
            }
            Start-Sleep -Seconds 2
        }
    } catch {}
    
    # Khởi động server
    # Build server trước rồi chạy executable để đảm bảo working directory đúng
    $env:GO_ENV = "development"
    
    # Build server
    Write-Host "  Dang build server..." -ForegroundColor Gray
    Push-Location $projectRoot
    $buildOutput = go build -o ".\server_test.exe" .\api\cmd\server\ 2>&1
    if ($LASTEXITCODE -ne 0) {
        Write-Host "  [ERROR] Khong the build server:" -ForegroundColor Red
        Write-Host $buildOutput
        Pop-Location
        exit 1
    }
    
    # Chạy server
    $serverProcess = Start-Process -FilePath ".\server_test.exe" `
        -PassThru `
        -WindowStyle Hidden `
        -WorkingDirectory $projectRoot
    Pop-Location
    
    Write-Host "  Server PID: $($serverProcess.Id)" -ForegroundColor Gray
    
    # Đợi server sẵn sàng
    Write-Host "  Dang doi server khoi dong..." -ForegroundColor Gray
    $ready = $false
    for ($i = 1; $i -le 60; $i++) {
        try {
            $response = Invoke-WebRequest -Uri "http://localhost:8080/api/v1/system/health" -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
            if ($response.StatusCode -eq 200) {
                $ready = $true
                Write-Host "  [OK] Server san sang sau $i giay" -ForegroundColor Green
                break
            }
        } catch {
            if ($i % 10 -eq 0) {
                Write-Host "  Dang doi... ($i/60 giay)" -ForegroundColor Gray
            }
            Start-Sleep -Seconds 1
        }
    }
    
    if (-not $ready) {
        Write-Host "  [ERROR] Server khong san sang sau 60 giay" -ForegroundColor Red
        if ($serverProcess) {
            Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
        }
        exit 1
    }
} else {
    if ($SkipServer) {
        Write-Host "[2/4] Bo qua khoi dong server (SkipServer flag)" -ForegroundColor Gray
    } else {
        Write-Host "[2/4] Bo qua khoi dong server (da chay)" -ForegroundColor Gray
    }
}

# ============================================
# BƯỚC 3: CHẠY TEST
# ============================================
Write-Host "[3/4] Chay test suite..." -ForegroundColor Yellow
Write-Host "========================================" -ForegroundColor Cyan

# Ghi nhận thời gian bắt đầu
$startTime = Get-Date

$testOutput = go test -v ./api-tests/cases/... 2>&1
$testExitCode = $LASTEXITCODE

# Ghi nhận thời gian kết thúc
$endTime = Get-Date
$duration = $endTime - $startTime

Write-Host "========================================" -ForegroundColor Cyan

# Đếm kết quả
$totalTests = ($testOutput | Select-String -Pattern "=== RUN" | Measure-Object).Count
$passedTests = ($testOutput | Select-String -Pattern "--- PASS:" | Measure-Object).Count
$failedTests = ($testOutput | Select-String -Pattern "--- FAIL:" | Measure-Object).Count
$skippedTests = ($testOutput | Select-String -Pattern "--- SKIP:" | Measure-Object).Count

# Parse test details
$testDetails = @()
$currentTest = $null
$testLines = $testOutput -split "`n"

foreach ($line in $testLines) {
    if ($line -match "=== RUN\s+(.+)") {
        if ($currentTest) {
            $testDetails += $currentTest
        }
        $currentTest = @{
            Name = $matches[1]
            Status = "RUNNING"
            Duration = ""
            Errors = @()
        }
    }
    elseif ($line -match "--- PASS:\s+(.+)\s+\(([\d.]+)s\)") {
        if ($currentTest) {
            $currentTest.Status = "PASS"
            $currentTest.Duration = $matches[2] + "s"
            $testDetails += $currentTest
            $currentTest = $null
        }
    }
    elseif ($line -match "--- FAIL:\s+(.+)\s+\(([\d.]+)s\)") {
        if ($currentTest) {
            $currentTest.Status = "FAIL"
            $currentTest.Duration = $matches[2] + "s"
            $testDetails += $currentTest
            $currentTest = $null
        }
    }
    elseif ($line -match "--- SKIP:\s+(.+)") {
        if ($currentTest) {
            $currentTest.Status = "SKIP"
            $testDetails += $currentTest
            $currentTest = $null
        }
    }
    elseif ($currentTest -and $line -match "^\s+Error") {
        $currentTest.Errors += $line.Trim()
    }
}

if ($currentTest) {
    $testDetails += $currentTest
}

# ============================================
# BƯỚC 4: DỪNG SERVER (nếu đã khởi động)
# ============================================
if ($serverProcess) {
    Write-Host "[4/5] Dung server..." -ForegroundColor Yellow
    try {
        Stop-Process -Id $serverProcess.Id -Force -ErrorAction SilentlyContinue
        Write-Host "  [OK] Da dung server" -ForegroundColor Green
    } catch {
        Write-Host "  [INFO] Khong the dung server (co the da tu dung)" -ForegroundColor Gray
    }
} else {
    Write-Host "[4/5] Bo qua dung server (khong phai do script khoi dong)" -ForegroundColor Gray
}

# ============================================
# BƯỚC 5: TẠO BÁO CÁO
# ============================================
Write-Host "[5/5] Tao bao cao..." -ForegroundColor Yellow

# Tính toán pass rate
$passRate = if ($totalTests -gt 0) { 
    [math]::Round(($passedTests / $totalTests) * 100, 1) 
} else { 
    0 
}

# Format duration đẹp hơn
$durationFormatted = if ($duration.TotalSeconds -lt 60) {
    "$([math]::Round($duration.TotalSeconds, 2)) giây"
} elseif ($duration.TotalMinutes -lt 60) {
    "$([math]::Round($duration.TotalMinutes, 2)) phút"
} else {
    "$([math]::Round($duration.TotalHours, 2)) giờ"
}

# Tạo thư mục reports nếu chưa có
$reportsPath = Join-Path $scriptDir "reports"
if (-not (Test-Path $reportsPath)) {
    New-Item -ItemType Directory -Path $reportsPath -Force | Out-Null
}

# Tạo tên file báo cáo với timestamp (Markdown format)
$timestamp = Get-Date -Format "yyyy-MM-dd_HH-mm-ss"
$reportFile = Join-Path $reportsPath "test_report_$timestamp.md"

# Đọc template Markdown
$templatePath = Join-Path $scriptDir "templates\report_template.md"
if (Test-Path $templatePath) {
    $template = Get-Content -Path $templatePath -Raw -Encoding UTF8
    
    # Tạo status badge cho Markdown (không dùng emoji trong PowerShell string)
    $statusBadge = ""
    if ($failedTests -eq 0 -and $totalTests -gt 0) {
        $statusBadge = "**TAT CA TEST DA PASS!** He thong hoat dong tot."
    } elseif ($failedTests -gt 0 -and $passedTests -gt 0) {
        $statusBadge = "**CO MOT SO TEST FAILED.** Vui long kiem tra chi tiet ben duoi."
    } elseif ($failedTests -eq $totalTests) {
        $statusBadge = "**TAT CA TEST DA FAILED!** Can kiem tra lai he thong."
    } else {
        $statusBadge = "Chua co test nao duoc chay."
    }
    
    # Tạo test details section (Markdown format)
    $testDetailsSection = ""
    if ($testDetails.Count -gt 0) {
        $testDetailsSection += "`n| # | Test Case | Trang Thai | Thoi Gian |`n"
        $testDetailsSection += "|:-:|-----------|:----------:|:---------:|`n"
        
        $testNumber = 1
        foreach ($test in $testDetails) {
            $statusIcon = switch ($test.Status) {
                "PASS" { "PASSED" }
                "FAIL" { "FAILED" }
                "SKIP" { "SKIPPED" }
                default { "RUNNING" }
            }
            
            # Escape Markdown special characters
            $testName = $test.Name -replace '\|', '\|'
            $testName = $testName -replace '\*', '\*'
            $testName = $testName -replace '_', '\_'
            $testName = $testName -replace '`', '\`'
            
            # Rút ngắn tên nếu quá dài
            if ($testName.Length -gt 60) {
                $testName = $testName.Substring(0, 57) + "..."
            }
            
            $duration = if ($test.Duration) { $test.Duration } else { "-" }
            
            $testDetailsSection += "| $testNumber | $testName | $statusIcon | $duration |`n"
            
            # Thêm error message nếu có
            if ($test.Status -eq "FAIL" -and $test.Errors.Count -gt 0) {
                $errorMsg = $test.Errors[0] -replace '\|', '\|'
                $errorMsg = $errorMsg -replace '\*', '\*'
                if ($errorMsg.Length -gt 150) {
                    $errorMsg = $errorMsg.Substring(0, 147) + "..."
                }
                $testDetailsSection += "|   | *$errorMsg* | | |`n"
            }
            
            $testNumber++
        }
    } else {
        $testDetailsSection = "Khong co test nao duoc chay hoac khong parse duoc ket qua."
    }
    
    # Tạo recommendations (Markdown format)
    $recommendations = ""
    if ($failedTests -gt 0) {
        $recommendations += "- Kiem tra lai cac test case da failed`n"
        $recommendations += "- Xem log chi tiet o phan duoi de tim nguyen nhan`n"
        $recommendations += "- Dam bao server dang chay dung va database da duoc khoi tao`n"
        $recommendations += "- Kiem tra cac API endpoint co dang hoat dong khong`n"
    } else {
        $recommendations += "- Tat ca test da pass! He thong hoat dong tot.`n"
        $recommendations += "- Co the tiep tuc deploy hoac merge code.`n"
    }
    
    if ($totalTests -eq 0) {
        $recommendations += "- Khong co test nao duoc chay. Kiem tra lai cau hinh test.`n"
    }
    
    # Format test output cho Markdown
    $testOutputText = [string]::Join("`n", $testOutput)
    # Giới hạn độ dài test output nếu quá dài (giữ lại 100KB đầu)
    if ($testOutputText.Length -gt 100000) {
        $testOutputText = $testOutputText.Substring(0, 100000) + "`n`n... (output đã được cắt ngắn do quá dài, xem log đầy đủ trong console khi chạy test)"
    }
    
    # Nếu không có output, thêm thông báo
    if ([string]::IsNullOrWhiteSpace($testOutputText)) {
        $testOutputText = "Không có log output từ test."
    }
    
    # Escape Markdown code block special characters
    $testOutputFormatted = $testOutputText -replace '```', '`` `'
    
    # Thay thế các placeholder
    $report = $template
    $report = $report.Replace("{{START_TIME}}", $startTime.ToString("yyyy-MM-dd HH:mm:ss"))
    $report = $report.Replace("{{END_TIME}}", $endTime.ToString("yyyy-MM-dd HH:mm:ss"))
    $report = $report.Replace("{{DURATION}}", $durationFormatted)
    $report = $report.Replace("{{PASS_RATE}}", $passRate.ToString())
    $report = $report.Replace("{{TOTAL_TESTS}}", $totalTests.ToString())
    $report = $report.Replace("{{PASSED_TESTS}}", $passedTests.ToString())
    $report = $report.Replace("{{FAILED_TESTS}}", $failedTests.ToString())
    $report = $report.Replace("{{SKIPPED_TESTS}}", $skippedTests.ToString())
    $report = $report.Replace("{{REPORT_FILE}}", (Split-Path -Leaf $reportFile))
    $report = $report.Replace("{{STATUS_BADGE}}", $statusBadge)
    $report = $report.Replace("{{TEST_DETAILS}}", $testDetailsSection)
    $report = $report.Replace("{{RECOMMENDATIONS}}", $recommendations)
    $report = $report.Replace("{{TEST_OUTPUT}}", $testOutputFormatted)
    
    # Ghi file Markdown với UTF-8
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($reportFile, $report, $utf8NoBom)
    
    Write-Host "  [OK] Bao cao da duoc luu tai: $reportFile" -ForegroundColor Green
} else {
    Write-Host "  [WARNING] Khong tim thay template Markdown, tao bao cao Markdown don gian..." -ForegroundColor Yellow
    
    # Tạo Markdown đơn giản nếu không có template
    $statusBadge = ""
    if ($failedTests -eq 0 -and $totalTests -gt 0) {
        $statusBadge = "**TAT CA TEST DA PASS!** He thong hoat dong tot."
    } elseif ($failedTests -gt 0) {
        $statusBadge = "**CO TEST FAILED**"
    } else {
        $statusBadge = "Chua co test nao duoc chay"
    }
    
    $testOutputText = [string]::Join("`n", $testOutput)
    if ($testOutputText.Length -gt 100000) {
        $testOutputText = $testOutputText.Substring(0, 100000) + "`n`n... (output đã được cắt ngắn)"
    }
    $testOutputFormatted = $testOutputText -replace '```', '`` `'
    
    $simpleReport = @"
# Bao Cao Ket Qua Test

**Thoi gian:** $($startTime.ToString("yyyy-MM-dd HH:mm:ss")) - $($endTime.ToString("yyyy-MM-dd HH:mm:ss"))
**Thoi luong:** $durationFormatted

## Tong Ket

| Chi so | Gia tri |
|--------|---------|
| Tong so test | $totalTests |
| Passed | $passedTests |
| Failed | $failedTests |
| Pass Rate | $passRate% |

$statusBadge

## Log Chi Tiet

````text
$testOutputFormatted
````
"@
    $utf8NoBom = New-Object System.Text.UTF8Encoding $false
    [System.IO.File]::WriteAllText($reportFile, $simpleReport, $utf8NoBom)
    Write-Host "  [OK] Bao cao da duoc luu tai: $reportFile" -ForegroundColor Green
}

# ============================================
# HIỂN THỊ KẾT QUẢ
# ============================================
Write-Host ""
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  KET QUA TEST" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "  Tong so test: $totalTests" -ForegroundColor White
Write-Host "  Passed: $passedTests" -ForegroundColor Green
Write-Host "  Failed: $failedTests" -ForegroundColor $(if ($failedTests -gt 0) { "Red" } else { "Green" })

if ($totalTests -gt 0) {
    Write-Host "  Pass Rate: $passRate%" -ForegroundColor $(if ($passRate -eq 100) { "Green" } else { "Yellow" })
}

Write-Host "  Bao cao: $reportFile" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

exit $testExitCode

