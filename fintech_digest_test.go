package main

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRejectionIncludesStatusAndServerDetail(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"ok":false,"error":null,"message":"invalid batch request"}`))
	}))
	defer server.Close()
	client := NewInfraiClient(server.URL, "test")
	_, err := client.SubmitOvernightBatch(context.Background(), nil, "test")
	if err == nil || !strings.Contains(err.Error(), "HTTP 400") || !strings.Contains(err.Error(), "invalid batch request") {
		t.Fatalf("expected status and rejection detail, got %v", err)
	}
}

func TestAssessQueuesFailedAndHighValuePayments(t *testing.T) {
	events := []PaymentEvent{
		{ID: "settled-large", Amount: 100000, Status: "settled"},
		{ID: "failed-small", Amount: 99, Status: "failed"},
		{ID: "settled-small", Amount: 99, Status: "settled"},
	}
	actions, notices := Assess(events)
	if len(actions) != 2 || len(notices) != 2 {
		t.Fatalf("got %d actions and %d notices, want 2 of each", len(actions), len(notices))
	}
	if actions[0].EventID != "settled-large" || actions[0].Reason != "high value payment" {
		t.Fatalf("unexpected first action: %#v", actions[0])
	}
	if actions[1].EventID != "failed-small" || actions[1].Reason != "failed payment" {
		t.Fatalf("unexpected second action: %#v", actions[1])
	}
}
