package options

import (
	"admitee/pkg/model"
	"context"
	"github.com/go-redis/redis/v9"
	"strconv"
)

func (opt *Options) NewClientRedis() (*model.AdmiteeRedisClient, error) {
	redis_opt := redis.Options{
		Addr:     opt.RedisAddress + ":" + strconv.Itoa(opt.RedisPort),
		Password: opt.RedisPassword,
		DB:       opt.RedisDB,
	}
	rdb := redis.NewClient(&redis_opt)

	var arc = &model.AdmiteeRedisClient{
		Client: rdb,
		Ctx:    context.Background(),
	}
	// 创建连接池

	// 判断是否能够链接到数据库
	_, err := rdb.Ping(arc.Ctx).Result()
	if err != nil {
		return arc, err
	}
	return arc, err
}
