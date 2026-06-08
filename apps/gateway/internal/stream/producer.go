// Package stream 把 Redis Streams 的细节封在 gateway 内部。
// 上层只看到 TaskProducer / ProgressSubscriber 两个接口。
package stream

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/redis/go-redis/v9"
)

// TaskEnvelope 是所有任务的通用包装；具体 payload 放 Args（JSON）。
// 任务种类用 Kind 区分：echo / wiki.collect / course.build / ...
type TaskEnvelope struct {
	TaskID string          `json:"task_id"`
	Kind   string          `json:"kind"`
	Args   json.RawMessage `json:"args,omitempty"`
}

// TaskProducer 把任务投递到对应 Stream。
type TaskProducer interface {
	Enqueue(ctx context.Context, e TaskEnvelope) error
}

// RedisProducer 把任务写入 Redis Streams。
// Stream 选择策略：按 Kind 路由。M1 只有 echo，对应 stream "wiki:collect:tasks"
// （M2 再加 course:build:tasks 等）。
type RedisProducer struct {
	rdb *redis.Client
}

func NewRedisProducer(rdb *redis.Client) *RedisProducer {
	return &RedisProducer{rdb: rdb}
}

// streamFor 按任务 kind 决定写到哪个 stream。
// 表驱动以便后续增加 kind 时只改这一处。
func streamFor(kind string) (string, error) {
	switch kind {
	case "echo", "wiki.collect":
		return "wiki:collect:tasks", nil
	case "course.build":
		return "course:build:tasks", nil
	default:
		return "", fmt.Errorf("unknown task kind %q", kind)
	}
}

func (p *RedisProducer) Enqueue(ctx context.Context, e TaskEnvelope) error {
	streamKey, err := streamFor(e.Kind)
	if err != nil {
		return err
	}
	payload, err := json.Marshal(e)
	if err != nil {
		return fmt.Errorf("marshal envelope: %w", err)
	}
	if _, err := p.rdb.XAdd(ctx, &redis.XAddArgs{
		Stream: streamKey,
		Values: map[string]any{"data": payload},
	}).Result(); err != nil {
		return fmt.Errorf("xadd: %w", err)
	}
	return nil
}