# Seed Rule Intelligence: RULE_DECISION_CONSUMER_DISPATCH (routing noop|dispatch cho consumer AI Decision).
# Cần .env với MONGODB_* giống khi chạy server.
#
# Chạy từ thư mục repo:
#   .\scripts\seed_aidecision_dispatch.ps1
#
$ErrorActionPreference = "Stop"
$root = Split-Path -Parent $PSScriptRoot
Set-Location (Join-Path $root "api")
go run ./cmd/server --seed-aidecision-dispatch
exit $LASTEXITCODE
