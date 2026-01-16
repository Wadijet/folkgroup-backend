# Script de lay du lieu mau tu API va luu vao thu muc sample-data
# Su dung bearer token cua admin user

$baseUrl = "http://localhost:8080/api/v1"
$bearerToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVmN2IzOGNiZjYyZGJhMGZiMDk0Y2IiLCJ0aW1lIjoiNjk2NWM4Y2UiLCJyYW5kb21OdW1iZXIiOiIxOCJ9.dNBKLgP0Hb7BHiudUanQCI96ot1Sw4IM2TwoMPiAnOA"
$outputDir = "docs-shared/ai-context/folkform/sample-data"

# Tao thu muc output neu chua co
if (-not (Test-Path $outputDir)) {
    New-Item -ItemType Directory -Path $outputDir -Force | Out-Null
}

# Ham de goi API va lay du lieu
function Get-SampleData {
    param(
        [string]$Endpoint,
        [string]$OutputFile,
        [int]$Limit = 10
    )
    
    try {
        $filterJson = "{}"
        $optionsJson = '{"limit":' + $Limit + '}'
        $filterEncoded = [System.Uri]::EscapeDataString($filterJson)
        $optionsEncoded = [System.Uri]::EscapeDataString($optionsJson)
        $url = "$baseUrl/$Endpoint/find?filter=$filterEncoded&options=$optionsEncoded"
        Write-Host "Dang lay du lieu tu: $url" -ForegroundColor Cyan
        
        $headers = @{
            "Authorization" = "Bearer $bearerToken"
            "Content-Type" = "application/json"
        }
        
        $response = Invoke-RestMethod -Uri $url -Method Get -Headers $headers -ErrorAction Stop
        
        if ($response.status -eq "success" -and $response.data) {
            $data = $response.data
            if ($data.Count -gt 0) {
                $json = $data | ConvertTo-Json -Depth 20
                $outputPath = Join-Path $outputDir $OutputFile
                $json | Out-File -FilePath $outputPath -Encoding UTF8
                Write-Host "OK Da luu $($data.Count) ban ghi vao $OutputFile" -ForegroundColor Green
                return $true
            } else {
                Write-Host "WARNING Khong co du lieu trong $Endpoint" -ForegroundColor Yellow
                return $false
            }
        } else {
            Write-Host "ERROR Response khong hop le tu $Endpoint" -ForegroundColor Red
            Write-Host "  Response: $($response | ConvertTo-Json -Depth 5)" -ForegroundColor Red
            return $false
        }
    }
    catch {
        Write-Host "ERROR Loi khi lay du lieu tu $Endpoint : $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "  Response body: $responseBody" -ForegroundColor Red
        }
        return $false
    }
}

