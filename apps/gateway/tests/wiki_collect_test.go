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

type collectFakeProducer struct {
	calls []stream.TaskEnvelope
}

func (f *collectFakeProducer) Enqueue(_ context.Context, e stream.TaskEnvelope) error {
	f.calls = append(f.calls, e)
	return nil
}

func TestWikiCollectEnqueuesCollectTask(t *testing.T) {
	fp := &collectFakeProducer{}
	r := api.NewRouter(api.Deps{Producer: fp})

	body, _ := json.Marshal(map[string]any{
		"topic":          "Python",
		"sources":        []string{"doc"},
		"max_per_source": 1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/wiki/collect", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusAccepted, w.Code)

	var resp map[string]string
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	require.NotEmpty(t, resp["task_id"])

	require.Len(t, fp.calls, 1)
	require.Equal(t, "wiki.collect", fp.calls[0].Kind)
	require.Equal(t, resp["task_id"], fp.calls[0].TaskID)

	var args map[string]any
	require.NoError(t, json.Unmarshal(fp.calls[0].Args, &args))
	require.Equal(t, "Python", args["topic"])
	require.Equal(t, float64(1), args["max_per_source"])
}
