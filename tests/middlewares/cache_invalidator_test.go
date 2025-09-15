// 測試目的：CacheInvalidator（清除 Redis 快取）
// 1) 先塞入 list/item key
// 2) 呼叫 PurgeEventsList / PurgeEventItem 後，keys 應被刪除
package tests

import (
	"context"
	"testing"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"

	"restapi/utils"
)

//先植入 cache:events:list:* 與 cache:events:item:* key，
//呼叫 PurgeEventsList / PurgeEventItem 後應全數清除（驗證快取失效器）。
func TestCacheInvalidator_Purge(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	inv := utils.NewCacheInvalidator(rdb)

	ctx := context.Background()
	_ = rdb.Set(ctx, "cache:events:list:page=1", "x", 0).Err()
	_ = rdb.Set(ctx, "cache:events:item:abc", "x", 0).Err()

	inv.PurgeEventsList(ctx)
	inv.PurgeEventItem(ctx, "abc")

	// 所有 key 都應該被清空
	if len(mr.Keys()) != 0 {
		t.Fatalf("keys not purged: %v", mr.Keys())
	}
}