# Ham de lay du lieu voi pagination
function Get-SampleDataWithPagination {
    param(
        [string]$Endpoint,
        [string]$OutputFile,
        [int]$Limit = 10
    )
    
    try {
        $filterJson = "{}"
        $filterEncoded = [System.Uri]::EscapeDataString($filterJson)
        $url = "$baseUrl/$Endpoint/find-with-pagination?filter=$filterEncoded&page=1&limit=$Limit"
        Write-Host "Dang lay du lieu tu: $url" -ForegroundColor Cyan
        
        $headers = @{
            "Authorization" = "Bearer $bearerToken"
            "Content-Type" = "application/json"
        }
        
        $response = Invoke-RestMethod -Uri $url -Method Get -Headers $headers -ErrorAction Stop
        
        if ($response.status -eq "success" -and $response.data) {
            $data = $response.data
            if ($data.items -and $data.items.Count -gt 0) {
                $json = $data.items | ConvertTo-Json -Depth 20
                $outputPath = Join-Path $outputDir $OutputFile
                $json | Out-File -FilePath $outputPath -Encoding UTF8
                Write-Host "OK Da luu $($data.items.Count) ban ghi vao $OutputFile (Tong: $($data.total))" -ForegroundColor Green
                return $true
            } elseif ($data.Count -gt 0) {
                $json = $data | ConvertTo-Json -Depth 20
                $outputPath = Join-Path $outputDir $OutputFile
                $json | Out-File -FilePath $outputPath -Encoding UTF8
                Write-Host "OK Da luu $($data.Count) ban ghi vao $OutputFile" -ForegroundColor Green
                return $true
            } else {
                Write-Host "WARNING Khong co du lieu trong $Endpoint" -ForegroundColor Yellow
                return $false
            }
        } else {
            Write-Host "ERROR Response khong hop le tu $Endpoint" -ForegroundColor Red
            Write-Host "  Response: $($response | ConvertTo-Json -Depth 5)" -ForegroundColor Red
            return $false
        }
    }
    catch {
        Write-Host "ERROR Loi khi lay du lieu tu $Endpoint : $($_.Exception.Message)" -ForegroundColor Red
        if ($_.Exception.Response) {
            $reader = New-Object System.IO.StreamReader($_.Exception.Response.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "  Response body: $responseBody" -ForegroundColor Red
        }
        return $false
    }
}

Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Bat dau lay du lieu mau tu API" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
Write-Host ""

# Danh sach cac collections can lay du lieu mau
$collections = @(
    # Auth & RBAC
    @{ Endpoint = "user"; OutputFile = "users-sample.json"; UsePagination = $false },
    @{ Endpoint = "role"; OutputFile = "roles-sample.json"; UsePagination = $false },
    @{ Endpoint = "permission"; OutputFile = "permissions-sample.json"; UsePagination = $false },
    @{ Endpoint = "role-permission"; OutputFile = "role-permissions-sample.json"; UsePagination = $false },
    @{ Endpoint = "user-role"; OutputFile = "user-roles-sample.json"; UsePagination = $false },
    @{ Endpoint = "organization"; OutputFile = "organizations-sample.json"; UsePagination = $false },
    @{ Endpoint = "organization-share"; OutputFile = "organization-shares-sample.json"; UsePagination = $false },
    
    # Facebook Integration
    @{ Endpoint = "facebook/page"; OutputFile = "fb-pages-sample.json"; UsePagination = $false },
    @{ Endpoint = "facebook/post"; OutputFile = "fb-posts-sample.json"; UsePagination = $false },
    @{ Endpoint = "facebook/conversation"; OutputFile = "fb-conversations-sample.json"; UsePagination = $false },
    @{ Endpoint = "facebook/message"; OutputFile = "fb-messages-sample.json"; UsePagination = $false },
    @{ Endpoint = "facebook/message-item"; OutputFile = "fb-message-items-sample.json"; UsePagination = $false },
    @{ Endpoint = "fb-customer"; OutputFile = "fb-customers-sample.json"; UsePagination = $false },
    
    # Customers
    @{ Endpoint = "customer"; OutputFile = "customers-sample.json"; UsePagination = $false },
    @{ Endpoint = "pc-pos-customer"; OutputFile = "pc-pos-customers-sample.json"; UsePagination = $false },
    
    # Pancake POS
    @{ Endpoint = "pancake-pos/shop"; OutputFile = "pc-pos-shops-sample.json"; UsePagination = $false },
    @{ Endpoint = "pancake-pos/warehouse"; OutputFile = "pc-pos-warehouses-sample.json"; UsePagination = $false },
    @{ Endpoint = "pancake-pos/product"; OutputFile = "pc-pos-products-sample.json"; UsePagination = $false },
    @{ Endpoint = "pancake-pos/variation"; OutputFile = "pc-pos-variations-sample.json"; UsePagination = $false },
    @{ Endpoint = "pancake-pos/category"; OutputFile = "pc-pos-categories-sample.json"; UsePagination = $false },
    @{ Endpoint = "pancake-pos/order"; OutputFile = "pc-pos-orders-sample.json"; UsePagination = $false },
    @{ Endpoint = "pc-order"; OutputFile = "pc-orders-sample.json"; UsePagination = $false },
    
    # Content Storage (Module 1)
    @{ Endpoint = "content/nodes"; OutputFile = "content-nodes-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/videos"; OutputFile = "content-videos-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/publications"; OutputFile = "content-publications-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/drafts/nodes"; OutputFile = "content-draft-nodes-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/drafts/videos"; OutputFile = "content-draft-videos-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/drafts/publications"; OutputFile = "content-draft-publications-sample.json"; UsePagination = $false },
    @{ Endpoint = "content/drafts/approvals"; OutputFile = "content-draft-approvals-sample.json"; UsePagination = $false },
    
    # AI Service (Module 2)
    @{ Endpoint = "ai/workflows"; OutputFile = "ai-workflows-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/steps"; OutputFile = "ai-steps-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/prompt-templates"; OutputFile = "ai-prompt-templates-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/provider-profiles"; OutputFile = "ai-provider-profiles-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/workflow-runs"; OutputFile = "ai-workflow-runs-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/step-runs"; OutputFile = "ai-step-runs-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/generation-batches"; OutputFile = "ai-generation-batches-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/candidates"; OutputFile = "ai-candidates-sample.json"; UsePagination = $false },
    @{ Endpoint = "ai/workflow-commands"; OutputFile = "ai-workflow-commands-sample.json"; UsePagination = $false },
    
    # Notification System
    @{ Endpoint = "notification/channel"; OutputFile = "notification-channels-sample.json"; UsePagination = $false },
    @{ Endpoint = "notification/template"; OutputFile = "notification-templates-sample.json"; UsePagination = $false },
    @{ Endpoint = "notification/sender"; OutputFile = "notification-senders-sample.json"; UsePagination = $false },
    @{ Endpoint = "notification/routing-rule"; OutputFile = "notification-routing-rules-sample.json"; UsePagination = $false },
    
    # Delivery System
    @{ Endpoint = "delivery/history"; OutputFile = "delivery-history-sample.json"; UsePagination = $true },
    @{ Endpoint = "delivery/queue"; OutputFile = "delivery-queue-sample.json"; UsePagination = $false },
    
    # CTA Module
    @{ Endpoint = "cta/library"; OutputFile = "cta-library-sample.json"; UsePagination = $false },
    @{ Endpoint = "cta/tracking"; OutputFile = "cta-tracking-sample.json"; UsePagination = $false },
    
    # Agent Management
    @{ Endpoint = "agent-management/registry"; OutputFile = "agent-registry-sample.json"; UsePagination = $false },
    @{ Endpoint = "agent-management/config"; OutputFile = "agent-configs-sample.json"; UsePagination = $false },
    @{ Endpoint = "agent-management/command"; OutputFile = "agent-commands-sample.json"; UsePagination = $false },
    @{ Endpoint = "agent-management/activity"; OutputFile = "agent-activity-logs-sample.json"; UsePagination = $true },
    
    # Webhook Logs
    @{ Endpoint = "webhook-log"; OutputFile = "webhook-logs-sample.json"; UsePagination = $true },
    
    # Access Tokens
    @{ Endpoint = "access-token"; OutputFile = "access-tokens-sample.json"; UsePagination = $false }
)

$successCount = 0
$failCount = 0

foreach ($collection in $collections) {
    Write-Host ""
    if ($collection.UsePagination) {
        $result = Get-SampleDataWithPagination -Endpoint $collection.Endpoint -OutputFile $collection.OutputFile -Limit 10
    } else {
        $result = Get-SampleData -Endpoint $collection.Endpoint -OutputFile $collection.OutputFile -Limit 10
    }
    
    if ($result) {
        $successCount++
    } else {
        $failCount++
    }
    
    Start-Sleep -Milliseconds 200
}

Write-Host ""
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Hoan thanh lay du lieu mau" -ForegroundColor Magenta
Write-Host "========================================" -ForegroundColor Magenta
Write-Host "Thanh cong: $successCount" -ForegroundColor Green
Write-Host "That bai: $failCount" -ForegroundColor Red
Write-Host "Thu muc output: $outputDir" -ForegroundColor Cyan
