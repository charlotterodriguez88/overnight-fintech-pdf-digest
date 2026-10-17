package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
)

func main() {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		fmt.Fprintln(os.Stderr, "INFRAI_API_KEY is required")
		os.Exit(2)
	}
	events := []PaymentEvent{
		{ID: "pay-104", AccountID: "acct-22", Amount: 125000, Currency: "USD", Status: "settled"},
		{ID: "pay-105", AccountID: "acct-22", Amount: 2400, Currency: "USD", Status: "failed"},
	}
	client := NewInfraiClient(os.Getenv("INFRAI_BASE_URL"), key)
	runID := "overnight-20260914"
	batch, err := client.SubmitOvernightBatch(context.Background(), events, runID)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	actions, notices := Assess(events)
	pdf, err := client.RenderDigest(context.Background(), digestMarkdown(events, actions, notices, batch), runID+"-pdf")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	encoded, _ := json.MarshalIndent(pdf, "", "  ")
	fmt.Println(string(encoded))
}
