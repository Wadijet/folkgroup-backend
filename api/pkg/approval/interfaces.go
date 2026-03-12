package approval

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FindFilter filter cho Find (list với filter — phục vụ frontend xem).
type FindFilter struct {
	Domain         string // Rỗng = tất cả domain
	Status         string // Rỗng = tất cả status
	Limit          int    // Mặc định 50
	SortField      string // Mặc định proposedAt
	SortOrder      int    // 1 = asc, -1 = desc (mặc định -1)
	FromProposedAt int64  // Lọc proposedAt >= (Unix ms), 0 = không lọc
	ToProposedAt   int64  // Lọc proposedAt <= (Unix ms), 0 = không lọc
}

// FindWithPaginationFilter filter cho FindWithPagination.
type FindWithPaginationFilter struct {
	FindFilter
	Page int64 // Trang (mặc định 1)
}

// Storage lưu trữ ActionPending. App cung cấp implementation (MongoDB).
type Storage interface {
	Insert(ctx context.Context, doc *ActionPending) error
	Update(ctx context.Context, doc *ActionPending) error
	FindById(ctx context.Context, id primitive.ObjectID, ownerOrgID primitive.ObjectID) (*ActionPending, error)
	FindPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]ActionPending, error)
	// FindQueued danh sách item status=queued để worker xử lý (filter nextRetryAt null hoặc <= now).
	FindQueued(ctx context.Context, domain string, limit int) ([]ActionPending, error)
	// Find danh sách với filter (domain, status, limit, sort) — phục vụ frontend.
	Find(ctx context.Context, ownerOrgID primitive.ObjectID, filter FindFilter) ([]ActionPending, error)
	// FindWithPagination danh sách có phân trang — trả items, total.
	FindWithPagination(ctx context.Context, ownerOrgID primitive.ObjectID, filter FindWithPaginationFilter) ([]ActionPending, int64, error)
	// Count đếm theo filter — phục vụ dashboard badges.
	Count(ctx context.Context, ownerOrgID primitive.ObjectID, domain, status string, fromProposedAt, toProposedAt int64) (int64, error)
}

// Notifier gửi thông báo. App cung cấp implementation (notifytrigger).
type Notifier interface {
	Notify(ctx context.Context, eventType string, payload map[string]interface{}, ownerOrgID primitive.ObjectID, baseURL string) (int, error)
}

// Executor thực thi khi approve. Mỗi domain đăng ký.
type Executor interface {
	Execute(ctx context.Context, doc *ActionPending) (response map[string]interface{}, err error)
}

// ExecutorFunc adapter cho function.
type ExecutorFunc func(ctx context.Context, doc *ActionPending) (map[string]interface{}, error)

func (f ExecutorFunc) Execute(ctx context.Context, doc *ActionPending) (map[string]interface{}, error) {
	return f(ctx, doc)
}
