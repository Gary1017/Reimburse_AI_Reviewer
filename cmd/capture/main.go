// capture connects to Lark WebSocket with go-vcr recording all HTTP
// interactions (GetInstanceDetail, token refresh, etc.) to cassettes.
// It also saves raw WebSocket event bodies as JSON fixtures.
//
// Usage:
//
//	go run cmd/capture/main.go
//	# Raise an approval in Lark, then press Ctrl+C.
//	# Cassettes are written to testdata/lark_fixtures/
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"syscall"
	"time"

	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher"
	"github.com/larksuite/oapi-sdk-go/v3/ws"
	"go.uber.org/zap"
	"gopkg.in/dnaeon/go-vcr.v4/pkg/recorder"

	"github.com/garyjia/ai-reimbursement/internal/config"
	infraLark "github.com/garyjia/ai-reimbursement/internal/infrastructure/external/lark"
)

const outDir = "testdata/lark_fixtures"

var (
	mu  sync.Mutex
	seq int
)

func main() {
	cfg, err := config.Load("configs/config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	if err := os.MkdirAll(outDir, 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create output dir: %v\n", err)
		os.Exit(1)
	}

	logger, _ := zap.NewDevelopment()

	// Set up go-vcr recorder in record-only mode.
	// All Lark HTTP API calls (token, GetInstanceDetail, etc.) are captured.
	cassettePath := filepath.Join(outDir, "lark_api")
	rec, err := recorder.New(
		cassettePath,
		recorder.WithMode(recorder.ModeRecordOnly),
		recorder.WithSkipRequestLatency(true),
	)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create recorder: %v\n", err)
		os.Exit(1)
	}

	// Create Lark SDK client with go-vcr's recording HTTP client
	sdkClient := infraLark.NewSDKClient(infraLark.Config{
		AppID:        cfg.Lark.AppID,
		AppSecret:    cfg.Lark.AppSecret,
		ApprovalCode: cfg.Lark.ApprovalCode,
	}, logger, rec.GetDefaultClient())

	// Create dispatcher that captures raw events AND triggers API calls
	d := dispatcher.NewEventDispatcher("", "")
	d.OnCustomizedEvent("approval_instance", func(ctx context.Context, event *larkevent.EventReq) error {
		return captureEvent(ctx, event, sdkClient, logger)
	})
	d.OnCustomizedEvent("approval_task", func(ctx context.Context, event *larkevent.EventReq) error {
		return captureEvent(ctx, event, sdkClient, logger)
	})

	wsClient := ws.NewClient(
		cfg.Lark.AppID,
		cfg.Lark.AppSecret,
		ws.WithEventHandler(d),
	)

	fmt.Println("[capture] go-vcr recording mode")
	fmt.Printf("[capture] Cassette: %s.yaml\n", cassettePath)
	fmt.Printf("[capture] Events:   %s/\n", outDir)
	fmt.Println("[capture] Connecting to Lark WebSocket... Raise an approval, then Ctrl+C.")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
		fmt.Println("\n[capture] Shutting down, saving cassette...")
		cancel()
	}()

	if err := wsClient.Start(ctx); err != nil {
		// Context cancellation is expected on Ctrl+C
		if ctx.Err() == nil {
			fmt.Fprintf(os.Stderr, "WebSocket error: %v\n", err)
		}
	}

	// Stop recorder to flush cassette to disk
	if err := rec.Stop(); err != nil {
		fmt.Fprintf(os.Stderr, "[capture] Failed to save cassette: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("[capture] Cassette saved to %s.yaml\n", cassettePath)
}

func captureEvent(ctx context.Context, event *larkevent.EventReq, sdkClient *infraLark.SDKClient, logger *zap.Logger) error {
	mu.Lock()
	seq++
	n := seq
	mu.Unlock()

	ts := time.Now().Format("20060102_150405")
	prefix := fmt.Sprintf("%s_%02d", ts, n)

	// 1. Save raw WebSocket event body (not HTTP, so go-vcr doesn't capture this)
	eventFile := filepath.Join(outDir, prefix+"_event.json")
	pretty := prettyJSON(event.Body)
	if err := os.WriteFile(eventFile, pretty, 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "[capture] Failed to write event: %v\n", err)
	} else {
		fmt.Printf("[capture] Saved event -> %s (%d bytes)\n", eventFile, len(pretty))
	}

	// 2. Call GetInstanceDetail — this HTTP call is recorded by go-vcr
	var parsed struct {
		Event struct {
			InstanceCode string `json:"instance_code"`
		} `json:"event"`
	}
	if err := json.Unmarshal(event.Body, &parsed); err == nil && parsed.Event.InstanceCode != "" {
		code := parsed.Event.InstanceCode
		fmt.Printf("[capture] Calling GetInstanceDetail(%s) — recorded by go-vcr\n", code)

		api := infraLark.NewApprovalAPI(sdkClient, logger)
		detail, err := api.GetInstanceDetail(ctx, code)
		if err != nil {
			fmt.Fprintf(os.Stderr, "[capture] GetInstanceDetail error: %v\n", err)
		} else {
			// Also save as JSON for easy inspection
			detailBytes, _ := json.MarshalIndent(detail, "", "  ")
			detailFile := filepath.Join(outDir, prefix+"_instance_detail.json")
			_ = os.WriteFile(detailFile, detailBytes, 0o644)
			fmt.Printf("[capture] Saved detail  -> %s (%d bytes)\n", detailFile, len(detailBytes))
		}
	}

	return nil
}

func prettyJSON(raw []byte) []byte {
	var v interface{}
	if err := json.Unmarshal(raw, &v); err != nil {
		return raw
	}
	out, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return raw
	}
	return out
}
