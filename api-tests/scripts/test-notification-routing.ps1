# Script test notification routing (Cách 2)
# Sử dụng: .\scripts\test-notification-routing.ps1 -Token "your_token_here"

param(
    [Parameter(Mandatory=$true)]
    [string]$Token,
    
    [string]$BaseURL = "http://localhost:8080/api/v1"
)

# Màu sắc cho output
function Write-Success { param([string]$Message) Write-Host "[OK] $Message" -ForegroundColor Green }
function Write-ErrorMsg { param([string]$Message) Write-Host "[ERROR] $Message" -ForegroundColor Red }
function Write-Info { param([string]$Message) Write-Host "[INFO] $Message" -ForegroundColor Cyan }
function Write-Warning { param([string]$Message) Write-Host "[WARN] $Message" -ForegroundColor Yellow }

# Headers với token
$headers = @{
    "Authorization" = "Bearer $Token"
    "Content-Type" = "application/json"
}

Write-Host "`n[TEST] TEST NOTIFICATION ROUTING (CACH 2)" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta

# Lấy role ID
Write-Info "Lay role ID..."
$activeRoleID = $null
try {
    $roleResponse = Invoke-RestMethod -Uri "$BaseURL/auth/roles" -Method GET -Headers $headers
    if ($roleResponse.data -and $roleResponse.data.Count -gt 0) {
        $firstRole = $roleResponse.data[0]
        if ($firstRole.roleId) {
            $activeRoleID = $firstRole.roleId
            $headers["X-Active-Role-ID"] = $activeRoleID
            Write-Success "Da lay duoc role ID: $activeRoleID"
        }
    }
} catch {
    Write-ErrorMsg "Khong the lay role ID: $($_.Exception.Message)"
    exit 1
}

# ============================================
# BƯỚC 1: Kiểm tra routing rules có sẵn
# ============================================
Write-Host "`n[STEP 1] Kiem tra routing rules co san" -ForegroundColor Yellow

try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Success "Tim thay $($routingResponse.data.Count) routing rules"
        
        # Tìm routing rule có eventType và isActive = true
        $activeRules = $routingResponse.data | Where-Object { 
            $_.eventType -and $_.isActive -eq $true 
        }
        
        if ($activeRules.Count -gt 0) {
            Write-Info "Tim thay $($activeRules.Count) routing rules active co eventType"
            
            # Lấy eventType đầu tiên để test
            $testEventType = $activeRules[0].eventType
            Write-Info "Se test voi eventType: $testEventType"
            
            # Kiểm tra có template cho eventType này không
            Write-Host "`n[STEP 2] Kiem tra template cho eventType: $testEventType" -ForegroundColor Yellow
            $templateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/template/find" -Method GET -Headers $headers
            $telegramTemplates = $templateResponse.data | Where-Object { 
                $_.eventType -eq $testEventType -and $_.channelType -eq "telegram" 
            }
            
            if ($telegramTemplates.Count -gt 0) {
                Write-Success "Tim thay template cho eventType: $testEventType (telegram)"
            } else {
                Write-Warning "Khong tim thay template telegram cho eventType: $testEventType"
                Write-Info "Se thu tao template..."
                
                # Lấy organization ID
                $orgID = $roleResponse.data[0].organizationId
                
                # Tạo template
                $templatePayload = @{
                    name = "Test Template for $testEventType"
                    eventType = $testEventType
                    channelType = "telegram"
                    subject = "Test Notification"
                    content = "Test notification qua routing`nEventType: {{eventType}}`nThoi gian: {{timestamp}}"
                    organizationId = $orgID
                } | ConvertTo-Json -Depth 10
                
                try {
                    $newTemplate = Invoke-RestMethod -Uri "$BaseURL/notification/template/insert-one" -Method POST -Headers $headers -Body $templatePayload
                    Write-Success "Da tao template thanh cong"
                } catch {
                    Write-Warning "Khong the tao template: $($_.Exception.Message)"
                }
            }
            
            # ============================================
            # BƯỚC 3: Trigger notification
            # ============================================
            Write-Host "`n[STEP 3] Trigger notification voi eventType: $testEventType" -ForegroundColor Yellow
            
            $testMessage = "Test Notification qua Routing System
EventType: $testEventType
Thoi gian: $(Get-Date -Format 'yyyy-MM-dd HH:mm:ss')
Day la test gui qua he thong notification voi routing rule.
Neu ban nhan duoc tin nhan nay, he thong notification hoat dong tot!"
            
            $triggerPayload = @{
                eventType = $testEventType
                payload = @{
                    message = $testMessage
                    eventType = $testEventType
                    timestamp = (Get-Date).ToString("yyyy-MM-dd HH:mm:ss")
                    baseUrl = $BaseURL
                }
            } | ConvertTo-Json -Depth 10
            
            Write-Info "Dang trigger notification..."
            $triggerResponse = Invoke-RestMethod -Uri "$BaseURL/notification/trigger" -Method POST -Headers $headers -Body $triggerPayload
            
            if ($triggerResponse.queued -gt 0) {
                Write-Success "Da them vao queue thanh cong!"
                Write-Host "   EventType: $($triggerResponse.eventType)" -ForegroundColor Gray
                Write-Host "   Queued: $($triggerResponse.queued)" -ForegroundColor Gray
                Write-Host "   Message: $($triggerResponse.message)" -ForegroundColor Gray
                Write-Info "Vui long kiem tra Telegram trong vong 10-30 giay..."
            } else {
                Write-Warning "Khong co notification nao duoc queue"
                Write-Host "   Message: $($triggerResponse.message)" -ForegroundColor Gray
                Write-Host "   EventType: $($triggerResponse.eventType)" -ForegroundColor Gray
                
                # Debug: Kiểm tra lại
                Write-Host "`n[DEBUG] Kiem tra lai routing rule:" -ForegroundColor Cyan
                $rule = $activeRules[0]
                Write-Host "   ID: $($rule.id)" -ForegroundColor DarkGray
                Write-Host "   EventType: $($rule.eventType)" -ForegroundColor DarkGray
                Write-Host "   IsActive: $($rule.isActive)" -ForegroundColor DarkGray
                Write-Host "   OrganizationIDs: $($rule.organizationIds.Count)" -ForegroundColor DarkGray
                Write-Host "   ChannelTypes: $($rule.channelTypes -join ', ')" -ForegroundColor DarkGray
            }
        } else {
            Write-Warning "Khong tim thay routing rule nao co eventType va isActive = true"
            Write-Info "Danh sach routing rules:"
            foreach ($rule in $routingResponse.data | Select-Object -First 5) {
                Write-Host "   - EventType: $($rule.eventType), IsActive: $($rule.isActive)" -ForegroundColor DarkGray
            }
        }
    } else {
        Write-Warning "Khong co routing rule nao"
    }
} catch {
    Write-ErrorMsg "Loi: $($_.Exception.Message)"
}

Write-Host "`n" + ("=" * 70) -ForegroundColor Magenta
Write-Host "[DONE] HOAN THANH TEST" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
