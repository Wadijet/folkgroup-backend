# Script tạo cả 3 loại AI Steps: GENERATE, JUDGE, STEP_GENERATION
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
Write-Host "TẠO CẢ 3 LOẠI AI STEPS" -ForegroundColor Cyan
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

# ========================================
# 1. GENERATE STEP
# ========================================
Write-Host "[1/3] Tạo Step GENERATE..." -ForegroundColor Yellow
$generateStepData = @{
    name = "Generate Content - Full Featured"
    description = "Step GENERATE để tạo content candidates với đầy đủ tính năng"
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
                description = "Context bổ sung"
                properties = @{
                    industry = @{
                        type = "string"
                    }
                    productType = @{
                        type = "string"
                    }
                    tone = @{
                        type = "string"
                        enum = @("professional", "casual", "friendly", "formal")
                    }
                }
            }
            numberOfCandidates = @{
                type = "integer"
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
                items = @{
                    type = "object"
                    properties = @{
                        content = @{
                            type = "string"
                        }
                        title = @{
                            type = "string"
                        }
                        summary = @{
                            type = "string"
                        }
                        metadata = @{
                            type = "object"
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
            }
            model = @{
                type = "string"
            }
            tokens = @{
                type = "object"
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
    }
}

$createGenerate = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $generateStepData -ActiveRoleID $activeRoleID
if ($createGenerate.Success) {
    $generateStepId = $createGenerate.Data.data.id
    Write-Host "✓ Đã tạo GENERATE Step - ID: $generateStepId" -ForegroundColor Green
} else {
    Write-Host "✗ Lỗi: $($createGenerate.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# ========================================
# 2. JUDGE STEP
# ========================================
Write-Host "[2/3] Tạo Step JUDGE..." -ForegroundColor Yellow
$judgeStepData = @{
    name = "Judge Content Quality - Full Featured"
    description = "Step JUDGE để đánh giá chất lượng content candidates"
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
                description = "Xếp hạng các candidates"
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
    }
}

$createJudge = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $judgeStepData -ActiveRoleID $activeRoleID
if ($createJudge.Success) {
    $judgeStepId = $createJudge.Data.data.id
    Write-Host "✓ Đã tạo JUDGE Step - ID: $judgeStepId" -ForegroundColor Green
} else {
    Write-Host "✗ Lỗi: $($createJudge.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# ========================================
# 3. STEP_GENERATION STEP
# ========================================
Write-Host "[3/3] Tạo Step STEP_GENERATION..." -ForegroundColor Yellow
$stepGenerationData = @{
    name = "Dynamic Step Generation"
    description = "Step STEP_GENERATION để tự động tạo các steps con dựa trên context và requirements"
    type = "STEP_GENERATION"
    inputSchema = @{
        type = "object"
        required = @("parentContext", "requirements", "targetLevel")
        properties = @{
            parentContext = @{
                type = "object"
                description = "Context từ parent pillar/step"
                properties = @{
                    pillarId = @{
                        type = "string"
                        description = "ID của parent pillar"
                    }
                    pillarName = @{
                        type = "string"
                    }
                    pillarType = @{
                        type = "string"
                        enum = @("L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8")
                    }
                    content = @{
                        type = "string"
                        description = "Nội dung của parent pillar"
                    }
                }
            }
            requirements = @{
                type = "object"
                description = "Yêu cầu cho việc generate steps"
                properties = @{
                    numberOfSteps = @{
                        type = "integer"
                        description = "Số lượng steps cần generate"
                        minimum = 1
                        maximum = 10
                        default = 3
                    }
                    stepTypes = @{
                        type = "array"
                        description = "Các loại steps muốn generate"
                        items = @{
                            type = "string"
                            enum = @("GENERATE", "JUDGE", "STEP_GENERATION")
                        }
                    }
                    focusAreas = @{
                        type = "array"
                        description = "Các lĩnh vực tập trung"
                        items = @{
                            type = "string"
                        }
                    }
                    complexity = @{
                        type = "string"
                        description = "Độ phức tạp"
                        enum = @("simple", "medium", "complex")
                    }
                }
            }
            targetLevel = @{
                type = "string"
                description = "Level mục tiêu cho các steps được generate"
                enum = @("L1", "L2", "L3", "L4", "L5", "L6", "L7", "L8")
            }
            constraints = @{
                type = "object"
                description = "Ràng buộc cho việc generate"
                properties = @{
                    maxExecutionTime = @{
                        type = "integer"
                        description = "Thời gian thực thi tối đa (seconds)"
                    }
                    requiredOutputs = @{
                        type = "array"
                        description = "Các outputs bắt buộc"
                        items = @{
                            type = "string"
                        }
                    }
                    excludedStepTypes = @{
                        type = "array"
                        description = "Các loại steps không muốn generate"
                        items = @{
                            type = "string"
                        }
                    }
                }
            }
            metadata = @{
                type = "object"
                description = "Metadata bổ sung"
                properties = @{
                    useCase = @{
                        type = "string"
                    }
                    priority = @{
                        type = "string"
                        enum = @("low", "medium", "high", "critical")
                    }
                }
            }
        }
    }
    outputSchema = @{
        type = "object"
        required = @("generatedSteps", "generationPlan", "generatedAt")
        properties = @{
            generatedSteps = @{
                type = "array"
                description = "Danh sách các steps đã được generate"
                items = @{
                    type = "object"
                    properties = @{
                        stepId = @{
                            type = "string"
                            description = "ID của step đã được tạo"
                        }
                        stepName = @{
                            type = "string"
                        }
                        stepType = @{
                            type = "string"
                            enum = @("GENERATE", "JUDGE", "STEP_GENERATION")
                        }
                        order = @{
                            type = "integer"
                            description = "Thứ tự trong workflow"
                        }
                        inputSchema = @{
                            type = "object"
                            description = "Input schema của step"
                        }
                        outputSchema = @{
                            type = "object"
                            description = "Output schema của step"
                        }
                        description = @{
                            type = "string"
                        }
                        dependencies = @{
                            type = "array"
                            description = "Các steps phụ thuộc"
                            items = @{
                                type = "string"
                            }
                        }
                    }
                }
            }
            generationPlan = @{
                type = "object"
                description = "Kế hoạch generation"
                properties = @{
                    totalSteps = @{
                        type = "integer"
                    }
                    estimatedTime = @{
                        type = "integer"
                        description = "Thời gian ước tính (seconds)"
                    }
                    workflowStructure = @{
                        type = "object"
                        description = "Cấu trúc workflow"
                        properties = @{
                            parallelSteps = @{
                                type = "array"
                                description = "Các steps có thể chạy song song"
                            }
                            sequentialSteps = @{
                                type = "array"
                                description = "Các steps phải chạy tuần tự"
                            }
                        }
                    }
                    reasoning = @{
                        type = "string"
                        description = "Lý do tại sao generate các steps này"
                    }
                }
            }
            generatedAt = @{
                type = "string"
                format = "date-time"
            }
            model = @{
                type = "string"
                description = "Model AI đã sử dụng"
            }
            tokens = @{
                type = "object"
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
    targetLevel = "L2"
    parentLevel = "L1"
    status = "active"
    metadata = @{
        category = "step-generation"
        version = "1.0.0"
        dynamic = $true
    }
}

$createStepGen = Invoke-ApiRequest -Method "POST" -Url "$baseUrl/api/v1/ai/steps/insert-one" -Body $stepGenerationData -ActiveRoleID $activeRoleID
if ($createStepGen.Success) {
    $stepGenId = $createStepGen.Data.data.id
    Write-Host "✓ Đã tạo STEP_GENERATION Step - ID: $stepGenId" -ForegroundColor Green
} else {
    Write-Host "✗ Lỗi: $($createStepGen.Error.message)" -ForegroundColor Red
    exit 1
}
Write-Host ""

# ========================================
# TỔNG KẾT
# ========================================
Write-Host "========================================" -ForegroundColor Cyan
Write-Host "TỔNG KẾT" -ForegroundColor Cyan
Write-Host "========================================" -ForegroundColor Cyan
Write-Host ""
Write-Host "✓ GENERATE Step:" -ForegroundColor Green
Write-Host "  ID: $generateStepId" -ForegroundColor Gray
Write-Host "  Input: pillarId, pillarName, targetAudience, context, numberOfCandidates" -ForegroundColor Gray
Write-Host "  Output: candidates[], generatedAt, model, tokens" -ForegroundColor Gray
Write-Host ""
Write-Host "✓ JUDGE Step:" -ForegroundColor Green
Write-Host "  ID: $judgeStepId" -ForegroundColor Gray
Write-Host "  Input: candidates[], criteria, context" -ForegroundColor Gray
Write-Host "  Output: scores[], rankings[], bestCandidate, judgedAt" -ForegroundColor Gray
Write-Host ""
Write-Host "✓ STEP_GENERATION Step:" -ForegroundColor Green
Write-Host "  ID: $stepGenId" -ForegroundColor Gray
Write-Host "  Input: parentContext, requirements, targetLevel, constraints" -ForegroundColor Gray
Write-Host "  Output: generatedSteps[], generationPlan, generatedAt" -ForegroundColor Gray
Write-Host ""
Write-Host "Tất cả 3 loại steps đã được tạo thành công!" -ForegroundColor Yellow
