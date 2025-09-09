package utils

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const secretKey = "supersecret"

func GenerateToken(email string, userId int64) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.MapClaims{
		"email": email,
		"userId": userId,
		"exp": time.Now().Add(time.Hour * 2).Unix(), 
	})

	return token.SignedString([]byte(secretKey))
}

//驗證token + 回傳id
func VerifyToken(token string) (int64, error) {
	
	//檢驗token
	parsedToken, err := jwt.Parse(token, func(token *jwt.Token) (any, error) {

		//檢查algo
		_, ok := token.Method.(*jwt.SigningMethodHMAC)
		if !ok {
			return nil, errors.New("Unexpected signing method")
		}

		return []byte(secretKey), nil
	}) 
	if err != nil {
		return 0, errors.New("Could not parse token.")
	}

	//就算簽章正確，Token 也不一定「有效」 (可能過期)
	tokenIsValid := parsedToken.Valid
	if !tokenIsValid {
		return 0, errors.New("Invalid token!")
	}

	
	
	//parsedToken.Claims 存的是 Payload
	//轉型成 jwt.MapClaims（map 格式）好存取 //map[string]interface{}
	Claims, ok := parsedToken.Claims.(jwt.MapClaims)
	if !ok {
		return 0, errors.New("Invalid token claims")
	}
	// email := Claims["email"].(string)
	userId := int64(Claims["userId"].(float64))

	

	return userId, nil
}


// <base64(header)>.<base64(payload)> 用 secretKey 和演算法產生出 <base64(signature)>
// 最後token就變成<base64(header)>.<base64(payload)>.<base64(signature)>

