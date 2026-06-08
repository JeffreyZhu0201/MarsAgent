package tests

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/marsagent/gateway/internal/api"
	"github.com/marsagent/gateway/internal/stream"
	"github.com/stretchr/testify/require"
)

// fakeProducer 实现 stream.TaskProducer 接口，用于隔离 Redis 依赖。
type fakeProducer struct {
	calls []stream.TaskEnvelope
}

func (f *fakeProducer) Enqueue(_ context.Context, e stream.TaskEnvelope) error {
	f.calls = append(f.calls, e)
	return nil
}

func TestEchoEnqueuesTask(t *testing.T) {
	fp := &fakeProducer{}
	r := api.NewRouter(api.Deps{Producer: fp})

	body, _ := json.Marshal(map[string]string{"msg": "hello"})
	req := httptest.NewRequest(http.MethodPost, "/api/echo", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["task_id"])

	require.Len(t, fp.calls, 1)
	require.Equal(t, "echo", fp.calls[0].Kind)
	require.Equal(t, resp["task_id"], fp.calls[0].TaskID)
}