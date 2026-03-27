# Nap bien moi truong tu development.env neu co (FIREBASE_API_KEY, ...)
# Va ham Get-ApiTestBearerToken: uu tien TEST_ADMIN_TOKEN; neu khong thi Firebase email/password -> POST /auth/login/firebase
# Dot-source: . (Join-Path $PSScriptRoot 'resolve-test-bearer-token.ps1')

# Thu muc chua file nay — dung trong ham vi $PSScriptRoot co the khong dung trong scope ham
$script:ApiTestsScriptsDir = if ($PSScriptRoot) { $PSScriptRoot } else { Split-Path -Parent $MyInvocation.MyCommand.Path }

function Import-DevEnvForApiTests {
    $root = Resolve-Path (Join-Path $script:ApiTestsScriptsDir '..\..')
    $envFile = Join-Path $root.Path 'api\config\env\development.env'
    if (-not (Test-Path $envFile)) { return }
    Get-Content $envFile | ForEach-Object {
        if ($_ -match '^\s*([^#][^=]+)=(.*)$') {
            $k = $matches[1].Trim()
            $v = $matches[2].Trim()
            if ($v -and -not [Environment]::GetEnvironmentVariable($k, 'Process')) {
                [Environment]::SetEnvironmentVariable($k, $v, 'Process')
            }
        }
    }
}

# Lay JWT cho goi API: TEST_ADMIN_TOKEN hoac Firebase + /auth/login/firebase
function Get-ApiTestBearerToken {
    param(
        [Parameter(Mandatory = $true)][string]$ApiBaseUrl,
        [string]$Hwid = 'api_test_script_hwid'
    )
    if ($env:TEST_ADMIN_TOKEN) {
        Write-Host 'Dung TEST_ADMIN_TOKEN tu moi truong' -ForegroundColor Gray
        return $env:TEST_ADMIN_TOKEN
    }

    Import-DevEnvForApiTests

    $email = if ($env:TEST_EMAIL) { $env:TEST_EMAIL } else { 'daomanhdung86@gmail.com' }
    $password = if ($env:TEST_PASSWORD) { $env:TEST_PASSWORD } else { '12345678' }

    $idToken = $env:TEST_FIREBASE_ID_TOKEN
    if (-not $idToken) {
        $apiKey = $env:FIREBASE_API_KEY
        if (-not $apiKey) {
            throw 'Can FIREBASE_API_KEY hoac TEST_FIREBASE_ID_TOKEN. Them vao api/config/env/development.env hoac set bien moi truong.'
        }
        $fbUrl = 'https://identitytoolkit.googleapis.com/v1/accounts:signInWithPassword?key=' + [uri]::EscapeDataString($apiKey)
        $fbBody = @{ email = $email; password = $password; returnSecureToken = $true } | ConvertTo-Json
        $fbResp = Invoke-RestMethod -Uri $fbUrl -Method Post -Body $fbBody -ContentType 'application/json' -ErrorAction Stop
        $idToken = $fbResp.idToken
        if (-not $idToken) { throw 'Firebase khong tra idToken' }
        Write-Host "Da lay Firebase idToken bang email: $email" -ForegroundColor Green
    }
    else {
        Write-Host 'Dung TEST_FIREBASE_ID_TOKEN tu moi truong' -ForegroundColor Gray
    }

    $loginUrl = ($ApiBaseUrl.TrimEnd('/') + '/auth/login/firebase')
    $loginBody = @{ idToken = $idToken; hwid = $Hwid } | ConvertTo-Json
    $loginResp = Invoke-RestMethod -Uri $loginUrl -Method Post -Body $loginBody -ContentType 'application/json' -ErrorAction Stop
    if (-not $loginResp.data.token) {
        throw 'Dang nhap API that bai: khong co data.token trong response'
    }
    Write-Host 'Dang nhap API thanh cong, co JWT' -ForegroundColor Green
    return [string]$loginResp.data.token
}
