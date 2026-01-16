#!/bin/bash

# Script kiểm tra race conditions trong codebase
# Sử dụng Go race detector để phát hiện các vấn đề về concurrent access

echo "🔍 Kiểm tra Race Conditions trong Codebase..."
echo "=============================================="
echo ""

# Màu sắc cho output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Kiểm tra xem có Go không
if ! command -v go &> /dev/null; then
    echo -e "${RED}❌ Go chưa được cài đặt hoặc không có trong PATH${NC}"
    exit 1
fi

echo -e "${GREEN}✅ Go version:$(go version)${NC}"
echo ""

# Tìm tất cả các file Go
echo "📁 Đang tìm các file Go..."
GO_FILES=$(find . -name "*.go" -not -path "./vendor/*" -not -path "./.git/*" | wc -l)
echo -e "${GREEN}✅ Tìm thấy $GO_FILES file Go${NC}"
echo ""

# Kiểm tra bytes.Buffer usage
echo "🔍 Kiểm tra sử dụng bytes.Buffer..."
BUFFER_USAGE=$(grep -r "bytes.Buffer" . --include="*.go" 2>/dev/null | grep -v "vendor" | grep -v ".git" | wc -l)
if [ "$BUFFER_USAGE" -gt 0 ]; then
    echo -e "${YELLOW}⚠️  Tìm thấy $BUFFER_USAGE nơi sử dụng bytes.Buffer${NC}"
    echo "   Các vị trí:"
    grep -r "bytes.Buffer" . --include="*.go" 2>/dev/null | grep -v "vendor" | grep -v ".git" | sed 's/^/   - /'
else
    echo -e "${GREEN}✅ Không tìm thấy sử dụng bytes.Buffer${NC}"
fi
echo ""

# Kiểm tra goroutines
echo "🔍 Kiểm tra goroutines..."
GOROUTINE_COUNT=$(grep -r "go func" . --include="*.go" 2>/dev/null | grep -v "vendor" | grep -v ".git" | wc -l)
echo -e "${GREEN}✅ Tìm thấy $GOROUTINE_COUNT nơi sử dụng goroutines${NC}"
echo ""

# Chạy race detector trên tests
if [ -d "./api" ]; then
    echo "🧪 Chạy race detector trên tests..."
    echo "   (Có thể mất vài phút...)"
    echo ""
    
    cd api || exit 1
    
    # Chạy race detector
    if go test -race ./... 2>&1 | tee /tmp/race-check.log; then
        echo ""
        echo -e "${GREEN}✅ Không phát hiện race condition trong tests${NC}"
    else
        echo ""
        echo -e "${RED}❌ Phát hiện race condition!${NC}"
        echo "   Xem chi tiết trong /tmp/race-check.log"
        echo ""
        echo "   Các vấn đề phổ biến:"
        echo "   1. bytes.Buffer được truy cập từ nhiều goroutine"
        echo "   2. Map được modify từ nhiều goroutine"
        echo "   3. Shared variable không có mutex protection"
        exit 1
    fi
    
    cd ..
else
    echo -e "${YELLOW}⚠️  Không tìm thấy thư mục ./api để chạy tests${NC}"
fi

echo ""
echo "=============================================="
echo -e "${GREEN}✅ Hoàn thành kiểm tra!${NC}"
echo ""
echo "💡 Lưu ý:"
echo "   - Race detector chỉ phát hiện race conditions khi code được chạy"
echo "   - Nên chạy race detector thường xuyên trong development"
echo "   - Xem thêm: docs/analysis/buffer-writebyte-crash-analysis.md"
