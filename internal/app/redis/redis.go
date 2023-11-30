package redis

import (
	"L1/internal/app/config"
	"context"
	"fmt"
	"log"
	"strconv"

	"github.com/go-redis/redis/v8"
)

const servicePrefix = "orbits-api."

type Client struct {
	cfg    config.RedisConfig
	client *redis.Client
}

func New(ctx context.Context, cfg config.RedisConfig) (*Client, error) {
	client := &Client{}

	client.cfg = cfg

	redisClient := redis.NewClient(&redis.Options{
		Password:    cfg.Password,
		Username:    cfg.User,
		Addr:        cfg.Host + ":" + strconv.Itoa(cfg.Port),
		DB:          0,
		DialTimeout: cfg.DialTimeout,
		ReadTimeout: cfg.ReadTimeout,
	})

	client.client = redisClient

	if _, err := redisClient.Ping(ctx).Result(); err != nil {
		log.Println("!!! NO REDIS !!!")
		return nil, fmt.Errorf("can't ping redis: %w", err)
	}
	log.Println("REDIS CLIENT: ", client.cfg)

	return client, nil
}

func (c *Client) Close() error {
	return c.client.Close()
}
