package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"time"

	"grpc/consumer"
	"grpc/ingester"
	"grpc/orchestrator"
)

type details struct {
	TeamId   string `json:"team_id"`
	Image    string `json:"image"`
	SubmitId string `json:"submit_id"` // Added missing field
}

// Define an API server struct to share the orchestrator instance
type apiServer struct {
	orch *orchestrator.Orchestrator
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	// Instantiate the orchestrator once here so it persists state across requests
	server := &apiServer{
		orch: orchestrator.NewOrchestrator(),
	}

	mux := http.NewServeMux()
	go func() {
		brokers := []string{"localhost:9092"}
		if err := ingester.StartIngesterServer(":50051", brokers); err != nil {
			log.Fatalf("Ingester service crashed: %v", err)
		}
	}()

	// Give the ingester a moment to bind to the port
	time.Sleep(1 * time.Second)
	go func() {
		brokers := []string{"localhost:9092"}
		pgConnStr := "postgres://postgres:postgres@localhost:5432/benchmark_db?sslmode=disable"
		redisAddr := "localhost:6379"

		if err := consumer.StartConsumerService(ctx, brokers, pgConnStr, redisAddr); err != nil {
			log.Fatalf("Consumer service crashed: %v", err)
		}
	}()

	// Wrap handlers to have access to the server struct
	mux.HandleFunc("POST /api/submit", server.submit)
	mux.HandleFunc("GET /api/status/{id}", server.HandleGetStatus)

	log.Println("API listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}

func (s *apiServer) submit(w http.ResponseWriter, r *http.Request) {
	// 1. Decode the incoming request JSON into a temporary payload variable
	var payload details
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// 2. Build the pointer struct for your orchestrator without variable shadowing
	req := &orchestrator.Submission{
		TeamID:   payload.TeamId,
		Image:    payload.Image,
		SubmitId: payload.SubmitId,
	}

	// 3. Pass it to the shared orchestrator instance
	s.orch.HandleCreateRun(req)

	// 4. Respond back to the client so the request doesn't hang
	w.WriteHeader(http.StatusAccepted)
	w.Write([]byte(`{"status": "run created"}`))
}

func (s *apiServer) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	// Use Go 1.22+ path values to extract the {id} parameter from the URL
	id := r.PathValue("id")
	if id == "" {
		http.Error(w, "Missing ID parameter", http.StatusBadRequest)
		return
	}

	// Example response (You can query s.orch here to fetch the real status)
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"id":     id,
		"status": "pending",
	})
}
