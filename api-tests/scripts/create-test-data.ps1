# Script Tạo Data Test với Admin Token
# Sử dụng: .\create-test-data.ps1

$adminToken = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTY2YzkwMGNiZjYyZGJhMGZjYWZkNGMiLCJ0aW1lIjoiNjk2NmM5MDAiLCJyYW5kb21OdW1iZXIiOiI1OCJ9.FflKAynO-2ArrbKWqTgRIAqIyQ13PrvHpjeB37E7MZI"
$baseURL = "http://localhost:8080/api/v1"

$headers = @{
    "Authorization" = "Bearer $adminToken"
    "Content-Type" = "application/json"
}

# Lay role ID de set active role
Write-Host "`n=== SETUP ===" -ForegroundColor Yellow
try {
    $roleResp = Invoke-RestMethod -Uri "$baseURL/auth/roles" -Method GET -Headers $headers -ErrorAction Stop
    if ($roleResp.data -and $roleResp.data.Count -gt 0) {
        $roleID = $roleResp.data[0].roleId
        $headers["X-Active-Role-ID"] = $roleID
        Write-Host "Da set active role: $roleID" -ForegroundColor Green
    }
}
catch {
    Write-Host "Khong the lay role ID" -ForegroundColor Red
    exit 1
}

# Lay organization ID
try {
    $orgResp = Invoke-RestMethod -Uri "$baseURL/organization/find-one" -Method GET -Headers $headers -ErrorAction Stop
    if ($orgResp.data) {
        $orgID = $orgResp.data.organizationId
        if (-not $orgID) {
            $orgID = $orgResp.data._id
        }
        Write-Host "Organization ID: $orgID" -ForegroundColor Green
    } else {
        Write-Host "Khong co organization data" -ForegroundColor Red
        exit 1
    }
}
catch {
    Write-Host "Khong the lay organization ID: $($_.Exception.Message)" -ForegroundColor Red
    exit 1
}

Write-Host "`n=== TAO DATA TEST ===" -ForegroundColor Yellow

# 1. Tao Content Node
Write-Host "`n1. Tao Content Node..." -ForegroundColor Cyan
$contentNodeBody = @{
    type = "pillar"
    text = "Test Content Node Text"
    name = "Test Content Node"
} | ConvertTo-Json -Depth 10

try {
    $nodeResp = Invoke-RestMethod -Uri "$baseURL/content/nodes/insert-one" -Method POST -Headers $headers -Body $contentNodeBody -ErrorAction Stop
    if ($nodeResp.data) {
        $nodeID = $nodeResp.data._id
        if (-not $nodeID) {
            $nodeID = $nodeResp.data.id
        }
        Write-Host "  Content Node ID: $nodeID" -ForegroundColor Green
    } else {
        Write-Host "  Khong co data tra ve" -ForegroundColor Red
        $nodeID = $null
    }
}
catch {
    $errorDetail = $_.Exception.Response
    if ($errorDetail) {
        try {
            $reader = New-Object System.IO.StreamReader($errorDetail.GetResponseStream())
            $responseBody = $reader.ReadToEnd()
            Write-Host "  Loi khi tao content node: $responseBody" -ForegroundColor Red
        } catch {
            Write-Host "  Loi khi tao content node: $($_.Exception.Message)" -ForegroundColor Red
        }
    } else {
        Write-Host "  Loi khi tao content node: $($_.Exception.Message)" -ForegroundColor Red
    }
    $nodeID = $null
}

# 2. Tao Draft Content Node
Write-Host "`n2. Tao Draft Content Node..." -ForegroundColor Cyan
if ($nodeID) {
    $draftNodeBody = @{
        type = "pillar"
        text = "Test Draft Node Text"
        name = "Test Draft Node"
        parentId = $nodeID
    } | ConvertTo-Json -Depth 10

    try {
        $draftResp = Invoke-RestMethod -Uri "$baseURL/content/drafts/nodes/insert-one" -Method POST -Headers $headers -Body $draftNodeBody -ErrorAction Stop
        $draftNodeID = $draftResp.data._id
        Write-Host "  Draft Node ID: $draftNodeID" -ForegroundColor Green
    }
    catch {
        Write-Host "  Loi khi tao draft node: $($_.Exception.Message)" -ForegroundColor Red
        $draftNodeID = $null
    }
} else {
    $draftNodeID = $null
    Write-Host "  Bo qua (can content node truoc)" -ForegroundColor Yellow
}

# 3. Tao AI Workflow Command (can workflowId va stepId, bo qua neu khong co)
Write-Host "`n3. Tao AI Workflow Command..." -ForegroundColor Cyan
Write-Host "  Bo qua (can workflowId va stepId)" -ForegroundColor Yellow
$commandID = $null

# 4. Tao Agent Command
Write-Host "`n4. Tao Agent Command..." -ForegroundColor Cyan
$agentCommandBody = @{
    agentId = "test-agent-001"
    commandType = "execute"
    status = "pending"
    ownerOrganizationId = $orgID
} | ConvertTo-Json -Depth 10

