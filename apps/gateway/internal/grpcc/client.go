// Package grpcc 把对 Python worker 的 gRPC 调用封装成强类型方法。
// 上层 handler 不直接 import 生成的 pb 包，便于未来切换契约。
package grpcc

import (
	"context"
	"fmt"
	"time"

	wikipb "github.com/marsagent/gateway/gen/proto/wiki"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type WikiClient struct {
	conn *grpc.ClientConn
	cli  wikipb.WikiRetrieverClient
}

// Dial 建立长连接。target 形如 "agents:50051" 或 "localhost:50051"。
func Dial(target string) (*WikiClient, error) {
	conn, err := grpc.NewClient(target,
		grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return nil, fmt.Errorf("grpc dial %s: %w", target, err)
	}
	return &WikiClient{conn: conn, cli: wikipb.NewWikiRetrieverClient(conn)}, nil
}

func (c *WikiClient) Close() error { return c.conn.Close() }

func (c *WikiClient) Ping(ctx context.Context, msg string) (string, string, error) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	resp, err := c.cli.Ping(ctx, &wikipb.PingReq{Msg: msg})
	if err != nil {
		return "", "", err
	}
	return resp.Echo, resp.ServerVersion, nil
}