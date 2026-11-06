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

// MetricEvent perfectly aligned with producer JSON output structural fields
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
		return fmt.Errorf("failed to initialize db connection string: %w", err)
	}
	defer db.Close()

	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("failed to ping Postgres database server: %w", err)
	}
	log.Println("✅ Successfully connected to PostgreSQL database.")

	// 2. Connect to Redis
	rdb := redis.NewClient(&redis.Options{
		Addr: redisAddr,
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		return fmt.Errorf("failed to ping Redis cluster instance: %w", err)
	}
	log.Println("✅ Successfully connected to Redis engine.")

	// 3. Connect to Redpanda / Kafka
	topics := []string{"telemetry-metrics", "run-results"}
	kClient, err := kgo.NewClient(
		kgo.SeedBrokers(kafkaBrokers...),
		kgo.ConsumeTopics(topics...),
		kgo.ConsumerGroup("platform-consumer-group-final-v9"),
		kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()), // ◄ CRUCIAL COUPLING LINE
	)
	if err != nil {
		return fmt.Errorf("failed to create Kafka engine worker: %w", err)
	}
	defer kClient.Close()

	log.Println("📥 Consumer service daemon started. Listening on topics: telemetry-metrics, run-results...")

	// 4. Consumption Loop
	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down consumer service daemon gracefully...")
			return nil
		default:
			fetches := kClient.PollFetches(ctx)
			if fetches.IsClientClosed() {
				log.Println("⚠️ Kafka connection client handles closed down.")
				return nil
			}

			if errs := fetches.Errors(); len(errs) > 0 {
				log.Printf("Kafka fetch warning block encountered: %v", errs)
			}

			fetches.EachRecord(func(record *kgo.Record) {
				log.Printf("[Kafka Message Received] Topic: %s | Partition: %d | Offset: %d", record.Topic, record.Partition, record.Offset)

				switch record.Topic {
				case "telemetry-metrics":
					var event MetricEvent
					if err := json.Unmarshal(record.Value, &event); err != nil {
						log.Printf("❌ Failed to unmarshal metric payload packet: %v | Raw payload: %s", err, string(record.Value))
						return
					}

					// A. Store raw timeseries metrics in PostgreSQL
					_, err = db.ExecContext(ctx,
						`INSERT INTO telemetry_timeseries (time, run_id, team_id, tps, p50_us, p90_us, p99_us) 
						 VALUES (TO_TIMESTAMP($1 / 1000000000.0), $2, $3, $4, $5, $6, $7)`,
						event.Timestamp, event.RunID, event.TeamID, event.TPS, event.P50Micro, event.P90Micro, event.P99Micro,
					)
					if err != nil {
						log.Printf("❌ Database Write Failure [telemetry_timeseries]: %v", err)
						return
					}

					// B. Broadcast live metrics instantly to Redis Pub/Sub for frontend WebSockets
					pubPayload, _ := json.Marshal(event)
					err = rdb.Publish(ctx, "channel:live-metrics", pubPayload).Err()
					if err != nil {
						log.Printf("⚠️ Redis Pub/Sub deployment dropped: %v", err)
					}

				case "run-results":
					var runEvent RunResultEvent
					if err := json.Unmarshal(record.Value, &runEvent); err != nil {
						log.Printf("❌ Failed to unmarshal final result bundle: %v | Raw payload: %s", err, string(record.Value))
						return
					}

					log.Printf("[Processing Run Finalization] Team: %s | Run ID: %s | Status: %s | Correctness: %s",
						runEvent.TeamID, runEvent.RunID, runEvent.Status, runEvent.Correctness)

					// A. Upsert final run results in PostgreSQL for historical auditing
					_, err = db.ExecContext(ctx,
						`INSERT INTO benchmark_runs (run_id, team_id, status, correctness, final_score, final_tps, p99_us, updated_at)
						 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW())
						 ON CONFLICT (run_id) DO UPDATE SET 
						   status = EXCLUDED.status,
						   correctness = EXCLUDED.correctness,
						   final_score = EXCLUDED.final_score,
						   final_tps = EXCLUDED.final_tps,
						   p99_us = EXCLUDED.p99_us,
						   updated_at = NOW()`,
						runEvent.RunID, runEvent.TeamID, runEvent.Status, runEvent.Correctness,
						runEvent.FinalScore, runEvent.FinalTPS, runEvent.P99Micro,
					)
					if err != nil {
						log.Printf("❌ Database Upsert Failure [benchmark_runs]: %v", err)
						return
					}
					log.Printf("💾 Successfully saved final stats to benchmark_runs for run %s", runEvent.RunID)

					// B. Update Redis Sorted Set Leaderboard (Updates if first submission or new personal best)
					if runEvent.Correctness == "PASS" && runEvent.Status == "COMPLETED" {
						existingScore, scoreErr := rdb.ZScore(ctx, "leaderboard:scores", runEvent.TeamID).Result()

						if scoreErr == redis.Nil || runEvent.FinalScore > existingScore {
							err = rdb.ZAdd(ctx, "leaderboard:scores", redis.Z{
								Score:  runEvent.FinalScore,
								Member: runEvent.TeamID,
							}).Err()
							if err != nil {
								log.Printf("❌ Redis Leaderboard Update Failure: %v", err)
								return
							}

							// Cache complete run stats in Redis for sub-millisecond leaderboard reads
							detailsBytes, err := json.Marshal(runEvent)
							if err == nil {
								_ = rdb.Set(ctx, "team:details:"+runEvent.TeamID, detailsBytes, 0).Err()
							}

							log.Printf("👑 [Leaderboard Updated: New Personal Best] Team: %s | Score: %.2f (Previous: %.2f)",
								runEvent.TeamID, runEvent.FinalScore, existingScore)
						} else {
							log.Printf("ℹ️ [Leaderboard Maintained] Team: %s | Current Run: %.2f <= Personal Best: %.2f",
								runEvent.TeamID, runEvent.FinalScore, existingScore)
						}
					} else {
						log.Printf("🚫 Run omitted from leaderboard rankings. Criteria status conditions not met.")
					}
				}
			})
		}
	}
}
