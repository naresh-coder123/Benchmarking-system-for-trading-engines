package ingester

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"

	"github.com/HdrHistogram/hdrhistogram-go"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"

	pb "grpc/proto"
)

type WorkerState struct {
	LastAcks      uint64
	LastTimestamp int64
}
type ProcessedMetricEvent struct {
	RunID     string  `json:"run_id"`
	TeamID    string  `json:"team_id"`
	WorkerID  string  `json:"worker_id"`
	Timestamp int64   `json:"timestamp_ns"`
	TPS       float64 `json:"tps"`
	P50Micro  float64 `json:"p50_us"`
	P90Micro  float64 `json:"p90_us"`
	P99Micro  float64 `json:"p99_us"`
	TotalSent uint64  `json:"total_sent"`
	TotalAcks uint64  `json:"total_acks"`
	Score     float64 `json:"score"`
}

type IngesterServer struct {
	pb.UnimplementedTelemetryIngesterServer
	kafkaClient *kgo.Client
	states      map[string]*WorkerState
	mu          sync.Mutex
}

func NewIngesterServer(brokers []string) (*IngesterServer, error) {
	kClient, err := kgo.NewClient(
		kgo.SeedBrokers(brokers...),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Redpanda client: %w", err)
	}

	return &IngesterServer{kafkaClient: kClient, states: make(map[string]*WorkerState), mu: sync.Mutex{}}, nil
}

func (s *IngesterServer) StreamMetrics(stream pb.TelemetryIngester_StreamMetricsServer) error {
	for {
		snapshot, err := stream.Recv()
		if err == io.EOF {
			return stream.SendAndClose(&pb.IngestAck{Success: true, Message: "Stream closed"})
		}
		if err != nil {
			return err
		}

		// 1. Decode binary HdrHistogram snapshot
		hist, err := hdrhistogram.Decode(snapshot.HdrHistogramBlob)
		if err != nil {
			log.Printf("failed to decode snapshot bytes: %v", err)
			continue
		}

		// 2. Extract Microsecond Percentiles
		p50Us := float64(hist.ValueAtQuantile(50.0)) / 1000.0
		p90Us := float64(hist.ValueAtQuantile(90.0)) / 1000.0
		p99Us := float64(hist.ValueAtQuantile(99.0)) / 1000.0

		// 3. Compute TPS and Score Formula
		p99Ms := p99Us / 1000.0
		if p99Ms <= 0 {
			p99Ms = 0.01 // Protect against divide-by-zero
		}

		stateKey := snapshot.TeamId + "-" + snapshot.WorkerId

		s.mu.Lock()
		state, exists := s.states[stateKey]
		if !exists {
			state = &WorkerState{
				LastAcks:      0,
				LastTimestamp: snapshot.TimestampNs - int64(time.Second), // Assume 1s initial window
			}
			s.states[stateKey] = state
		}

		// Calculate Deltas
		deltaAcks := snapshot.TotalAcks - state.LastAcks
		deltaTimeSec := float64(snapshot.TimestampNs-state.LastTimestamp) / 1e9

		// Fallback guard to prevent division by zero or negative time warps
		if deltaTimeSec <= 0 {
			deltaTimeSec = 1.0
		}

		// True Rolling TPS rate
		rollingTPS := float64(deltaAcks) / deltaTimeSec

		// Update historical trackers for the next ticker tick
		state.LastAcks = snapshot.TotalAcks
		state.LastTimestamp = snapshot.TimestampNs
		s.mu.Unlock()
		// ────────────────────────────────────────────────────────────────

		// Score now utilizes true window throughput instead of total historical volume
		score := rollingTPS / p99Ms

		event := ProcessedMetricEvent{
			TeamID:    snapshot.TeamId,
			WorkerID:  snapshot.WorkerId,
			Timestamp: snapshot.TimestampNs,
			TPS:       rollingTPS,
			P50Micro:  p50Us,
			P90Micro:  p90Us,
			P99Micro:  p99Us,
			TotalSent: snapshot.TotalSent,
			TotalAcks: snapshot.TotalAcks,
			Score:     score,
		}

		// 4. Publish Processed Metrics to Redpanda
		payload, _ := json.Marshal(event)
		s.kafkaClient.Produce(context.Background(), &kgo.Record{
			Topic: "telemetry-metrics",
			Key:   []byte(snapshot.TeamId),
			Value: payload,
		}, nil)

		log.Printf("[Ingest] Team: %-10s | TPS: %6.1f | p50: %6.2f µs | p99: %6.2f µs | Score: %8.2f",
			event.TeamID, event.TPS, event.P50Micro, event.P99Micro, event.Score)
	}
}

// StartIngesterServer boots up the gRPC ingestion service as a callable function
func StartIngesterServer(port string, kafkaBrokers []string) error {
	lis, err := net.Listen("tcp", port)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", port, err)
	}

	server, err := NewIngesterServer(kafkaBrokers)
	if err != nil {
		return fmt.Errorf("failed to setup ingester server: %w", err)
	}

	grpcServer := grpc.NewServer()
	pb.RegisterTelemetryIngesterServer(grpcServer, server)

	log.Printf("Telemetry Ingester Daemon running on gRPC port %s...", port)
	return grpcServer.Serve(lis)
}
