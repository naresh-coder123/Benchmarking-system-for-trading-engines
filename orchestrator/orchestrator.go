package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"sync"
	"time"

	"grpc/loadgen"
	runner "grpc/sandbox-runner"
)

// RunState represents the current phase of a benchmark submission
type RunState string

const (
	StatePending       RunState = "PENDING"
	StateSpawning      RunState = "SPAWNING_SANDBOX"
	StateValidating    RunState = "VALIDATING"
	StateStressTesting RunState = "STRESS_TESTING"
	StateCompleted     RunState = "COMPLETED"
	StateFailed        RunState = "FAILED"
)

var allocatedPort int = 9000

// BenchmarkRun holds the tracking information for a submission
type BenchmarkRun struct {
	RunID      string    `json:"run_id"`
	TeamID     string    `json:"team_id"`
	Image      string    `json:"image"`
	ExposePort int       `json:"expose_port"`
	State      RunState  `json:"state"`
	ErrorMsg   string    `json:"error_msg,omitempty"`
	CreatedAt  time.Time `json:"created_at"`
}

type Submission struct {
	TeamID   string `json:"team_id"`
	Image    string `json:"image"`
	SubmitId string `json:"submit_id"`
}

// Orchestrator manages runs in memory (can be backed by Redis/PostgreSQL later)
type Orchestrator struct {
	mu            sync.Mutex
	runs          map[string]*BenchmarkRun
	nextAllocPort int
}

func NewOrchestrator() *Orchestrator {
	return &Orchestrator{
		runs:          make(map[string]*BenchmarkRun),
		nextAllocPort: 9000,
	}
}

func (o *Orchestrator) allocatePort() int {
	o.mu.Lock()
	defer o.mu.Unlock()
	o.nextAllocPort++
	return o.nextAllocPort
}

// HandleCreateRun triggers a new benchmark workflow asynchronously
func (o *Orchestrator) HandleCreateRun(req *Submission) {

	runID := req.SubmitId
	assignedPort := o.allocatePort()
	run := &BenchmarkRun{
		RunID:      req.SubmitId,
		TeamID:     req.TeamID,
		Image:      req.Image,
		ExposePort: assignedPort,
		State:      StatePending,
		CreatedAt:  time.Now(),
	}

	o.mu.Lock()
	o.runs[runID] = run
	o.mu.Unlock()

	// Launch workflow asynchronously
	go o.executeWorkflow(runID)
}

// HandleGetStatus checks the status of an ongoing or completed run
func (o *Orchestrator) HandleGetStatus(w http.ResponseWriter, r *http.Request) {
	runID := r.URL.Path[len("/api/status/"):]

	o.mu.Lock()
	run, exists := o.runs[runID]
	o.mu.Unlock()

	if !exists {
		http.Error(w, "Run not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(run)
}

// executeWorkflow runs the sequential pipeline steps
func (o *Orchestrator) executeWorkflow(runID string) {
	updateState := func(state RunState, errStr string) {
		o.mu.Lock()
		defer o.mu.Unlock()
		if r, ok := o.runs[runID]; ok {
			r.State = state
			r.ErrorMsg = errStr
		}
	}

	o.mu.Lock()
	runInfo := o.runs[runID]
	o.mu.Unlock()

	if runInfo == nil {
		log.Printf("[%s] Error: run info not found in memory", runID)
		return
	}

	// Step 1: Spawning Sandbox Container
	updateState(StateSpawning, "")
	log.Printf("[%s] Spawning container for image %s on port %d...", runID, runInfo.Image, runInfo.ExposePort)

	sr, err := runner.NewSandboxRunner()
	if err != nil {
		updateState(StateFailed, err.Error())
		log.Printf("[%s] Failed to create sandbox runner: %v", runID, err)
		return
	}
	defer sr.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	rd := &runner.RunDetails{
		RunID:      runID,
		TeamID:     runInfo.TeamID,
		ExposePort: runInfo.ExposePort,
		ImageURL:   runInfo.Image,
	}

	// Start container (pulls image if not present locally)
	containerID, err := sr.StartSandbox(ctx, rd)
	if err != nil {
		updateState(StateFailed, err.Error())
		log.Printf("[%s] Failed to start sandbox container: %v", runID, err)
		return
	}

	// Ensure container cleanup on exit
	defer func() {
		cleanupCtx, cleanupCancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cleanupCancel()
		if err := sr.StopSandbox(cleanupCtx, containerID); err != nil {
			log.Printf("[%s] Container cleanup failed: %v", runID, err)
		}
	}()

	// Wait until the containerized engine TCP endpoint is ready
	log.Printf("[%s] Waiting for container readiness probe on port %d...", runID, runInfo.ExposePort)
	if err := sr.WaitForReady(ctx, "127.0.0.1", runInfo.ExposePort, 15*time.Second); err != nil {
		updateState(StateFailed, err.Error())
		log.Printf("[%s] Readiness probe timed out: %v", runInfo.RunID, err)
		return
	}

	// Step 2: Stress Testing with Load Generator (Bot Fleet)
	updateState(StateStressTesting, "")
	log.Printf("[%s] Launching load-generator bot fleet (2000 TPS)...", runID)

	cfg := loadgen.Config{
		TargetURL:    fmt.Sprintf("ws://127.0.0.1:%d/ws", runInfo.ExposePort),
		IngesterAddr: "localhost:50051",
		TeamID:       runInfo.TeamID,
		RunID:        runID,
		NumWorkers:   10,
		OrdersPerSec: 2000,
		Duration:     15 * time.Second,
	}

	if err := loadgen.RunBenchmark(cfg); err != nil {
		updateState(StateFailed, err.Error())
		log.Printf("[%s] Load test execution failed: %v", runID, err)
		return
	}

	// Step 3: Completed
	updateState(StateCompleted, "")
	log.Printf("[%s] Benchmark run successfully completed!", runID)
}
