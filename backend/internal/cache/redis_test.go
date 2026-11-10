package cache

import (
	"context"
	"os"
	"testing"
	"time"
)

// Redis 集成测试：设了 UMORE_TEST_REDIS_ADDR 才跑。
//
//	UMORE_TEST_REDIS_ADDR=127.0.0.1:6379 go test ./internal/cache/
func newRedisStore(t *testing.T) Store {
	t.Helper()
	addr := os.Getenv("UMORE_TEST_REDIS_ADDR")
	if addr == "" {
		t.Skip("UMORE_TEST_REDIS_ADDR 未设置，跳过 Redis 集成测试")
	}
	store, err := NewRedis(context.Background(), RedisOptions{
		Addr:        addr,
		Prefix:      "umore_test",
		DialTimeout: 2 * time.Second,
	})
	if err != nil {
		t.Fatalf("connect redis: %v", err)
	}
	t.Cleanup(func() { store.Close() })
	return store
}

func TestRedisSetGetDel(t *testing.T) {
	store := newRedisStore(t)
	ctx := context.Background()
	key := "ut:" + time.Now().Format("150405.000000")

	if err := store.Set(ctx, key, []byte("v"), time.Minute); err != nil {
		t.Fatalf("set: %v", err)
	}
	val, ok, err := store.Get(ctx, key)
	if err != nil || !ok || string(val) != "v" {
		t.Fatalf("get: %q ok=%v err=%v", val, ok, err)
	}
	if err := store.Del(ctx, key); err != nil {
		t.Fatalf("del: %v", err)
	}
	if _, ok, _ := store.Get(ctx, key); ok {
		t.Fatal("删掉之后不该还能读到")
	}
}

// TTL 生效：设 1 秒，等过期后读不到
func TestRedisTTL(t *testing.T) {
	store := newRedisStore(t)
	ctx := context.Background()
	key := "ut:ttl:" + time.Now().Format("150405.000000")

	if err := store.Set(ctx, key, []byte("v"), time.Second); err != nil {
		t.Fatalf("set: %v", err)
	}
	time.Sleep(1200 * time.Millisecond)
	if _, ok, _ := store.Get(ctx, key); ok {
		t.Fatal("过期的键应该读不到")
	}
}

func TestRedisIncr(t *testing.T) {
	store := newRedisStore(t)
	ctx := context.Background()
	key := "ut:incr:" + time.Now().Format("150405.000000")

	for i := int64(1); i <= 3; i++ {
		n, err := store.Incr(ctx, key, time.Minute)
		if err != nil {
			t.Fatalf("incr: %v", err)
		}
		if n != i {
			t.Fatalf("want %d, got %d", i, n)
		}
	}
	store.Del(ctx, key)
}

// 前缀确实加上了，别把键写进别人的命名空间
func TestRedisPrefixApplied(t *testing.T) {
	store := newRedisStore(t)
	rs, ok := store.(*redisStore)
	if !ok {
		t.Fatal("应是 redis 实现")
	}
	if rs.key("x") != "umore_test:x" {
		t.Fatalf("前缀不对: %q", rs.key("x"))
	}
}
