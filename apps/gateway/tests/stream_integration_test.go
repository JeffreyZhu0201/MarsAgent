//go:build integration

package tests

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/marsagent/gateway/internal/stream"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
	tcredis "github.com/testcontainers/testcontainers-go/modules/redis"
)

func TestProducerSubscriberRoundTrip(t *testing.T) {
	ctx := context.Background()

	container, err := tcredis.Run(ctx, "redis:7-alpine")
	require.NoError(t, err)
	defer container.Terminate(ctx)

	endpoint, err := container.Endpoint(ctx, "")
	require.NoError(t, err)
	rdb := redis.NewClient(&redis.Options{Addr: endpoint})

	sub := stream.NewRedisSubscriber(rdb)
	taskID := "task-test-1"

	subCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	ch, err := sub.Subscribe(subCtx, taskID)
	require.NoError(t, err)

	// 模拟 worker 写进度事件
	go func() {
		time.Sleep(200 * time.Millisecond)
		ev := stream.ProgressEvent{Type: "agent.progress", TaskID: taskID, Message: "halfway", Timestamp: time.Now().Unix()}
		data, _ := json.Marshal(ev)
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: "progress:" + taskID,
			Values: map[string]any{"data": string(data)},
		})
		done := stream.ProgressEvent{Type: "task.done", TaskID: taskID, Timestamp: time.Now().Unix()}
		data2, _ := json.Marshal(done)
		rdb.XAdd(ctx, &redis.XAddArgs{
			Stream: "progress:" + taskID,
			Values: map[string]any{"data": string(data2)},
		})
	}()

	var got []stream.ProgressEvent
	for ev := range ch {
		got = append(got, ev)
	}
	require.GreaterOrEqual(t, len(got), 2)
	require.Equal(t, "task.done", got[len(got)-1].Type)
}