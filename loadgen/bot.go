package loadgen

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gorilla/websocket"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "grpc/proto"
)

type Config struct {
	TargetURL    string
	IngesterAddr string
	TeamID       string
	RunID        string
	NumWorkers   int
	OrdersPerSec int
	Duration     time.Duration
}

type Order struct {
	OrderID   string  `json:"order_id"`
	Side      string  `json:"side"`
	Price     float64 `json:"price"`
	Quantity  int     `json:"quantity"`
	Timestamp int64   `json:"timestamp"`
}

type ExecutionAck struct {
	OrderID    string `json:"order_id"`
	Status     string `json:"status"`
	ServerTime int64  `json:"server_time"`
}

type MetricsCollector struct {
	hist      *hdrhistogram.Histogram
	mu        sync.Mutex
	totalSent atomic.Uint64
	totalAcks atomic.Uint64
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		hist: hdrhistogram.New(1_000, 10_000_000_000, 3), // 1 µs to 10 s
	}
}

func (m *MetricsCollector) RecordLatency(nanos int64) {
	m.mu.Lock()
	_ = m.hist.RecordValue(nanos)
	m.mu.Unlock()
	m.totalAcks.Add(1)
}

func runBot(ctx context.Context, id int, cfg Config, metrics *MetricsCollector, wg *sync.WaitGroup) {
	defer wg.Done()
	var conn *websocket.Conn
	var err error
	for i := 0; i < 5; i++ {
		conn, _, err = websocket.DefaultDialer.Dial(cfg.TargetURL, nil)
		if err == nil {
			break
		}
		log.Printf("Connection failed: %v. Retrying...", err)
		time.Sleep(1 * time.Second)
	}
	if err != nil {
		log.Fatalf("Bot %d could not connect after 5 attempts", id)
	}
	defer conn.Close()

	var inflight sync.Map

	go func() {
		for {
			_, msg, err := conn.ReadMessage()
			if err != nil {
				return
			}

			var ack ExecutionAck
			if err := json.Unmarshal(msg, &ack); err != nil {
				continue
			}

			if sentAtVal, ok := inflight.LoadAndDelete(ack.OrderID); ok {
				sentAt := sentAtVal.(time.Time)
				metrics.RecordLatency(time.Since(sentAt).Nanoseconds())
			}
		}
	}()

	interval := time.Duration(float64(time.Second) / (float64(cfg.OrdersPerSec) / float64(cfg.NumWorkers)))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	orderSeq := 0
	sides := []string{"BUY", "SELL"}
	rng := rand.New(rand.NewSource(time.Now().UnixNano() + int64(id)))

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orderSeq++
			orderID := fmt.Sprintf("bot-%d-%d", id, orderSeq)
			side := sides[rng.Intn(2)]
			price := 99.0 + rng.Float64()*3.0

			req := Order{
				OrderID:   orderID,
				Side:      side,
				Price:     float64(int(price*100)) / 100,
				Quantity:  rng.Intn(10) + 1,
				Timestamp: time.Now().UnixNano(),
			}

			payload, _ := json.Marshal(req)
			inflight.Store(orderID, time.Now())
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				inflight.Delete(orderID)
				return
			}
			metrics.totalSent.Add(1)
		}
	}
}

func streamMetricsToIngester(ctx context.Context, cfg Config, metrics *MetricsCollector) {
	conn, err := grpc.Dial(cfg.IngesterAddr, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Printf("[gRPC] Connection to Ingester failed: %v", err)
		return
	}
	defer conn.Close()

	client := pb.NewTelemetryIngesterClient(conn)
	stream, err := client.StreamMetrics(ctx)
	if err != nil {
		log.Printf("[gRPC] Metric stream creation failed for run %s: %v", cfg.RunID, err)
		return
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			_, _ = stream.CloseAndRecv()
			return
		case <-ticker.C:
			metrics.mu.Lock()
			hdrBlob, err := metrics.hist.Encode(hdrhistogram.V2CompressedEncodingCookieBase)
			metrics.mu.Unlock()
			if err != nil {
				log.Printf("error encoding histogram: %v", err)
			}

			snapshot := &pb.MetricSnapshot{
				TeamId:           cfg.TeamID,
				WorkerId:         fmt.Sprintf("load-gen-%s", cfg.RunID[:8]),
				TimestampNs:      time.Now().UnixNano(),
				TotalSent:        metrics.totalSent.Load(),
				TotalAcks:        metrics.totalAcks.Load(),
				HdrHistogramBlob: hdrBlob,
			}

			if err := stream.Send(snapshot); err != nil {
				log.Printf("[gRPC] Error sending telemetry snapshot: %v", err)
				return
			}
		}
	}
}

// RunBenchmark is the exported function called directly by the Orchestrator
func RunBenchmark(cfg Config) error {
	metrics := NewMetricsCollector()
	ctx, cancel := context.WithTimeout(context.Background(), cfg.Duration)
	defer cancel()

	log.Printf("[Run: %s] Starting load test for team %s: %d workers, target ~%d TPS for %v...",
		cfg.RunID, cfg.TeamID, cfg.NumWorkers, cfg.OrdersPerSec, cfg.Duration)

	go streamMetricsToIngester(ctx, cfg, metrics)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < cfg.NumWorkers; i++ {
		wg.Add(1)
		go runBot(ctx, i, cfg, metrics, &wg)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	log.Printf("[Run: %s] Completed! Sent: %d, Acks: %d, Actual TPS: %.2f",
		cfg.RunID, metrics.totalSent.Load(), metrics.totalAcks.Load(), float64(metrics.totalAcks.Load())/totalDuration.Seconds())
	return nil
}
