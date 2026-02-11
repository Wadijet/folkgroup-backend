package utility

// Contains kiểm tra một phần tử có trong slice hay không
func Contains[T comparable](slice []T, item T) bool {
	for _, v := range slice {
		if v == item {
			return true
		}
	}
	return false
}
