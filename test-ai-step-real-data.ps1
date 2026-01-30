# Script tạo AI Step với dữ liệu thật - có input/output schema đầy đủ
$baseUrl = "http://localhost:8080"
$token = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJ1c2VySWQiOiI2OTVmN2IzOGNiZjYyZGJhMGZiMDk0Y2IiLCJ0aW1lIjoiNjk2NDU4ZGEiLCJyYW5kb21OdW1iZXIiOiI3NyJ9.GumBzdYurrNOB-hVVjCbSYp0Na8E7hdMQg8XpLO6g6k"
$headers = @{
    "Authorization" = "Bearer $token"
    "Content-Type" = "application/json"
}

function Invoke-ApiRequest {
    param(
        [string]$Method,
        [string]$Url,
        [object]$Body = $null,
        [string]$ActiveRoleID = $null
    )
    
    $requestHeaders = $headers.Clone()
    if ($ActiveRoleID) {
        $requestHeaders["X-Active-Role-ID"] = $activeRoleID
    }
    
    $params = @{
        Method = $Method
        Uri = $Url
        Headers = $requestHeaders
    }
    
    if ($Body) {
        $params.Body = ($Body | ConvertTo-Json -Depth 10)
    }
    
    try {
        $response = Invoke-RestMethod @params
        return @{
            Success = $true
            Data = $response
            Error = $null
        }
    } catch {
        $errorResponse = $_.ErrorDetails.Message | ConvertFrom-Json -ErrorAction SilentlyContinue
        return @{
            Success = $false
            Data = $null
            Error = if ($errorResponse) { $errorResponse } else { $_.Exception.Message }
        }
    }
}

Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TẠO AI STEP VỚI DỮ LIỆU THẬT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""

# Lấy role ID
$rolesResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/auth/roles"
if (-not $rolesResult.Success) {
    Write-Host "✗ Không thể lấy roles" -ForegroundColor Red
    exit 1
}
$activeRoleID = $rolesResult.Data.data[0].roleId
Write-Host "Role ID: $activeRoleID" -ForegroundColor Gray
Write-Host ""

# Kiểm tra step hiện tại
Write-Host "[1] Kiểm tra step hiện tại..." -ForegroundColor Yellow
$stepsResult = Invoke-ApiRequest -Method "GET" -Url "$baseUrl/api/v1/ai/steps/find" -ActiveRoleID $activeRoleID
if ($stepsResult.Success -and $stepsResult.Data.data.Count -gt 0) {
    $existingStep = $stepsResult.Data.data[0]
    Write-Host "✓ Tìm thấy step: $($existingStep.name)" -ForegroundColor Green
    Write-Host "  ID: $($existingStep.id)" -ForegroundColor Gray
    Write-Host "  Type: $($existingStep.type)" -ForegroundColor Gray
    
    if ($existingStep.inputSchema) {
        Write-Host "  ✓ Có InputSchema" -ForegroundColor Green
        Write-Host "    InputSchema: $($existingStep.inputSchema | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
    } else {
        Write-Host "  ✗ Chưa có InputSchema" -ForegroundColor Red
    }
    
    if ($existingStep.outputSchema) {
        Write-Host "  ✓ Có OutputSchema" -ForegroundColor Green
        Write-Host "    OutputSchema: $($existingStep.outputSchema | ConvertTo-Json -Depth 3)" -ForegroundColor Gray
    } else {
        Write-Host "  ✗ Chưa có OutputSchema" -ForegroundColor Red
    }
} else {
    Write-Host "⚠ Không có step nào" -ForegroundColor Yellow
}
Write-Host ""

