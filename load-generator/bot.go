package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/signal"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/gorilla/websocket"
)

const (
	TargetURL    = "ws://localhost:8080/ws"
	NumWorkers   = 50               // Parallel bots spawning concurrent connections
	OrdersPerSec = 500              // Total target throughput across all workers
	Duration     = 10 * time.Second // Test duration
)

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
		// 1 microsecond (1,000 ns) to 10 seconds (10,000,000,000 ns), 3 significant digits
		hist: hdrhistogram.New(1_000, 10_000_000_000, 3),
	}
}

func (m *MetricsCollector) RecordLatency(nanos int64) {
	m.mu.Lock()
	_ = m.hist.RecordValue(nanos)
	m.mu.Unlock()
	m.totalAcks.Add(1)
}

// runBot runs an independent bot simulating a single market participant
func runBot(ctx context.Context, id int, metrics *MetricsCollector, wg *sync.WaitGroup) {
	defer wg.Done()

	conn, _, err := websocket.DefaultDialer.Dial(TargetURL, nil)
	if err != nil {
		log.Printf("[Bot %d] Failed to connect: %v", id, err)
		return
	}
	defer conn.Close()

	// In-flight map to measure T2 - T1 RTT
	var inflight sync.Map

	// Reader goroutine: reads ACKs and measures elapsed monotonic duration
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
				latencyNanos := time.Since(sentAt).Nanoseconds()
				metrics.RecordLatency(latencyNanos)
			}
		}
	}()

	// Worker pacing: distributes total rate across bots
	interval := time.Duration(float64(time.Second) / (float64(OrdersPerSec) / float64(NumWorkers)))
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	orderSeq := 0
	sides := []string{"BUY", "SELL"}

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			orderSeq++
			orderID := fmt.Sprintf("bot-%d-%d", id, orderSeq)
			side := sides[rand.Intn(2)]
			price := 99.0 + rand.Float64()*3.0 // prices around 99.00 - 102.00

			req := Order{
				OrderID:   orderID,
				Side:      side,
				Price:     float64(int(price*100)) / 100, // round to 2 decimals
				Quantity:  rand.Intn(10) + 1,
				Timestamp: time.Now().UnixNano(),
			}

			payload, _ := json.Marshal(req)

			// Record start timestamp immediately before wire push
			inflight.Store(orderID, time.Now())
			if err := conn.WriteMessage(websocket.TextMessage, payload); err != nil {
				inflight.Delete(orderID)
				return
			}
			metrics.totalSent.Add(1)
		}
	}
}

func main() {
	metrics := NewMetricsCollector()
	ctx, cancel := context.WithTimeout(context.Background(), Duration)
	defer cancel()

	// Intercept Ctrl+C
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-sigChan
		cancel()
	}()

	log.Printf("Starting load test: %d workers, target ~%d TPS for %v...", NumWorkers, OrdersPerSec, Duration)

	var wg sync.WaitGroup
	startTime := time.Now()

	for i := 0; i < NumWorkers; i++ {
		wg.Add(1)
		go runBot(ctx, i, metrics, &wg)
	}

	wg.Wait()
	totalDuration := time.Since(startTime)

	// Summary output
	sent := metrics.totalSent.Load()
	acks := metrics.totalAcks.Load()
	actualTPS := float64(acks) / totalDuration.Seconds()

	metrics.mu.Lock()
	defer metrics.mu.Unlock()

	fmt.Println("\n================= BENCHMARK RESULTS =================")
	fmt.Printf("Duration:         %.2fs\n", totalDuration.Seconds())
	fmt.Printf("Total Sent:       %d orders\n", sent)
	fmt.Printf("Total Acks:       %d orders\n", acks)
	fmt.Printf("Throughput (TPS): %.2f orders/sec\n", actualTPS)
	fmt.Println("----------------- Latency Profiles ------------------")
	fmt.Printf("p50 (Median):     %8.2f µs\n", float64(metrics.hist.ValueAtQuantile(50.0))/1000.0)
	fmt.Printf("p90:              %8.2f µs\n", float64(metrics.hist.ValueAtQuantile(90.0))/1000.0)
	fmt.Printf("p99:              %8.2f µs\n", float64(metrics.hist.ValueAtQuantile(99.0))/1000.0)
	fmt.Printf("p99.9:            %8.2f µs\n", float64(metrics.hist.ValueAtQuantile(99.9))/1000.0)
	fmt.Printf("Max Latency:      %8.2f µs\n", float64(metrics.hist.Max())/1000.0)
	fmt.Println("=====================================================")
}
