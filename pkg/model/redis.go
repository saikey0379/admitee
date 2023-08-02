package model

import (
	"context"
	"sync"
	"time"

	"github.com/go-redis/redis/v9"
	"github.com/golang/glog"
)

type AdmiteeRedisClient struct {
	Client *redis.Client
	Ctx    context.Context
	Health bool
	sync.Mutex
}

func (c *AdmiteeRedisClient) Lock(key string) bool {
	c.Mutex.Lock()
	defer c.Mutex.Unlock()
	bool, err := c.Client.SetNX(c.Ctx, key, 1, 10*time.Second).Result()
	if err != nil {
		glog.Errorf("FAILURE: Lock[%v]", err)
	}
	return bool
}

func (c *AdmiteeRedisClient) UnLock(key string) int64 {
	nums, err := c.Client.Del(c.Ctx, key).Result()
	if err != nil {
		glog.Errorf("FAILURE: UnLock[%v]", err)
	}
	return nums
}

func (c *AdmiteeRedisClient) HealthCheckRdb() {
	for {
		_, err := c.Client.Ping(c.Ctx).Result()
		if err != nil {
			c.Health = false
		} else {
			c.Health = true
		}
		time.Sleep(time.Duration(10) * time.Second)
	}
}
