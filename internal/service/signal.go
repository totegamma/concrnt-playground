package service

import (
	"context"
	"encoding/json"

	"github.com/redis/go-redis/v9"

	"github.com/totegamma/concrnt-playground"
)

type SignalService struct {
	rdb *redis.Client
}

func NewSignalService(redisClient *redis.Client) *SignalService {
	return &SignalService{
		rdb: redisClient,
	}
}

func (s *SignalService) Publish(ctx context.Context, channel string, event concrnt.Event) error {

	jsonstr, err := json.Marshal(event)
	if err != nil {
		return err
	}

	err = s.rdb.Publish(ctx, channel, jsonstr).Err()
	if err != nil {
		return err

	}

	return nil
}
