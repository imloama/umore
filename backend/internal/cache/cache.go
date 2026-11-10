// Package cache 是一个很小的键值缓存抽象。
// 存在的理由很具体：验证码、登录 token 这类东西不该落库，
// 正式环境放 Redis，本地没起 Redis 就退化成进程内 map，代码只有一份。
package cache

import (
	"context"
	"sync"
	"time"
)

// Store 缓存需要的最小能力：带 TTL 的读写、原子自增。
// 只留这几个方法，是为了让 Redis 实现和内存实现都能一眼看完。
type Store interface {
	Set(ctx context.Context, key string, value []byte, ttl time.Duration) error
	Get(ctx context.Context, key string) (value []byte, ok bool, err error)
	Del(ctx context.Context, key string) error
	// Incr 原子自增，键不存在时从 1 开始，返回自增后的值；TTL 只在键刚创建时生效
	Incr(ctx context.Context, key string, ttl time.Duration) (int64, error)
	Close() error
	// Mode 给启动日志用：redis / memory
	Mode() string
}

// memoryStore 进程内实现，单机开发用。重启即清空，正好符合「验证码、token 丢了就重登」。
type memoryStore struct {
	mu    sync.Mutex
	items map[string]memoryItem
}

type memoryItem struct {
	value     []byte
	expiresAt time.Time // 零值表示不过期
}

func NewMemory() Store {
	s := &memoryStore{items: map[string]memoryItem{}}
	go s.sweep()
	return s
}

func (s *memoryStore) Mode() string { return "memory" }

func (s *memoryStore) Set(_ context.Context, key string, value []byte, ttl time.Duration) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	item := memoryItem{value: append([]byte(nil), value...)}
	if ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = item
	return nil
}

func (s *memoryStore) Get(_ context.Context, key string) ([]byte, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	item, ok := s.items[key]
	if !ok {
		return nil, false, nil
	}
	if !item.expiresAt.IsZero() && time.Now().After(item.expiresAt) {
		delete(s.items, key)
		return nil, false, nil
	}
	return append([]byte(nil), item.value...), true, nil
}

func (s *memoryStore) Del(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.items, key)
	return nil
}

func (s *memoryStore) Incr(_ context.Context, key string, ttl time.Duration) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var current int64
	if item, ok := s.items[key]; ok {
		if item.expiresAt.IsZero() || time.Now().Before(item.expiresAt) {
			current = parseInt(item.value)
		}
	}
	current++
	item := memoryItem{value: []byte(itoa(current))}
	// TTL 只在键刚建好时设置，自增不刷新过期时间，跟 Redis 的 INCR 语义保持一致
	if current == 1 && ttl > 0 {
		item.expiresAt = time.Now().Add(ttl)
	}
	s.items[key] = item
	return current, nil
}

func (s *memoryStore) Close() error { return nil }

// sweep 定期清理过期键，免得开发时内存只进不出
func (s *memoryStore) sweep() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()
	for range ticker.C {
		now := time.Now()
		s.mu.Lock()
		for key, item := range s.items {
			if !item.expiresAt.IsZero() && now.After(item.expiresAt) {
				delete(s.items, key)
			}
		}
		s.mu.Unlock()
	}
}
