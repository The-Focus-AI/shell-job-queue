package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
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

type queuedJob struct {
	meta          *JobMeta
	inputFilePath string
}

var (
	runningJobs = make(map[string]*RunningJob)
	queue       = make(chan *queuedJob, 100)
	mu          sync.Mutex
)

type RunningJob struct {
	Cmd    *exec.Cmd
	Meta   *JobMeta
	Cancel context.CancelFunc
}

func main() {
	var fixedArgs []string
	if len(os.Args) > 1 {
		fixedArgs = os.Args[1:]
		fmt.Fprintf(os.Stderr, "Server running on :8080 (fixed command: %v)\n", fixedArgs)
	} else {
		fmt.Fprintln(os.Stderr, "Server running on :8080")
	}

	http.HandleFunc("/jobs", func(w http.ResponseWriter, r *http.Request) {
		jobsHandler(w, r, fixedArgs)
	})
	http.HandleFunc("/jobs/", func(w http.ResponseWriter, r *http.Request) {
		jobsHandler(w, r, fixedArgs)
	})
	go workerLoop()
	err := http.ListenAndServe(":8080", nil)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start server: %v\n", err)
		os.Exit(1)
	}
}

func jobsHandler(w http.ResponseWriter, r *http.Request, fixedArgs []string) {
	fmt.Fprintf(os.Stderr, "[DEBUG] jobsHandler: method=%s path=%s\n", r.Method, r.URL.Path)
	if r.URL.Path == "/jobs" && r.Method == http.MethodPost {
		submitJob(w, r, fixedArgs)
		return
	}
	if r.URL.Path == "/jobs" && r.Method == http.MethodGet {
		listJobs(w, r)
		return
	}
	if strings.HasPrefix(r.URL.Path, "/jobs/") {
		jobHandler(w, r)
		return
	}
	http.NotFound(w, r)
}

func submitJob(w http.ResponseWriter, r *http.Request, fixedArgs []string) {
	var req struct {
		Args     []string `json:"args"`
		MimeType string   `json:"mime_type,omitempty"`
		Webhook  string   `json:"webhook,omitempty"`
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(&req); err != nil {
		http.Error(w, "Invalid JSON", http.StatusBadRequest)
		return
	}

	id := uuid.NewString()
	jobDir := filepath.Join(getJobsDir(), id)
	os.MkdirAll(jobDir, 0755)

	// Save any remaining body as input file
	inputFilePath := ""
	remaining, _ := io.ReadAll(r.Body)
	if len(remaining) > 0 {
		inputFilePath = filepath.Join(os.TempDir(), "input-"+id+".tmp")
		f, err := os.Create(inputFilePath)
		if err == nil {
			_, _ = f.Write(remaining)
			f.Close()
		}
	}

	args := req.Args
	if len(fixedArgs) > 0 {
		args = append(append([]string{}, fixedArgs...), req.Args...)
	}

	meta := &JobMeta{
		ID:         id,
		Args:       args,
		MimeType:   req.MimeType,
		Webhook:    req.Webhook,
		Status:     "IN_QUEUE",
		EnqueuedAt: time.Now(),
	}
	saveMeta(meta)
	queue <- &queuedJob{meta: meta, inputFilePath: inputFilePath}

	baseURL := os.Getenv("BASE_URL")
	statusPath := "/jobs/" + id + "/status"
	resultPath := "/jobs/" + id + "/result"
	logPath := "/jobs/" + id + "/log"
	if baseURL != "" {
		statusPath = baseURL + statusPath
		resultPath = baseURL + resultPath
		logPath = baseURL + logPath
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"id":         id,
		"status_url": statusPath,
		"result_url": resultPath,
		"log_url":    logPath,
	})
}

