// Package utility — UID theo chuẩn hệ thống (Unified Data Contract).
//
// Format: {prefix}_{unique_part}
// - prefix: 3–5 ký tự (cust_, ord_, sess_, evt_, trace_, dec_, act_, exe_, corr_)
// - unique_part: 12 ký tự alphanumeric
package utility

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// UIDPrefix các prefix chuẩn theo contract.
const (
	UIDPrefixCustomer   = "cust_"
	UIDPrefixOrder      = "ord_"
	UIDPrefixSession    = "sess_"
	UIDPrefixEvent      = "evt_"
	UIDPrefixTrace      = "trace_"
	UIDPrefixDecision   = "dec_"
	UIDPrefixAction     = "act_"
	UIDPrefixExecution   = "exe_"
	UIDPrefixCorrelation = "corr_"
	UIDPrefixPlan        = "plan_"
	UIDPrefixNote        = "note_"
	UIDPrefixActivity    = "act_"
	UIDPrefixConversation = "conv_"
	UIDPrefixDecisionCase = "dcs_" // decision case (runtime)

	// POS catalog (CIO sync — pc_pos_*)
	UIDPrefixPosProduct   = "pprd_"
	UIDPrefixPosCategory  = "pctg_"
	UIDPrefixPosVariation = "pvar_"
	UIDPrefixPosWarehouse = "pwhs_"
	UIDPrefixPosShop      = "pshp_"

	// Meta Marketing API (CIO sync — meta_*)
	UIDPrefixMetaAdAccount = "macc_"
	UIDPrefixMetaCampaign  = "mcmp_"
	UIDPrefixMetaAdSet     = "mset_"
	UIDPrefixMetaAd        = "mtad_"
	UIDPrefixMetaInsight   = "mins_"

	// Facebook messages (CIO interaction_message)
	UIDPrefixFbMessage     = "fmsg_" // metadata 1 doc / conversation — fb_messages
	UIDPrefixFbMessageItem = "fmit_" // từng tin — fb_message_items
)

// alphanumeric cho unique_part (base62-like, bỏ 0OIl để tránh nhầm).
const uidChars = "abcdefghijklmnopqrstuvwxyz0123456789"

// GenerateUID tạo UID mới với prefix. Dùng khi tạo entity mới.
// unique_part: 12 ký tự random — đủ unique cho hầu hết use case.
func GenerateUID(prefix string) string {
	prefix = normalizePrefix(prefix)
	b := make([]byte, 12)
	if _, err := rand.Read(b); err != nil {
		// Fallback: dùng uuid
		return prefix + strings.ReplaceAll(uuid.New().String(), "-", "")[:12]
	}
	for i := range b {
		b[i] = uidChars[int(b[i])%len(uidChars)]
	}
	return prefix + string(b)
}

// UIDFromObjectID tạo uid = prefix + _id.Hex(). Dùng cho entity tạo mới trong hệ.
// Một nguồn uniqueness (ObjectID), đơn giản, không collision.
func UIDFromObjectID(prefix string, id primitive.ObjectID) string {
	return normalizePrefix(prefix) + id.Hex()
}

// UIDFromSource tạo UID deterministic từ giá trị nguồn. Cùng input → cùng output.
// Dùng khi transform ID nguồn (pos uuid, fb id) sang format contract — idempotent upsert.
func UIDFromSource(prefix string, sourceValue interface{}) string {
	prefix = normalizePrefix(prefix)
	var input string
	switch v := sourceValue.(type) {
	case string:
		input = v
	case int64:
		input = fmt.Sprintf("%d", v)
	case int:
		input = fmt.Sprintf("%d", v)
	default:
		input = fmt.Sprintf("%v", sourceValue)
	}
	if input == "" {
		return ""
	}
	hash := sha256.Sum256([]byte(input))
	hexStr := hex.EncodeToString(hash[:])
	return prefix + hexStr[:12]
}

// normalizePrefix đảm bảo prefix kết thúc bằng _.
func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return "uid_"
	}
	if !strings.HasSuffix(prefix, "_") {
		return prefix + "_"
	}
	return prefix
}

// IsUID kiểm tra string có đúng format UID (prefix_uniquepart).
func IsUID(s string) bool {
	if len(s) < 5 {
		return false
	}
	idx := strings.Index(s, "_")
	if idx <= 0 || idx >= len(s)-1 {
		return false
	}
	_ = s[:idx+1] // prefix
	part := s[idx+1:]
	return len(part) >= 8 && len(part) <= 24 && isAlphanumeric(part)
}

func isAlphanumeric(s string) bool {
	for _, c := range s {
		if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			continue
		}
		return false
	}
	return true
}