# Tạo Step GENERATE với schema chi tiết
Write-Host "[2] Tạo Step GENERATE với input/output schema chi tiết..." -ForegroundColor Yellow
$generateStepData = @{
    name = "Generate Content - Pillar L1"
    description = "Step để generate content cho Pillar L1 từ thông tin pillar và context"
    type = "GENERATE"
    inputSchema = @{
        type = "object"
        required = @("pillarId", "pillarName", "targetAudience")
        properties = @{
            pillarId = @{
                type = "string"
                description = "ID của pillar cần generate content"
            }
            pillarName = @{
                type = "string"
                description = "Tên của pillar"
            }
            pillarDescription = @{
                type = "string"
                description = "Mô tả của pillar"
            }
            targetAudience = @{
                type = "string"
                description = "Đối tượng mục tiêu"
                enum = @("B2B", "B2C", "B2B2C")
            }
            context = @{
                type = "object"
                description = "Context bổ sung cho việc generate"
                properties = @{
                    industry = @{
                        type = "string"
                        description = "Ngành nghề"
                    }
                    productType = @{
                        type = "string"
                        description = "Loại sản phẩm"
                    }
                    tone = @{
                        type = "string"
                        description = "Tone của content"
                        enum = @("professional", "casual", "friendly", "formal")
                    }
                }
            }
            numberOfCandidates = @{
                type = "integer"
                description = "Số lượng candidates cần generate"
                minimum = 1
                maximum = 10
                default = 3
            }
        }
    }
    outputSchema = @{
        type = "object"
        required = @("candidates", "generatedAt")
        properties = @{
            candidates = @{
                type = "array"
                description = "Danh sách các content candidates đã được generate"
                items = @{
                    type = "object"
                    properties = @{
                        content = @{
                            type = "string"
                            description = "Nội dung của candidate"
                        }
                        title = @{
                            type = "string"
                            description = "Tiêu đề của candidate"
                        }
                        summary = @{
                            type = "string"
                            description = "Tóm tắt của candidate"
                        }
                        metadata = @{
                            type = "object"
                            description = "Metadata bổ sung"
                            properties = @{
                                wordCount = @{
                                    type = "integer"
                                }
                                language = @{
                                    type = "string"
                                }
                                tone = @{
                                    type = "string"
                                }
                            }
                        }
                    }
                }
            }
            generatedAt = @{
                type = "string"
                format = "date-time"
                description = "Thời gian generate"
            }
            model = @{
                type = "string"
                description = "Model AI đã sử dụng"
            }
            tokens = @{
                type = "object"
                description = "Thông tin về tokens đã sử dụng"
                properties = @{
                    input = @{
                        type = "integer"
                    }
                    output = @{
                        type = "integer"
                    }
                    total = @{
                        type = "integer"
                    }
                }
            }
        }
    }
    targetLevel = "L1"
    status = "active"
    metadata = @{
        category = "content-generation"
        version = "1.0.0"
        author = "system"
    }
}

