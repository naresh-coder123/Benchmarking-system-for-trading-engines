package consumer

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"

	_ "github.com/lib/pq"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
)

type MetricEvent struct {
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

type RunResultEvent struct {
	RunID       string  `json:"run_id"`
	TeamID      string  `json:"team_id"`
	Status      string  `json:"status"`      // COMPLETED, FAILED
	Correctness string  `json:"correctness"` // PASS, FAIL
	FinalScore  float64 `json:"final_score"`
	FinalTPS    float64 `json:"final_tps"`
	P99Micro    float64 `json:"p99_us"`
}

// StartConsumerService initializes and runs the Kafka consumer daemon
func StartConsumerService(ctx context.Context, kafkaBrokers []string, pgConnStr string, redisAddr string) error {
	// 1. Connect to PostgreSQL
	db, err := sql.Open("postgres", pgConnStr)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer db.Close()

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to connect to Redis: %w", err)
	}

	// 3. Connect to Redpanda / Kafka (Consuming from both topics)
	topics := []string{"telemetry-metrics", "run-results"}
	kClient, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers...),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumerGroup("platform-consumer-group"),
	)
	if err != nil {
		return fmt.Errorf("failed to create Kafka consumer: %w", err)
	}
	defer kClient.Close()

	log.Println("Consumer service daemon started, listening on Kafka topics: telemetry-metrics, run-results...")

	// 4. Consumption Loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down consumer service daemon...")
			return nil
		default:
			fetches := kClient.PollFetches(ctx)
			if fetches.IsClientClosed() {
				return nil
			}

			fetches.EachRecord(func(record *kgo.Record) {
				switch record.Topic {
				case "telemetry-metrics":
					var event MetricEvent
					if err := json.Unmarshal(record.Value, &event); err != nil {
						log.Printf("Failed to unmarshal metric event: %v", err)
						return
					}

					// A. Store raw timeseries metrics in PostgreSQL/TimescaleDB
					_, _ = db.Exec(
						`INSERT INTO telemetry_timeseries (time, run_id, team_id, tps, p50_us, p90_us, p99_us) 
						 VALUES (TO_TIMESTAMP($1 / 1000000000.0), $2, $3, $4, $5, $6, $7)`,
						event.Timestamp, event.RunID, event.TeamID, event.TPS, event.P50Micro, event.P90Micro, event.P99Micro,
					)

					// B. Broadcast live metrics instantly to Redis Pub/Sub for frontend WebSockets
					pubPayload, _ := json.Marshal(event)
					_ = rdb.Publish(ctx, "channel:live-metrics", pubPayload).Err()

				case "run-results":
					var runEvent RunResultEvent
					if err := json.Unmarshal(record.Value, &runEvent); err != nil {
						log.Printf("Failed to unmarshal run result event: %v", err)
						return
					}

					// A. Upsert final run results in PostgreSQL for historical auditing
					_, err := db.Exec(
						`INSERT INTO benchmark_runs (run_id, team_id, status, correctness, final_score, final_tps, p99_us)
						 VALUES ($1, $2, $3, $4, $5, $6, $7)
						 ON CONFLICT (run_id) DO UPDATE SET 
						   status = EXCLUDED.status,
						   correctness = EXCLUDED.correctness,
						   final_score = EXCLUDED.final_score,
						   final_tps = EXCLUDED.final_tps,
						   p99_us = EXCLUDED.p99_us`,
						runEvent.RunID, runEvent.TeamID, runEvent.Status, runEvent.Correctness,
						runEvent.FinalScore, runEvent.FinalTPS, runEvent.P99Micro,
					)
					if err != nil {
						log.Printf("Failed to upsert benchmark run result: %v", err)
						return
					}

					// B. Update Redis Sorted Set (`leaderboard:scores`) for O(log(N)) ranked queries
					// Only index runs that pass correctness and complete successfully
					if runEvent.Correctness == "PASS" && runEvent.Status == "COMPLETED" {
						err = rdb.ZAdd(ctx, "leaderboard:scores", redis.Z{
							Score:  runEvent.FinalScore,
							Member: runEvent.TeamID,
						}).Err()
						if err != nil {
							log.Printf("Failed to update Redis sorted set leaderboard: %v", err)
						}
					}

					log.Printf("[Leaderboard Updated] Team: %s | Score: %.2f | Correctness: %s",
						runEvent.TeamID, runEvent.FinalScore, runEvent.Correctness)
				}
			})
		}
	}
}
