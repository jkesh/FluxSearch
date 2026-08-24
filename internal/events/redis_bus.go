package events

import (
	"context"
	"fmt"
	"log"

	"github.com/redis/go-redis/v9"
)

type RedisBus struct {
	rdb *redis.Client
}

func NewRedisBus(rdb *redis.Client) *RedisBus {
	return &RedisBus{rdb: rdb}
}

func (b *RedisBus) Publish(ctx context.Context, ev Event) error {
	if b == nil || b.rdb == nil {
		return nil
	}
	raw, err := ev.Marshal()
	if err != nil {
		return err
	}
	return b.rdb.Publish(ctx, RedisChannel, raw).Err()
}

func (b *RedisBus) Subscribe(ctx context.Context, handler func(Event)) error {
	if b == nil || b.rdb == nil {
		return fmt.Errorf("redis unavailable")
	}
	sub := b.rdb.Subscribe(ctx, RedisChannel)
	defer sub.Close()

	ch := sub.Channel()
	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case msg, ok := <-ch:
			if !ok {
				return nil
			}
			ev, err := Parse([]byte(msg.Payload))
			if err != nil {
				log.Printf("events parse: %v", err)
				continue
			}
			handler(ev)
		}
	}
}
