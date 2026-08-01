package simulator

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"equipment-telemetry-simulator/internal/model"
)

type PushClient struct {
	targetURL  string
	httpClient *http.Client
}

func NewPushClient(targetURL string) *PushClient {
	return &PushClient{
		targetURL: targetURL,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (c *PushClient) Push(ctx context.Context, assets []model.Asset) error {
	payload := struct {
		SentAt time.Time     `json:"sentAt"`
		Assets []model.Asset `json:"assets"`
	}{
		SentAt: time.Now().UTC(),
		Assets: assets,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal push payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.targetURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build push request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send push request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return fmt.Errorf("push target returned %s", resp.Status)
	}
	return nil
}