try {
    $agentCmdResp = Invoke-RestMethod -Uri "$baseURL/agent-management/command/insert-one" -Method POST -Headers $headers -Body $agentCommandBody -ErrorAction Stop
    $agentCommandID = $agentCmdResp.data._id
    Write-Host "  Agent Command ID: $agentCommandID" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi tao agent command: $($_.Exception.Message)" -ForegroundColor Red
    $agentCommandID = $null
}

# 5. Tao Agent Config
Write-Host "`n5. Tao Agent Config..." -ForegroundColor Cyan
$agentConfigBody = @{
    agentId = "test-agent-001"
    configData = @{
        test = "value"
    }
    ownerOrganizationId = $orgID
} | ConvertTo-Json -Depth 10

try {
    $configResp = Invoke-RestMethod -Uri "$baseURL/agent-management/config/insert-one" -Method POST -Headers $headers -Body $agentConfigBody -ErrorAction Stop
    $configID = $configResp.data._id
    Write-Host "  Agent Config ID: $configID" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi tao agent config: $($_.Exception.Message)" -ForegroundColor Red
    $configID = $null
}

# 6. Tao Notification Channel (de test delivery/send)
Write-Host "`n6. Tao Notification Channel..." -ForegroundColor Cyan
$channelBody = @{
    name = "Test Email Channel"
    channelType = "email"
    status = "active"
    ownerOrganizationId = $orgID
    config = @{
        smtpHost = "smtp.test.com"
        smtpPort = 587
    }
} | ConvertTo-Json -Depth 10

try {
    $channelResp = Invoke-RestMethod -Uri "$baseURL/notification/channel/insert-one" -Method POST -Headers $headers -Body $channelBody -ErrorAction Stop
    $channelID = $channelResp.data._id
    Write-Host "  Channel ID: $channelID" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi tao channel: $($_.Exception.Message)" -ForegroundColor Red
    $channelID = $null
}

# 7. Tao User (de test block/unblock)
Write-Host "`n7. Lay User ID de test block/unblock..." -ForegroundColor Cyan
try {
    $userResp = Invoke-RestMethod -Uri "$baseURL/user/find-one" -Method GET -Headers $headers -ErrorAction Stop
    $userID = $userResp.data.userId
    Write-Host "  User ID: $userID" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi lay user: $($_.Exception.Message)" -ForegroundColor Red
    $userID = $null
}

# 8. Tao Role (de test update role permissions)
Write-Host "`n8. Lay Role ID de test update permissions..." -ForegroundColor Cyan
try {
    $roleResp2 = Invoke-RestMethod -Uri "$baseURL/role/find-one" -Method GET -Headers $headers -ErrorAction Stop
    $testRoleID = $roleResp2.data.roleId
    Write-Host "  Role ID: $testRoleID" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi lay role: $($_.Exception.Message)" -ForegroundColor Red
    $testRoleID = $null
}

# 9. Tao Facebook Message Item (de test find-by-message-id)
Write-Host "`n9. Tao Facebook Message Item..." -ForegroundColor Cyan
$fbMessageItemBody = @{
    messageId = "test-message-123"
    conversationId = "test-conv-123"
    content = "Test message"
    ownerOrganizationId = $orgID
} | ConvertTo-Json -Depth 10

try {
    $fbMsgResp = Invoke-RestMethod -Uri "$baseURL/facebook/message-item/insert-one" -Method POST -Headers $headers -Body $fbMessageItemBody -ErrorAction Stop
    $fbMessageID = $fbMsgResp.data._id
    $fbMessageItemMessageId = "test-message-123"
    Write-Host "  Facebook Message Item ID: $fbMessageID" -ForegroundColor Green
    Write-Host "  Message ID: $fbMessageItemMessageId" -ForegroundColor Green
}
catch {
    Write-Host "  Loi khi tao facebook message item: $($_.Exception.Message)" -ForegroundColor Red
    $fbMessageID = $null
    $fbMessageItemMessageId = $null
}

# Luu ket qua vao file
$testData = @{
    nodeID = $nodeID
    draftNodeID = $draftNodeID
    commandID = $commandID
    agentCommandID = $agentCommandID
    configID = $configID
    channelID = $channelID
    userID = $userID
    roleID = $testRoleID
    fbMessageID = $fbMessageID
    fbMessageItemMessageId = $fbMessageItemMessageId
    orgID = $orgID
}

$testData | ConvertTo-Json -Depth 10 | Out-File -FilePath "test-data.json" -Encoding UTF8

Write-Host "`n=== KET QUA ===" -ForegroundColor Yellow
Write-Host "Data da duoc luu vao: test-data.json" -ForegroundColor Green
Write-Host "`nDanh sach ID da tao:" -ForegroundColor Cyan
$testData | Format-List

Write-Host "`n=== HOAN TAT ===" -ForegroundColor Green