$createGenerateStep = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $generateStepData -ActiveRoleID $activeRoleID
if ($createGenerateStep.Success) {
    $generateStepId = $createGenerateStep.Data.data.id
    Write-Host "✓ Đã tạo Generate Step với ID: $generateStepId" -ForegroundColor Green
    Write-Host "  Name: $($createGenerateStep.Data.data.name)" -ForegroundColor Gray
    Write-Host "  InputSchema: Có $($createGenerateStep.Data.data.inputSchema.properties.Count) properties" -ForegroundColor Gray
    Write-Host "  OutputSchema: Có $($createGenerateStep.Data.data.outputSchema.properties.Count) properties" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi khi tạo Generate Step: $($createGenerateStep.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Tạo Step JUDGE với schema chi tiết
Write-Host "[3] Tạo Step JUDGE với input/output schema chi tiết..." -ForegroundColor Yellow
$judgeStepData = @{
    name = "Judge Content Quality"
    description = "Step để đánh giá chất lượng và điểm số của các content candidates"
    type = "JUDGE"
    inputSchema = @{
        type = "object"
        required = @("candidates", "criteria")
        properties = @{
            candidates = @{
                type = "array"
                description = "Danh sách candidates cần đánh giá"
                items = @{
                    type = "object"
                    properties = @{
                        candidateId = @{
                            type = "string"
                        }
                        content = @{
                            type = "string"
                        }
                        title = @{
                            type = "string"
                        }
                    }
                }
            }
            criteria = @{
                type = "object"
                description = "Tiêu chí đánh giá"
                properties = @{
                    relevance = @{
                        type = "number"
                        description = "Độ liên quan (0-10)"
                        minimum = 0
                        maximum = 10
                    }
                    clarity = @{
                        type = "number"
                        description = "Độ rõ ràng (0-10)"
                        minimum = 0
                        maximum = 10
                    }
                    engagement = @{
                        type = "number"
                        description = "Độ hấp dẫn (0-10)"
                        minimum = 0
                        maximum = 10
                    }
                    accuracy = @{
                        type = "number"
                        description = "Độ chính xác (0-10)"
                        minimum = 0
                        maximum = 10
                    }
                }
            }
            context = @{
                type = "object"
                description = "Context để đánh giá"
                properties = @{
                    targetAudience = @{
                        type = "string"
                    }
                    industry = @{
                        type = "string"
                    }
                }
            }
        }
    }
    outputSchema = @{
        type = "object"
        required = @("scores", "rankings", "judgedAt")
        properties = @{
            scores = @{
                type = "array"
                description = "Điểm số của từng candidate"
                items = @{
                    type = "object"
                    properties = @{
                        candidateId = @{
                            type = "string"
                        }
                        overallScore = @{
                            type = "number"
                            description = "Điểm tổng thể (0-10)"
                        }
                        criteriaScores = @{
                            type = "object"
                            properties = @{
                                relevance = @{
                                    type = "number"
                                }
                                clarity = @{
                                    type = "number"
                                }
                                engagement = @{
                                    type = "number"
                                }
                                accuracy = @{
                                    type = "number"
                                }
                            }
                        }
                        feedback = @{
                            type = "string"
                            description = "Nhận xét về candidate"
                        }
                    }
                }
            }
            rankings = @{
                type = "array"
                description = "Xếp hạng các candidates theo điểm số"
                items = @{
                    type = "object"
                    properties = @{
                        rank = @{
                            type = "integer"
                        }
                        candidateId = @{
                            type = "string"
                        }
                        score = @{
                            type = "number"
                        }
                    }
                }
            }
            bestCandidate = @{
                type = "object"
                description = "Candidate tốt nhất"
                properties = @{
                    candidateId = @{
                        type = "string"
                    }
                    score = @{
                        type = "number"
                    }
                    reason = @{
                        type = "string"
                    }
                }
            }
            judgedAt = @{
                type = "string"
                format = "date-time"
            }
        }
    }
    targetLevel = "L1"
    status = "active"
    metadata = @{
        category = "content-judging"
        version = "1.0.0"
        author = "system"
    }
}

$createJudgeStep = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $judgeStepData -ActiveRoleID $activeRoleID
if ($createJudgeStep.Success) {
    $judgeStepId = $createJudgeStep.Data.data.id
    Write-Host "✓ Đã tạo Judge Step với ID: $judgeStepId" -ForegroundColor Green
    Write-Host "  Name: $($createJudgeStep.Data.data.name)" -ForegroundColor Gray
    Write-Host "  InputSchema: Có $($createJudgeStep.Data.data.inputSchema.properties.Count) properties" -ForegroundColor Gray
    Write-Host "  OutputSchema: Có $($createJudgeStep.Data.data.outputSchema.properties.Count) properties" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi khi tạo Judge Step: $($createJudgeStep.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Tạo Workflow với 2 steps
Write-Host "[4] Tạo Workflow với 2 steps (GENERATE + JUDGE)..." -ForegroundColor Yellow
$workflowData = @{
    name = "Content Generation & Quality Check Workflow"
    description = "Workflow để generate content và đánh giá chất lượng"
    version = "1.0.0"
    steps = @(
        @{
            stepId = $generateStepId
            order = 0
            policy = @{
                retryCount = 2
                timeout = 120
                onFailure = "stop"
                onSuccess = "continue"
                parallel = $false
            }
        },
        @{
            stepId = $judgeStepId
            order = 1
            policy = @{
                retryCount = 1
                timeout = 60
                onFailure = "continue"
                onSuccess = "continue"
                parallel = $false
            }
        }
    )
    rootRefType = "pillar"
    targetLevel = "L1"
    defaultPolicy = @{
        retryCount = 2
        timeout = 90
        onFailure = "continue"
        onSuccess = "continue"
        parallel = $false
    }
    status = "active"
    metadata = @{
        category = "content-workflow"
        version = "1.0.0"
        createdBy = "test-script"
    }
}

$createWorkflow = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/workflows/insert-one" -Body $workflowData -ActiveRoleID $activeRoleID
if ($createWorkflow.Success) {
    $workflowId = $createWorkflow.Data.data.id
    Write-Host "✓ Đã tạo Workflow với ID: $workflowId" -ForegroundColor Green
    Write-Host "  Name: $($createWorkflow.Data.data.name)" -ForegroundColor Gray
    Write-Host "  Steps: $($createWorkflow.Data.data.steps.Count)" -ForegroundColor Gray
    Write-Host "  OwnerOrganizationID: $($createWorkflow.Data.data.ownerOrganizationId)" -ForegroundColor Gray
} else {
    Write-Host "✗ Lỗi khi tạo Workflow: $($createWorkflow.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# Hiển thị tổng kết
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TỔNG KẾT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "✓ Generate Step ID: $generateStepId" -ForegroundColor Green
Write-Host "  - Input: pillarId, pillarName, targetAudience, context, numberOfCandidates" -ForegroundColor Gray
Write-Host "  - Output: candidates[], generatedAt, model, tokens" -ForegroundColor Gray
Write-Host ""
Write-Host "✓ Judge Step ID: $judgeStepId" -ForegroundColor Green
Write-Host "  - Input: candidates[], criteria, context" -ForegroundColor Gray
Write-Host "  - Output: scores[], rankings[], bestCandidate, judgedAt" -ForegroundColor Gray
Write-Host ""
Write-Host "✓ Workflow ID: $workflowId" -ForegroundColor Green
Write-Host "  - Steps: 2 (GENERATE → JUDGE)" -ForegroundColor Gray
Write-Host ""
Write-Host "Tất cả dữ liệu đã được lưu vào database!" -ForegroundColor Yellow
