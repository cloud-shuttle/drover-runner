package runner_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cloud-shuttle/drover-runner/pkg/runner"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestExecutionSandbox_SpinsUpVMAndExtractsLogs(t *testing.T) {
	// Setup a mock Unikraft Cloud REST server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")

		// Handle Instance Creation
		if r.Method == http.MethodPost && r.URL.Path == "/v1/instances" {
			var body map[string]interface{}
			if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
				w.WriteHeader(http.StatusBadRequest)
				return
			}
			w.WriteHeader(http.StatusCreated)
			_, _ = w.Write([]byte(`{
				"id": "inst-12345",
				"state": "starting",
				"ip": "10.0.0.5"
			}`))
			return
		}

		// Handle Instance Status/Get
		if r.Method == http.MethodGet && r.URL.Path == "/v1/instances/inst-12345" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{
				"id": "inst-12345",
				"state": "running",
				"ip": "10.0.0.5"
			}`))
			return
		}

		// Handle Instance Logs
		if r.Method == http.MethodGet && r.URL.Path == "/v1/instances/inst-12345/logs" {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte(`{"logs": "Hello, Unikraft! Multi-tenant VM successfully booted.\n"}`))
			return
		}

		// Handle Instance Deletion
		if r.Method == http.MethodDelete && r.URL.Path == "/v1/instances/inst-12345" {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	// Instantiate the adapter pointing to the mock server
	adapter := runner.NewUnikraftCloudSandboxAdapter(server.URL, "mock-auth-token")

	// 1. Spin up VM
	cfg := runner.VMConfig{
		ImageName: "unikraft/helloworld-go:latest",
		Env: map[string]string{
			"PORT": "8080",
		},
		MemoryMB: 128,
	}
	instance, err := adapter.CreateInstance(ctx, cfg)
	require.NoError(t, err)
	assert.Equal(t, "inst-12345", instance.ID)
	assert.Equal(t, "starting", instance.State)

	// 2. Query status
	status, err := adapter.GetInstance(ctx, instance.ID)
	require.NoError(t, err)
	assert.Equal(t, "running", status.State)
	assert.Equal(t, "10.0.0.5", status.IP)

	// 3. Extract logs
	logs, err := adapter.GetInstanceLogs(ctx, instance.ID)
	require.NoError(t, err)
	assert.Contains(t, logs, "Hello, Unikraft!")

	// 4. Terminate VM
	err = adapter.DeleteInstance(ctx, instance.ID)
	require.NoError(t, err)
}
