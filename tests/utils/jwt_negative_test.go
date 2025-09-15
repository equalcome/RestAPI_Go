// 測試目的：VerifyToken 異常路徑（token 被竄改 → 驗證失敗）【】
package tests

import (
	"restapi/utils"
	"testing"
)

//把 token 竄改後必須驗證失敗（涵蓋 JWT 驗證的錯誤支線）
func TestVerifyToken_Tampered_Fails(t *testing.T) {
	tok, err := utils.GenerateToken("x@x.com", 99)
	if err != nil { t.Fatalf("gen: %v", err) }

	// 竄改 payload（簡單替換字元破壞簽章）
	tampered := tok + "x"
	if _, err := utils.VerifyToken(tampered); err == nil {
		t.Fatalf("expect verify to fail on tampered token")
	}
}
