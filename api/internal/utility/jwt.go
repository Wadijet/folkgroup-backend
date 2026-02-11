package utility

import (
	authmodels "meta_commerce/internal/api/auth/models"

	"github.com/dgrijalva/jwt-go"
)

// CreateToken : hàm này nhận vào userId làm tham số,
// tạo ra JWT token và
// trả về chuỗi JWT token
// CreateToken tạo ra một JWT token sử dụng secret key, id, thời gian, và số ngẫu nhiên được cung cấp.
//
// Tham số:
// - secretKey: Chuỗi bí mật dùng để ký token.
// - id: ID của người dùng hoặc đối tượng cần xác thực.
// - time: Thời gian tạo token hoặc thời gian hết hạn.
// - randomNum: Số ngẫu nhiên để tăng tính bảo mật cho token.
//
// Trả về:
// - map[string]string: Bản đồ chứa token đã được ký.
// - error: Lỗi nếu có trong quá trình tạo token.
//
// Ví dụ:
//
//	tokenData, err := CreateToken("mySecretKey", "userID123", "2023-10-01T12:00:00Z", "random123")
//	if err != nil {
//	    log.Fatal(err)
//	}
//	fmt.Println(tokenData["token"])
func CreateToken(secretKey string, id string, time string, randomNum string) (map[string]string, error) {

	// Tạo đối tượng JwtToken với các thông tin cần thiết
	_token := authmodels.JwtToken{
		UserID:       id,
		Time:         time,
		RandomNumber: randomNum,
	}

	// Tạo token mới với phương thức ký HS256 và thông tin từ _token
	token := jwt.NewWithClaims(jwt.GetSigningMethod("HS256"), _token)

	// Ký token bằng secretKey
	tokenString, err := token.SignedString([]byte(secretKey))
	if err != nil {
		return nil, err
	}

	// Tạo bản đồ để chứa token đã ký
	m := make(map[string]string)
	m["token"] = tokenString // thiết lập dữ liệu phản hồi
	return m, nil
}
