package stream

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
)

// ProgressEvent 是 SSE 推送给前端的一个事件单元。
// 跨语言对齐：Python worker 写入的 JSON 字段名必须与此一致（spec §5.4）。
type ProgressEvent struct {
	Type      string         `json:"type"`
	TaskID    string         `json:"task_id"`
	Agent     string         `json:"agent,omitempty"`
	Pct       *int           `json:"pct,omitempty"`
	Message   string         `json:"message,omitempty"`
	Extra     map[string]any `json:"extra,omitempty"`
	Timestamp int64          `json:"ts"`
}

// ProgressSubscriber 订阅某个 task_id 的进度事件流，事件通过 channel 推送。
// 调用方 Cancel ctx 即解除订阅；实现应保证 channel 被关闭。
type ProgressSubscriber interface {
	Subscribe(ctx context.Context, taskID string) (<-chan ProgressEvent, error)
}

// RedisSubscriber 基于 XREAD 阻塞读 progress stream。
// 每个 SSE 连接一个独立 goroutine + 一个 channel，互不影响。
type RedisSubscriber struct {
	rdb *redis.Client
}

func NewRedisSubscriber(rdb *redis.Client) *RedisSubscriber {
	return &RedisSubscriber{rdb: rdb}
}

func progressStream(taskID string) string {
	return fmt.Sprintf("progress:%s", taskID)
}

func (s *RedisSubscriber) Subscribe(ctx context.Context, taskID string) (<-chan ProgressEvent, error) {
	out := make(chan ProgressEvent, 16)
	go func() {
		defer close(out)
		lastID := "0" // 从头开始读，允许补发已落库的事件
		key := progressStream(taskID)
		for {
			select {
			case <-ctx.Done():
				return
			default:
			}

			res, err := s.rdb.XRead(ctx, &redis.XReadArgs{
				Streams: []string{key, lastID},
				Block:   2 * time.Second,
				Count:   16,
			}).Result()
			if err == redis.Nil {
				continue
			}
			if err != nil {
				// ctx 取消时也会走这里；安全退出
				return
			}
			for _, stream := range res {
				for _, msg := range stream.Messages {
					lastID = msg.ID
					raw, ok := msg.Values["data"].(string)
					if !ok {
						continue
					}
					var ev ProgressEvent
					if err := json.Unmarshal([]byte(raw), &ev); err != nil {
						continue
					}
					select {
					case <-ctx.Done():
						return
					case out <- ev:
					}
					// 收到 task.done 后主动结束
					if ev.Type == "task.done" {
						return
					}
				}
			}
		}
	}()
	return out, nil
}