package approval

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Storage lưu trữ ActionPending. App cung cấp implementation (MongoDB).
type Storage interface {
	Insert(ctx context.Context, doc *ActionPending) error
	Update(ctx context.Context, doc *ActionPending) error
	FindById(ctx context.Context, id primitive.ObjectID, ownerOrgID primitive.ObjectID) (*ActionPending, error)
	FindPending(ctx context.Context, ownerOrgID primitive.ObjectID, domain string, limit int) ([]ActionPending, error)
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
