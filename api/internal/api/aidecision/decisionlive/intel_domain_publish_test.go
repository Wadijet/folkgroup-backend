package decisionlive

import (
	"testing"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestPublishIntelDomainMilestone_boQuKhiThieuTrace(t *testing.T) {
	// Không panic / không lỗi khi thiếu trace — cùng hợp đồng Publish.
	PublishIntelDomainMilestone(primitive.NewObjectID(), "", "c", IntelDomainCIX, IntelMilestoneStart, "x", nil, nil)
}
