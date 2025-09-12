package utils

import (
	"context"
	"strings"

	"github.com/redis/go-redis/v9"
)

type CacheInvalidator struct{ rdb *redis.Client }
func NewCacheInvalidator(rdb *redis.Client) *CacheInvalidator { return &CacheInvalidator{rdb} }

func (ci *CacheInvalidator) PurgeEventsList(ctx context.Context) {
	// 刪除所有 events 列表 key
	iter := ci.rdb.Scan(ctx, 0, "cache:events:list:*", 0).Iterator()
	for iter.Next(ctx) {
		_ = ci.rdb.Del(ctx, iter.Val()).Err()
	}
}

func (ci *CacheInvalidator) PurgeEventItem(ctx context.Context, id string) {
	// id 被組成 sha1 後存在 key 裡，要跟產生規則一致
	// 這裡直接按前綴掃描（若你保留原始 id 在 key 裡，可更精準）
	iter := ci.rdb.Scan(ctx, 0, "cache:events:item:*", 0).Iterator()
	for iter.Next(ctx) {
		k := iter.Val()
		// 簡化：如果 key 命名有包含 id 原文可用 strings.Contains(k, id)
		// 我們用 sha1 了→此處保守刪整個 item:*（或在 CacheKeyFrom 改成含原始 id）
		// 還是全刪了 (因為id用 sha1)
		if strings.HasPrefix(k, "cache:events:item:") {
			_ = ci.rdb.Del(ctx, k).Err()
		}
	}
}
