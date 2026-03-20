package identity

import "meta_commerce/internal/utility"

// BuildLinkResolved tạo LinkItem đã resolve — có uid.
func BuildLinkResolved(uid string, externalRefs []ExternalRef) LinkItem {
	if len(externalRefs) == 0 && uid != "" {
		externalRefs = nil
	}
	return LinkItem{
		Uid:          uid,
		ExternalRefs: externalRefs,
		Status:       LinkStatusResolved,
	}
}

// BuildLinkPending tạo LinkItem chưa resolve — uid null, có externalRefs.
func BuildLinkPending(source, id string) LinkItem {
	return LinkItem{
		Uid:          "",
		ExternalRefs: []ExternalRef{{Source: source, ID: id}},
		Status:       LinkStatusPendingResolution,
	}
}

// IsUIDFormat kiểm tra string có đúng format UID (prefix_uniquepart).
func IsUIDFormat(s string) bool {
	return utility.IsUID(s)
}
