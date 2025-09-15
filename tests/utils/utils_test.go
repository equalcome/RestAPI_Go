package tests

import (
	"restapi/utils"
	"testing"
)

//bcrypt 正確密碼應通過；錯誤密碼應失敗。
func TestHashAndCheckPassword(t *testing.T) {
	hashed, err := utils.HashPassword("p@ss")
	if err != nil { t.Fatalf("hash err: %v", err) }
	if !utils.CheckPasswordHash("p@ss", hashed) {
		t.Fatalf("should match")
	}
	if utils.CheckPasswordHash("hahaha", hashed) {
		t.Fatalf("should not match")
	}
}

//能成功產生 JWT，VerifyToken 解析出正確 userId（200/成功路徑的核心）。
func TestJWTGenerateAndVerify(t *testing.T) {
	token, err := utils.GenerateToken("a@b.com", 87)
	if err != nil { t.Fatalf("gen token err: %v", err) }
	uid, err := utils.VerifyToken(token)
	if err != nil { t.Fatalf("verify err: %v", err) }
	if uid != 87 { t.Fatalf("want 87 got %d", uid) }
}
