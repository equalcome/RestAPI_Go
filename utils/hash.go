package utils

import "golang.org/x/crypto/bcrypt"

func HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 14)
	return string(bytes), err
}

func CheckPasswordHash(password, hashedPassword string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	//hashedPassword（資料庫的雜湊值）去比對 password（使用者輸入的明碼）
	//從 hashedPassword 讀 鹽值。
	//每個密碼都有自己的顏值
	return err == nil
}