package middlewares

import (
	"context"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type QuotaRule struct {
	Limit  int // 配額上限（某段時間允許多少次請求）
	Window time.Duration // 視窗大小，例如 1 小時
	KeyFn  func(*gin.Context) string // 決定用什麼 key 來區分配額 用 userId 作 key
}

func Quota(rdb *redis.Client, rule QuotaRule) gin.HandlerFunc {
	return func(c *gin.Context) {
		key := rule.KeyFn(c)
		if key == "" {
			c.Next()
			return
		}
		ctx := context.Background()

		//INCR 是計數器，每次請求讓計數+1。 n：表示加完後的數字（int64）。
		//如果 key 不存在 → Redis 會自動創一個 key，初始值是 0
		n, err := rdb.Incr(ctx, key).Result()  
		if err != nil {
			// Redis 掛了→降級放行 讓你過拔QQ
			c.Next()
			return
		}
		//第一次建立Key Window 給配額
		if n == 1 {
			_ = rdb.Expire(ctx, key, rule.Window).Err()  //告訴 Redis「這個 key 再過 duration 時間就自動刪掉
		}
		if int(n) > rule.Limit {
			c.AbortWithStatusJSON(429, gin.H{
				"message": "Usage quota exceeded. Please try again later.",
			})
			return
		}
		c.Header("X-Quota-Used", fmt.Sprintf("%d/%d", n, rule.Limit))  //X-Quota-Used: 5/100
		c.Next()
	}
}
