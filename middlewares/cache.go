package middlewares

import (
	"bytes"
	"context"
	"crypto/sha1"
	"encoding/gob"
	"encoding/hex"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redis/go-redis/v9"
)

type cachedBody struct {
	Status int
	Header map[string][]string
	Body   []byte
}

//把 路徑+參數 轉成 SHA1 雜湊字串，避免 Redis key 太長
func sha1Hex(s string) string {
	sum := sha1.Sum([]byte(s))
	return hex.EncodeToString(sum[:])
}

// 依路徑命名空間，方便失效：list 與 item
// 第一個字串：完整 Redis Key 第二個字串：類別標籤
func CacheKeyFrom(c *gin.Context) (string, string) {   
	method := c.Request.Method // HTTP 方法，例如 GET
	path := c.FullPath() // 路由模板，例如 /events/:id，而不是實際的 /events/123
	rawq := c.Request.URL.RawQuery // Query String（網址後面的 ?page=2）

	//修改資料的請求(eg post)不會有塊取
	if method != "GET" || path == "" {
		return "", ""
	}

	switch {
	case strings.HasPrefix(path, "/events/:id"):
		id := c.Param("id")
		return "cache:events:item:" + sha1Hex("GET|/events/"+id), "item"  // cache:events:item:abcd1234...
	case strings.HasPrefix(path, "/events"):
		return "cache:events:list:" + sha1Hex("GET|/events|"+rawq), "list"
	default:
		// 其他 GET 也想快取可以在這加
		return "cache:generic:" + sha1Hex(method+"|"+path+"|"+rawq), "generic"
	}
}

func ResponseCache(rdb *redis.Client, ttl time.Duration) gin.HandlerFunc {
	return func(c *gin.Context) {
		key, _ := CacheKeyFrom(c)
		if key == "" {
			c.Next()    //根本不是get 滾，跑下個headler
			return
		}

		// 請求進來，查 Redis 有沒有快取資料(有hit) //b 是 Redis 取到的資料（byte slice 格式） //查key看value有沒有hit
		if b, err := rdb.Get(context.Background(), key).Bytes(); err == nil && len(b) > 0 { //如果 Redis 有這個快取，就進入下一步。
			var hit cachedBody
			if err := gob.NewDecoder(bytes.NewReader(b)).Decode(&hit); err == nil { // 把 Redis 裡存的快取資料解碼回 hit 變數 //把「Redis 中的快取資料」轉回 Go 的物件
				for k, vals := range hit.Header {
					for _, v := range vals {
						c.Writer.Header().Add(k, v)   //還原 API 回應時的 Handler
					}
				}
				//下面把hit到的write回去
				c.Writer.Header().Set("X-Cache", "HIT")
				c.Status(hit.Status)  //還原 HTTP 狀態碼
				_, _ = c.Writer.Write(hit.Body)  //還原 Response Body   
				return  //有快取 不呼叫 c.Next() 
			}
		}

		// 攔截回應 (沒hit)
		buf := &bytes.Buffer{}  // 用來暫存 API 回傳的內容
		bw := &bufferedWriter{ResponseWriter: c.Writer, buf: buf}  //換成bufferedWriter為了偷偷存一份回應
		c.Writer = bw

		c.Next() //來去存回應

		// 只快取 2xx  //回應回來了 把 buf 存 Redis做後續收尾
		if bw.Status() >= 200 && bw.Status() < 300 {
			item := cachedBody{
				Status: bw.Status(),
				Header: c.Writer.Header(),
				Body:   buf.Bytes(),
			}

			//把編碼後的資料存進 Redis
			var o bytes.Buffer
			if err := gob.NewEncoder(&o).Encode(item); err == nil {
				_ = rdb.Set(context.Background(), key, o.Bytes(), ttl).Err()
			}
			c.Writer.Header().Set("X-Cache", "MISS")
		}
	}
}

type bufferedWriter struct{ gin.ResponseWriter; buf *bytes.Buffer }

func (w *bufferedWriter) Write(b []byte) (int, error) {
	w.buf.Write(b)  //存一份到 buf (內存)                                    // 先寫到記憶體 buffer
	return w.ResponseWriter.Write(b)  //呼叫原本 ResponseWriter 寫給客戶端   // 再寫到真正的網路回應
}  //


// 系統收到請求 → 先查 Redis

// 有資料：直接回應（快）

// 沒資料：查資料庫 → 寫進 Redis → 回應

// Client → Redis Cache → (Miss) → DB → Redis Cache → Client
