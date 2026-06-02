package redis

import (
	"context"

	redis "github.com/redis/go-redis/v9"
)

func NewClient(ctx context.Context, redisURL string) (*redis.Client, error) {
	opts, err := redis.ParseURL(redisURL)
	if err != nil {
		return nil, err
	}

	client := redis.NewClient(opts)
	if _, err := client.Ping(ctx).Result(); err != nil {
		client.Close()
		return nil, err
	}

	return client, nil
}