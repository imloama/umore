package cache

import (
	"context"
	"log"
	"time"
)

// Options 选缓存实现所需参数
type Options struct {
	Addr        string
	Password    string
	DB          int
	Prefix      string
	DialTimeout time.Duration
}

// Open 配了 Redis 地址就用 Redis，没配退化成进程内缓存。
// 配置了却连不上直接报错：正式环境不该悄悄降级成单机内存缓存，
// 否则多实例部署时验证码和登录态会各存各的。
func Open(ctx context.Context, opt Options) (Store, error) {
	if opt.Addr == "" {
		log.Printf("WARN: 未配置 redis，验证码与登录 token 存进程内存（仅适合本地开发）")
		return NewMemory(), nil
	}
	store, err := NewRedis(ctx, RedisOptions(opt))
	if err != nil {
		return nil, err
	}
	log.Printf("Redis 已连接: %s (prefix=%s)", opt.Addr, opt.Prefix)
	return store, nil
}
