package utility

import (
	"fmt"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// FormatBytes chuyển đổi số bytes thành chuỗi dễ đọc (KB, MB, GB)
func FormatBytes(bytes uint64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}

	div, exp := uint64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// String2ObjectID chuyển đổi chuỗi thành ObjectID
// @params - chuỗi cần chuyển đổi
// @returns - ObjectID
func String2ObjectID(id string) primitive.ObjectID {
	objectId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return primitive.NilObjectID
	}
	return objectId
}

// ObjectID2String chuyển đổi ObjectID thành chuỗi
// @params - ObjectID cần chuyển đổi
// @returns - chuỗi ObjectID
func ObjectID2String(id primitive.ObjectID) string {
	stringObjectID := id.Hex()
	return stringObjectID
}

// StringArray2ObjectIDArray chuyển đổi mảng chuỗi thành mảng ObjectID
// @params - mảng chuỗi cần chuyển đổi
// @returns - mảng ObjectID
func StringArray2ObjectIDArray(ids []string) []primitive.ObjectID {
	var objectIDs []primitive.ObjectID
	for _, id := range ids {
		objectIDs = append(objectIDs, String2ObjectID(id))
	}
	return objectIDs
}
