# 🛠 Process-Based Job Queue in Go

This is a simple, file-backed job queue server written in Go. Each job runs as a separate OS-level process. Useful for wrapping command-line tools like model runners, converters, or custom scripts.

## ✅ Features

- Jobs are executed as child processes
- Each job logs `stderr` and stores `stdout` as the final result
- Persistent metadata and logs saved to the filesystem
- Webhook support to notify external services on job completion
- REST API for job submission, status tracking, result fetching, and cancellation

---

## 🚀 Usage

### 1. Build and Run Locally

```bash
make build
./processjobqueue
```

Or run directly:

```bash
make run
```

### 2. Submit a Job

```bash
curl -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "args": ["echo", "Hello, world!"],
    "mime_type": "text/plain",
    "webhook": "https://webhook.site/your-id"
  }'
```

### 3. Check Status

```bash
curl http://localhost:8080/jobs/<job-id>/status
```

### 4. Get Result

```bash
curl http://localhost:8080/jobs/<job-id>/result
```

### 5. Cancel a Job

```bash
curl -X PUT http://localhost:8080/jobs/<job-id>/cancel
```

---

## 🐳 Docker

### Build

```bash
make docker-build
```

### Run

```bash
make docker-run
```

Mounts the `jobs/` folder for persistent storage and serves the API on port `8080`.

---

## 📁 Job Directory Structure

Each job is stored in:

```
jobs/<job-id>/
├── meta.json      ← job status + metadata
├── stdout.txt     ← final output
└── stderr.txt     ← logs (live updates)
```

---

## 🧩 Requirements

- Go 1.21+
- (Optional) Docker for containerized deployment

---

## 📬 TODOs

- Retry + sign webhooks
- Job priorities or delayed execution
- Streaming logs over SSE
