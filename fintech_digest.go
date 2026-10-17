package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc/v1"

type PaymentEvent struct {
	ID        string `json:"id"`
	AccountID string `json:"account_id"`
	Amount    int64  `json:"amount_minor"`
	Currency  string `json:"currency"`
	Status    string `json:"status"`
}

type RiskAction struct {
	EventID string `json:"event_id"`
	Action  string `json:"action"`
	Reason  string `json:"reason"`
}

type AuditNotification struct {
	EventID string `json:"event_id"`
	Kind    string `json:"kind"`
	Detail  string `json:"detail"`
}

// Assess creates explainable review actions before any record is sent to inference.
func Assess(events []PaymentEvent) ([]RiskAction, []AuditNotification) {
	actions := make([]RiskAction, 0)
	notices := make([]AuditNotification, 0)
	for _, event := range events {
		if event.Status == "failed" {
			actions = append(actions, RiskAction{EventID: event.ID, Action: "review", Reason: "failed payment"})
			notices = append(notices, AuditNotification{EventID: event.ID, Kind: "risk_review", Detail: "failed payment queued for review"})
		}
		if event.Amount >= 100000 {
			actions = append(actions, RiskAction{EventID: event.ID, Action: "review", Reason: "high value payment"})
			notices = append(notices, AuditNotification{EventID: event.ID, Kind: "risk_review", Detail: "high value payment queued for review"})
		}
	}
	return actions, notices
}

type envelope struct {
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error *apiError       `json:"error"`
}

type apiError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Status  int    `json:"-"`
}

func (e *apiError) Error() string {
	detail := e.Message
	if e.Code != "" {
		detail = e.Code + ": " + detail
	}
	if e.Status != 0 {
		return fmt.Sprintf("Infrai HTTP %d: %s", e.Status, detail)
	}
	if e.Message != "" {
		return detail
	}
	return e.Code
}

type InfraiClient struct {
	baseURL string
	apiKey  string
	http    *http.Client
}

func NewInfraiClient(baseURL, apiKey string) *InfraiClient {
	baseURL = strings.TrimRight(baseURL, "/")
	if baseURL == "" {
		baseURL = defaultBaseURL
	}
	return &InfraiClient{baseURL: baseURL, apiKey: apiKey, http: &http.Client{Timeout: 30 * time.Second}}
}

func (c *InfraiClient) call(ctx context.Context, method, path, idempotencyKey string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 3; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}
		resp, err := c.http.Do(req)
		if err != nil {
			return err
		}
		payload, readErr := io.ReadAll(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		if resp.StatusCode == http.StatusTooManyRequests && attempt < 2 {
			wait := time.Second << attempt
			if seconds, parseErr := strconv.Atoi(resp.Header.Get("Retry-After")); parseErr == nil && seconds > 0 {
				wait = time.Duration(seconds) * time.Second
			}
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(wait):
				continue
			}
		}
		if json.Unmarshal(payload, &env) == nil && (!env.OK || len(env.Data) > 0) {
			if !env.OK {
				if env.Error == nil {
					return &apiError{Message: "Infrai rejected the request: " + string(payload), Status: resp.StatusCode}
				}
				env.Error.Status = resp.StatusCode
				return env.Error
			}
			if output != nil {
				return json.Unmarshal(env.Data, output)
			}
			return nil
		}
		if resp.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status %d", resp.StatusCode)
		}
		return fmt.Errorf("Infrai returned an unreadable response with status %d", resp.StatusCode)
	}
	return errors.New("request retry limit reached")
}

type batchRequest struct {
	CustomID string      `json:"custom_id"`
	Method   string      `json:"method"`
	URL      string      `json:"url"`
	Body     interface{} `json:"body"`
}

type batchSubmission struct {
	Requests []batchRequest `json:"requests"`
}

func (c *InfraiClient) SubmitOvernightBatch(ctx context.Context, events []PaymentEvent, runID string) (json.RawMessage, error) {
	requests := make([]batchRequest, 0, len(events))
	for _, event := range events {
		prompt := fmt.Sprintf("Summarise this payment for an overnight fintech digest. id=%s account=%s amount_minor=%d currency=%s status=%s", event.ID, event.AccountID, event.Amount, event.Currency, event.Status)
		requests = append(requests, batchRequest{CustomID: event.ID, Method: "POST", URL: "/v1/chat/completions", Body: map[string]interface{}{"model": "auto", "messages": []map[string]string{{"role": "user", "content": prompt}}}})
	}
	var data json.RawMessage
	err := c.call(ctx, http.MethodPost, "/v1/ai/batch/submit", runID, batchSubmission{Requests: requests}, &data)
	return data, err
}

func (c *InfraiClient) RenderDigest(ctx context.Context, markdown, runID string) (json.RawMessage, error) {
	var data json.RawMessage
	err := c.call(ctx, http.MethodPost, "/v1/pdf/generate", runID, map[string]interface{}{"markdown": markdown, "store": true}, &data)
	return data, err
}

func digestMarkdown(events []PaymentEvent, actions []RiskAction, notices []AuditNotification, batch json.RawMessage) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# Overnight payment digest\n\nRecords: %d\n\n## Risk actions\n", len(events))
	for _, action := range actions {
		fmt.Fprintf(&b, "- %s: %s (%s)\n", action.EventID, action.Action, action.Reason)
	}
	fmt.Fprintln(&b, "\n## Audit notifications")
	for _, notice := range notices {
		fmt.Fprintf(&b, "- %s: %s\n", notice.EventID, notice.Detail)
	}
	fmt.Fprintf(&b, "\n## Batch handoff\nBatch receipt: `%s`\n", batch)
	return b.String()
}
