package cache

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// redisStore 正式环境的实现。验证码、登录 token 都在这里，不落 Postgres。
type redisStore struct {
	client *redis.Client
	prefix string
}

// RedisOptions 连 Redis 需要的东西
type RedisOptions struct {
	Addr        string
	Password    string
	DB          int
	Prefix      string
	DialTimeout time.Duration
}

// NewRedis 连不上就报错，让启动阶段直接失败，而不是等到用户注册时才发现缓存没配好
func NewRedis(ctx context.Context, opt RedisOptions) (Store, error) {
	client := redis.NewClient(&redis.Options{
		Addr:        opt.Addr,
		Password:    opt.Password,
		DB:          opt.DB,
		DialTimeout: opt.DialTimeout,
	})
	pingCtx, cancel := context.WithTimeout(ctx, opt.DialTimeout)
	defer cancel()
	if err := client.Ping(pingCtx).Err(); err != nil {
		client.Close()
		return nil, fmt.Errorf("connect redis %s: %w", opt.Addr, err)
	}
	prefix := opt.Prefix
	if prefix == "" {
		prefix = "umore"
	}
	return &redisStore{client: client, prefix: prefix}, nil
}

func (s *redisStore) Mode() string { return "redis" }

func (s *redisStore) key(key string) string { return s.prefix + ":" + key }

func (s *redisStore) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	return s.client.Set(ctx, s.key(key), value, ttl).Err()
}

func (s *redisStore) Get(ctx context.Context, key string) ([]byte, bool, error) {
	val, err := s.client.Get(ctx, s.key(key)).Bytes()
	if errors.Is(err, redis.Nil) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return val, true, nil
}

func (s *redisStore) Del(ctx context.Context, key string) error {
	return s.client.Del(ctx, s.key(key)).Err()
}

// Incr 用 pipeline 保证「自增」与「首次设过期」一起发出，
// 早期版本的 Redis 没有 INCR ... EX 的原子写法，这样写兼容性最好。
func (s *redisStore) Incr(ctx context.Context, key string, ttl time.Duration) (int64, error) {
	full := s.key(key)
	pipe := s.client.TxPipeline()
	incr := pipe.Incr(ctx, full)
	if ttl > 0 {
		// NX：只在这个键还没有过期时间时设置，别把已有倒计时续上
		pipe.ExpireNX(ctx, full, ttl)
	}
	if _, err := pipe.Exec(ctx); err != nil {
		return 0, err
	}
	return incr.Val(), nil
}

func (s *redisStore) Close() error { return s.client.Close() }
