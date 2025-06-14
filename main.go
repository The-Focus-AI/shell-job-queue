package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

type JobMeta struct {
	ID          string    `json:"id"`
	Args        []string  `json:"args"`
	MimeType    string    `json:"mime_type,omitempty"`
	Webhook     string    `json:"webhook,omitempty"`
	Status      string    `json:"status"`
	PID         int       `json:"pid,omitempty"`
	EnqueuedAt  time.Time `json:"enqueued_at"`
	StartedAt   time.Time `json:"started_at,omitempty"`
	CompletedAt time.Time `json:"completed_at,omitempty"`
}

var (
	runningJobs = make(map[string]*RunningJob)
	queue       = make(chan *JobMeta, 100)
	mu          sync.Mutex
)

type RunningJob struct {
	Cmd    *exec.Cmd
	Meta   *JobMeta
	Cancel context.CancelFunc
}

func main() {
	http.HandleFunc("/jobs", submitJob)
	http.HandleFunc("/jobs/", jobHandler)
	go workerLoop()
	fmt.Println("Server running on :8080")
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}

func submitJob(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Args     []string `json:"args"`
		MimeType string   `json:"mime_type,omitempty"`
		Webhook  string   `json:"webhook,omitempty"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}
	id := uuid.NewString()
	jobDir := filepath.Join("jobs", id)
	os.MkdirAll(jobDir, 0755)

	meta := &JobMeta{
		ID:         id,
		Args:       req.Args,
		MimeType:   req.MimeType,
		Webhook:    req.Webhook,
		Status:     "IN_QUEUE",
		EnqueuedAt: time.Now(),
	}
	saveMeta(meta)
	queue <- meta
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":         id,
		"status_url": "/jobs/" + id + "/status",
		"result_url": "/jobs/" + id + "/result",
	})
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	id := filepath.Base(r.URL.Path)
	switch {
	case r.URL.Path == "/jobs/"+id+"/status":
		meta, err := loadMeta(id)
		if err != nil {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(meta)
	case r.URL.Path == "/jobs/"+id+"/result":
		meta, err := loadMeta(id)
		if err != nil || meta.Status != "COMPLETED" {
			http.Error(w, "Result not available", http.StatusNotFound)
			return
		}
		path := filepath.Join("jobs", id, "stdout.txt")
		http.ServeFile(w, r, path)
	case r.Method == http.MethodPut && r.URL.Path == "/jobs/"+id+"/cancel":
		mu.Lock()
		if job, ok := runningJobs[id]; ok {
			job.Cancel()
		}
		mu.Unlock()
		w.WriteHeader(http.StatusOK)
	default:
		http.NotFound(w, r)
	}
}

func workerLoop() {
	for meta := range queue {
		go runJob(meta)
	}
}

func runJob(meta *JobMeta) {
	jobDir := filepath.Join("jobs", meta.ID)
	stdoutPath := filepath.Join(jobDir, "stdout.txt")
	stderrPath := filepath.Join(jobDir, "stderr.txt")
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, meta.Args[0], meta.Args[1:]...)
	stdoutFile, _ := os.Create(stdoutPath)
	stderrFile, _ := os.Create(stderrPath)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	if err := cmd.Start(); err != nil {
		meta.Status = "FAILED"
		saveMeta(meta)
		return
	}
	meta.PID = cmd.Process.Pid
	meta.Status = "IN_PROGRESS"
	meta.StartedAt = time.Now()
	saveMeta(meta)

	mu.Lock()
	runningJobs[meta.ID] = &RunningJob{Cmd: cmd, Meta: meta, Cancel: cancel}
	mu.Unlock()

	err := cmd.Wait()
	meta.CompletedAt = time.Now()

	mu.Lock()
	delete(runningJobs, meta.ID)
	mu.Unlock()

	stdoutFile.Close()
	stderrFile.Close()

	if ctx.Err() == context.Canceled {
		meta.Status = "CANCELED"
	} else if err != nil {
		meta.Status = "FAILED"
	} else {
		meta.Status = "COMPLETED"
	}
	saveMeta(meta)

	if meta.Webhook != "" {
		go sendWebhook(meta)
	}
}

func saveMeta(meta *JobMeta) {
	path := filepath.Join("jobs", meta.ID, "meta.json")
	data, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(path, data, 0644)
}

func loadMeta(id string) (*JobMeta, error) {
	path := filepath.Join("jobs", id, "meta.json")
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var meta JobMeta
	json.Unmarshal(data, &meta)
	return &meta, nil
}

func sendWebhook(meta *JobMeta) {
	payload := map[string]string{
		"id":         meta.ID,
		"status":     meta.Status,
		"result_url": "/jobs/" + meta.ID + "/result",
	}
	data, _ := json.Marshal(payload)
	http.Post(meta.Webhook, "application/json", bytes.NewReader(data))
}