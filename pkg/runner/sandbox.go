package runner

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

// VMConfig holds the specs for a new Unikraft Cloud virtual machine instance.
type VMConfig struct {
	ImageName string            `json:"image_name"`
	Env       map[string]string `json:"env"`
	MemoryMB  int               `json:"memory_mb"`
}

// VMInstance represents the operational and network identity of a multi-tenant VM.
type VMInstance struct {
	ID    string `json:"id"`
	State string `json:"state"`
	IP    string `json:"ip"`
}

// ExecutionSandbox defines the deep seam targeting multi-tenant VM provision and telemetry.
type ExecutionSandbox interface {
	CreateInstance(ctx context.Context, config VMConfig) (*VMInstance, error)
	GetInstance(ctx context.Context, id string) (*VMInstance, error)
	DeleteInstance(ctx context.Context, id string) error
	GetInstanceLogs(ctx context.Context, id string) (string, error)
}

type unikraftCloudSandboxAdapter struct {
	baseURL   string
	authToken string
	client    *http.Client
}

// NewUnikraftCloudSandboxAdapter returns a concrete ExecutionSandbox backed by standard REST integrations.
func NewUnikraftCloudSandboxAdapter(baseURL, authToken string) ExecutionSandbox {
	return &unikraftCloudSandboxAdapter{
		baseURL:   baseURL,
		authToken: authToken,
		client:    &http.Client{},
	}
}

func (a *unikraftCloudSandboxAdapter) CreateInstance(ctx context.Context, config VMConfig) (*VMInstance, error) {
	url := fmt.Sprintf("%s/v1/instances", a.baseURL)
	bodyBytes, err := json.Marshal(config)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal VMConfig: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.authToken))
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated && resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var instance VMInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &instance, nil
}

func (a *unikraftCloudSandboxAdapter) GetInstance(ctx context.Context, id string) (*VMInstance, error) {
	url := fmt.Sprintf("%s/v1/instances/%s", a.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.authToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var instance VMInstance
	if err := json.NewDecoder(resp.Body).Decode(&instance); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return &instance, nil
}

func (a *unikraftCloudSandboxAdapter) DeleteInstance(ctx context.Context, id string) error {
	url := fmt.Sprintf("%s/v1/instances/%s", a.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, url, nil)
	if err != nil {
		return fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.authToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	return nil
}

func (a *unikraftCloudSandboxAdapter) GetInstanceLogs(ctx context.Context, id string) (string, error) {
	url := fmt.Sprintf("%s/v1/instances/%s/logs", a.baseURL, id)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("failed to create http request: %w", err)
	}

	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", a.authToken))

	resp, err := a.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var logResp struct {
		Logs string `json:"logs"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&logResp); err != nil {
		return "", fmt.Errorf("failed to decode response: %w", err)
	}

	return logResp.Logs, nil
}
