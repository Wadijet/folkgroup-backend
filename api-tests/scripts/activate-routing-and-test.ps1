# Script bật routing rule và test notification routing
# Sử dụng: .\scripts\activate-routing-and-test.ps1 -Token "your_token_here"

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

Write-Host "`n[ACTIVATE] BAT ROUTING RULE VA TEST" -ForegroundColor Magenta
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
# BƯỚC 1: Tìm và bật routing rules
# ============================================
Write-Host "`n[STEP 1] Tim va bat routing rules" -ForegroundColor Yellow

try {
    $routingResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find" -Method GET -Headers $headers
    
    if ($routingResponse.data -and $routingResponse.data.Count -gt 0) {
        Write-Success "Tim thay $($routingResponse.data.Count) routing rules"
        
        # Tìm các routing rules có eventType bắt đầu bằng "test_telegram_notification"
        $testRules = $routingResponse.data | Where-Object { 
            $_.eventType -and $_.eventType.StartsWith("test_telegram_notification")
        }
        
        if ($testRules.Count -gt 0) {
            Write-Info "Tim thay $($testRules.Count) routing rule(s) cho test"
            
            foreach ($rule in $testRules) {
                Write-Host "`n   Routing Rule:" -ForegroundColor Gray
                Write-Host "     ID: $($rule.id)" -ForegroundColor DarkGray
                Write-Host "     EventType: $($rule.eventType)" -ForegroundColor DarkGray
                Write-Host "     IsActive: $($rule.isActive)" -ForegroundColor $(if ($rule.isActive) { "Green" } else { "Red" })
                
                # Kiểm tra và cập nhật routing rule
                $needsUpdate = $false
                $updateData = @{}
                
                if (-not $rule.isActive) {
                    $updateData["isActive"] = $true
                    $needsUpdate = $true
                }
                
                # Kiểm tra ChannelTypes
                if (-not $rule.channelTypes -or $rule.channelTypes.Count -eq 0) {
                    $updateData["channelTypes"] = @("telegram")
                    $needsUpdate = $true
                    Write-Info "Routing rule chua co ChannelTypes, se them 'telegram'"
                }
                
                if ($needsUpdate) {
                    Write-Info "Dang cap nhat routing rule: $($rule.id)"
                    
                    try {
                        $updatePayload = $updateData | ConvertTo-Json -Depth 10
                        
                        $updateResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/update-by-id/$($rule.id)" -Method PUT -Headers $headers -Body $updatePayload
                        
                        if ($updateResponse.data) {
                            Write-Success "Da cap nhat routing rule thanh cong!"
                            if ($updateResponse.data.isActive) {
                                Write-Host "   IsActive: $($updateResponse.data.isActive)" -ForegroundColor Green
                            }
                            if ($updateResponse.data.channelTypes) {
                                Write-Host "   ChannelTypes: $($updateResponse.data.channelTypes -join ', ')" -ForegroundColor Green
                            }
                        } else {
                            Write-Warning "Khong the cap nhat routing rule"
                        }
                    } catch {
                        Write-ErrorMsg "Loi khi cap nhat routing rule: $($_.Exception.Message)"
                    }
                } else {
                    Write-Success "Routing rule da duoc cau hinh dung"
                }
            }
            
            # Lấy eventType đầu tiên để test
            $testEventType = $testRules[0].eventType
            Write-Info "Se test voi eventType: $testEventType"
            
            # Đợi một chút để MongoDB cập nhật
            Start-Sleep -Seconds 2
            
            # ============================================
            # BƯỚC 2: Test trigger notification
            # ============================================
            Write-Host "`n[STEP 2] Test trigger notification" -ForegroundColor Yellow
            
            $testMessage = "Test Notification qua Routing System (Cach 2)
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
            try {
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
                    
                    # Debug: Kiểm tra lại routing rule
                    Write-Host "`n[DEBUG] Kiem tra lai routing rule sau khi bat:" -ForegroundColor Cyan
                    $checkResponse = Invoke-RestMethod -Uri "$BaseURL/notification/routing/find-by-id/$($testRules[0].id)" -Method GET -Headers $headers
                    if ($checkResponse.data) {
                        Write-Host "   ID: $($checkResponse.data.id)" -ForegroundColor DarkGray
                        Write-Host "   EventType: $($checkResponse.data.eventType)" -ForegroundColor DarkGray
                        Write-Host "   IsActive: $($checkResponse.data.isActive)" -ForegroundColor $(if ($checkResponse.data.isActive) { "Green" } else { "Red" })
                        Write-Host "   OrganizationIDs: $($checkResponse.data.organizationIds.Count)" -ForegroundColor DarkGray
                        Write-Host "   ChannelTypes: $($checkResponse.data.channelTypes -join ', ')" -ForegroundColor DarkGray
                    }
                }
            } catch {
                Write-ErrorMsg "Loi khi trigger notification: $($_.Exception.Message)"
                if ($_.ErrorDetails.Message) {
                    $errorDetail = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
                    if ($errorDetail) {
                        Write-Host "   Message: $($errorDetail.message)" -ForegroundColor Red
                    }
                }
            }
        } else {
            Write-Warning "Khong tim thay routing rule nao cho test"
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
Write-Host "[DONE] HOAN THANH" -ForegroundColor Magenta
Write-Host ("=" * 70) -ForegroundColor Magenta
