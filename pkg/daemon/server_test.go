package daemon

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

type mockDriver struct {
	launched map[string]bool
}

func (d *mockDriver) LaunchVM(ctx context.Context, id string, imageName string, memoryMB int, env map[string]string) (*ActiveVM, error) {
	d.launched[id] = true
	return &ActiveVM{
		ID:    id,
		IP:    "192.168.1.1",
		State: "running",
		Stop: func() error {
			d.launched[id] = false
			return nil
		},
		Logs: func() (string, error) {
			return "Mock VM active logs\n", nil
		},
	}, nil
}

func TestServerAuthorization(t *testing.T) {
	cfg := Config{
		AuthToken: "secret-token",
	}
	driver := &mockDriver{launched: make(map[string]bool)}
	server := NewServer(cfg, driver)

	// Unauthorized Request
	req, _ := http.NewRequest(http.MethodPost, "/v1/instances", bytes.NewBufferString("{}"))
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401 Unauthorized, got %d", rr.Code)
	}

	// Authorized Request
	req, _ = http.NewRequest(http.MethodPost, "/v1/instances", bytes.NewBufferString("{}"))
	req.Header.Set("Authorization", "Bearer secret-token")
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code == http.StatusUnauthorized {
		t.Error("expected access, but got 401 Unauthorized")
	}
}

func TestServerInstanceLifecycle(t *testing.T) {
	cfg := Config{
		AuthToken: "admin-token",
	}
	driver := &mockDriver{launched: make(map[string]bool)}
	server := NewServer(cfg, driver)

	// 1. Create VM Instance
	cfgPayload := VMConfig{
		ImageName: "helloworld-test",
		MemoryMB:  128,
		Env:       map[string]string{"ENV_VAR": "value"},
	}
	body, _ := json.Marshal(cfgPayload)
	req, _ := http.NewRequest(http.MethodPost, "/v1/instances", bytes.NewReader(body))
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusCreated {
		t.Fatalf("expected 201 Created, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var inst VMInstance
	if err := json.NewDecoder(rr.Body).Decode(&inst); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if inst.ID != "inst-1" {
		t.Errorf("expected ID 'inst-1', got %s", inst.ID)
	}
	if inst.State != "running" {
		t.Errorf("expected State 'running', got %s", inst.State)
	}

	// 2. Query VM Instance Status
	req, _ = http.NewRequest(http.MethodGet, "/v1/instances/inst-1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var status VMInstance
	_ = json.NewDecoder(rr.Body).Decode(&status)
	if status.ID != "inst-1" || status.State != "running" {
		t.Errorf("invalid queried state: %+v", status)
	}

	// 3. Query VM Logs
	req, _ = http.NewRequest(http.MethodGet, "/v1/instances/inst-1/logs", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected 200 OK, got %d", rr.Code)
	}

	var logsResp map[string]string
	_ = json.NewDecoder(rr.Body).Decode(&logsResp)
	if logsResp["logs"] != "Mock VM active logs\n" {
		t.Errorf("expected mock logs, got %q", logsResp["logs"])
	}

	// 4. Delete / Stop VM Instance
	req, _ = http.NewRequest(http.MethodDelete, "/v1/instances/inst-1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNoContent {
		t.Fatalf("expected 204 No Content, got %d", rr.Code)
	}

	if driver.launched["inst-1"] {
		t.Error("expected VM to be stopped, but mock reports it is still running")
	}

	// 5. Query Deleted Status (Should be 404)
	req, _ = http.NewRequest(http.MethodGet, "/v1/instances/inst-1", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr = httptest.NewRecorder()
	server.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404 Not Found for deleted instance, got %d", rr.Code)
	}
}