func jobHandler(w http.ResponseWriter, r *http.Request) {
	// Extract job ID and subpath
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/jobs/"), "/")
	if len(parts) < 2 {
		http.NotFound(w, r)
		return
	}
	id := parts[0]
	endpoint := parts[1]

	switch endpoint {
	case "status":
		meta, err := loadMeta(id)
		if err != nil {
			http.Error(w, "Job not found", http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(meta)
	case "result":
		meta, err := loadMeta(id)
		if err != nil || meta.Status != "COMPLETED" {
			http.Error(w, "Result not available", http.StatusNotFound)
			return
		}
		path := filepath.Join(getJobsDir(), id, "stdout.txt")
		http.ServeFile(w, r, path)
	case "log":
		path := filepath.Join(getJobsDir(), id, "stderr.txt")
		if _, err := os.Stat(path); err != nil {
			http.Error(w, "Log not available", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, path)
	case "cancel":
		if r.Method != http.MethodPut {
			http.NotFound(w, r)
			return
		}
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
	for qj := range queue {
		go runJob(qj.meta, qj.inputFilePath)
	}
}

func runJob(meta *JobMeta, inputFilePath string) {
	jobDir := filepath.Join(getJobsDir(), meta.ID)
	stdoutPath := filepath.Join(jobDir, "stdout.txt")
	stderrPath := filepath.Join(jobDir, "stderr.txt")
	ctx, cancel := context.WithCancel(context.Background())

	cmd := exec.CommandContext(ctx, meta.Args[0], meta.Args[1:]...)
	stdoutFile, _ := os.Create(stdoutPath)
	stderrFile, _ := os.Create(stderrPath)
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile

	// If input file exists, use it as stdin
	if inputFilePath != "" {
		inFile, err := os.Open(inputFilePath)
		if err == nil {
			cmd.Stdin = inFile
			defer inFile.Close()
		}
	}

	// Log the command if DEBUG=1
	if os.Getenv("DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] Running command: %v\n", cmd.Args)
	}

	if err := cmd.Start(); err != nil {
		meta.Status = "FAILED"
		meta.StartedAt = time.Now()
		meta.CompletedAt = meta.StartedAt
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

	// Remove input file after job completes
	if inputFilePath != "" {
		os.Remove(inputFilePath)
	}

	if ctx.Err() == context.Canceled {
		meta.Status = "CANCELED"
	} else if err != nil {
		meta.Status = "FAILED"
	} else {
		meta.Status = "COMPLETED"
	}
	saveMeta(meta)

	if os.Getenv("DEBUG") == "1" {
		fmt.Fprintf(os.Stderr, "[DEBUG] Job finished: id=%s status=%s\n", meta.ID, meta.Status)
	}

	if meta.Webhook != "" {
		if os.Getenv("DEBUG") == "1" {
			fmt.Fprintf(os.Stderr, "[DEBUG] Triggering webhook: url=%s id=%s status=%s\n", meta.Webhook, meta.ID, meta.Status)
		}
		go sendWebhook(meta)
	}
}

func saveMeta(meta *JobMeta) {
	path := filepath.Join(getJobsDir(), meta.ID, "meta.json")
	data, _ := json.MarshalIndent(meta, "", "  ")
	os.WriteFile(path, data, 0644)
}

func loadMeta(id string) (*JobMeta, error) {
	path := filepath.Join(getJobsDir(), id, "meta.json")
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

func listJobs(w http.ResponseWriter, r *http.Request) {
	entries, err := os.ReadDir(getJobsDir())
	if err != nil {
		http.Error(w, "Failed to read jobs directory", http.StatusInternalServerError)
		return
	}
	var jobs []struct {
		ID         string   `json:"id"`
		Args       []string `json:"args"`
		Status     string   `json:"status"`
		ResultURL  string   `json:"result_url"`
		LogURL     string   `json:"log_url"`
		EnqueuedAt string   `json:"enqueued_at"`
	}
	baseURL := os.Getenv("BASE_URL")
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		metaPath := filepath.Join(getJobsDir(), entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			continue
		}
		var meta JobMeta
		if err := json.Unmarshal(data, &meta); err != nil {
			continue
		}
		resultPath := "/jobs/" + meta.ID + "/result"
		logPath := "/jobs/" + meta.ID + "/log"
		if baseURL != "" {
			resultPath = baseURL + resultPath
			logPath = baseURL + logPath
		}
		jobs = append(jobs, struct {
			ID         string   `json:"id"`
			Args       []string `json:"args"`
			Status     string   `json:"status"`
			ResultURL  string   `json:"result_url"`
			LogURL     string   `json:"log_url"`
			EnqueuedAt string   `json:"enqueued_at"`
		}{
			ID:         meta.ID,
			Args:       meta.Args,
			Status:     meta.Status,
			ResultURL:  resultPath,
			LogURL:     logPath,
			EnqueuedAt: meta.EnqueuedAt.Format(time.RFC3339),
		})
	}
	// Sort jobs by EnqueuedAt descending
	sort.Slice(jobs, func(i, j int) bool {
		return jobs[i].EnqueuedAt > jobs[j].EnqueuedAt
	})
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(jobs)
}

func getJobsDir() string {
	dir := os.Getenv("JOBS_DIR")
	if dir == "" {
		dir = "jobs"
	}
	return dir
}
