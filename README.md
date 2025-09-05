# 🛠 Process-Based Job Queue in Go

This is a simple, file-backed job queue server written in Go. Each job runs as a separate OS-level process. Useful for wrapping command-line tools like model runners, converters, or custom scripts.

## ✅ Features

- Jobs are executed as child processes
- Each job logs `stderr` and stores `stdout` as the final result
- Persistent metadata and logs saved to the filesystem
- **Configurable notifications** with support for:
  - Traditional webhooks on job completion
  - Slack webhooks with rich formatting for job start and/or completion events
- REST API for job submission, status tracking, result fetching, and cancellation
- **Static file serving** from `public/` directory at the root path

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

**Environment Variables:**
- `PUBLIC_DIR`: Set the public directory path (e.g., `/var/www/public`)
- `JOBS_DIR`: Set the jobs storage directory (e.g., `/data/jobs`)
- `BASE_URL`: Set the base URL for job URLs (e.g., `https://example.com`)
- `DEBUG`: Set to `1` to enable debug logging

### 2. Static File Serving

The server automatically serves static files from the `public/` directory at the root path:

- Place any files (HTML, CSS, JS, images, etc.) in the `public/` directory
- They will be accessible at `http://localhost:8080/`
- If `public/index.html` exists, it will be served at the root path
- If no `index.html` exists, a default status page will be shown

**Directory Resolution Priority:**
1. `PUBLIC_DIR` environment variable (if set)
2. `public/` directory relative to the executable location
3. `public/` directory relative to current working directory

**Usage Examples:**

```bash
# Default behavior - serves from ./public/
go run main.go

# Custom public directory
PUBLIC_DIR=/var/www/public go run main.go

# With debug logging
DEBUG=1 go run main.go
```

Example file structure:
```
public/
├── index.html          → http://localhost:8080/
├── style.css           → http://localhost:8080/style.css
├── script.js           → http://localhost:8080/script.js
└── images/logo.png     → http://localhost:8080/images/logo.png
```

**Server Output:**
When you start the server, it will show which directory it's serving from:
```
Server running on :8080
Serving static files from: /path/to/public
```

### 3. Submit a Job

**Basic job submission:**
```bash
curl -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "args": ["echo", "Hello, world!"],
    "mime_type": "text/plain",
    "webhook": "https://webhook.site/your-id"
  }'
```

**With Slack notifications:**
```bash
curl -X POST http://localhost:8080/jobs \
  -H 'Content-Type: application/json' \
  -d '{
    "args": ["python", "my_script.py"],
    "mime_type": "text/plain",
    "slack_webhook": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
    "notify_on_start": true,
    "notify_on_finish": true
  }'
```

**Job submission parameters:**
- `args` (required): Array of command and arguments to execute
- `mime_type` (optional): MIME type for the result content
- `webhook` (optional): URL for traditional webhook notifications (triggered on job completion)
- `slack_webhook` (optional): Slack webhook URL for rich notifications
- `notify_on_start` (optional): Send Slack notification when job starts (default: false)
- `notify_on_finish` (optional): Send Slack notification when job completes (default: false)

### 4. Check Status

```bash
curl http://localhost:8080/jobs/<job-id>/status
```

### 5. Get Result

```bash
curl http://localhost:8080/jobs/<job-id>/result
```

### 6. Cancel a Job

```bash
curl -X PUT http://localhost:8080/jobs/<job-id>/cancel
```

---

## 📋 API Endpoints

| Method | Endpoint | Description |
|--------|----------|-------------|
| `POST` | `/jobs` | Submit a new job |
| `GET` | `/jobs` | List all jobs |
| `GET` | `/jobs/{id}/status` | Get job status |
| `GET` | `/jobs/{id}/result` | Get job result |
| `GET` | `/jobs/{id}/log` | Get job log |
| `PUT` | `/jobs/{id}/cancel` | Cancel a running job |

**Job Status Values:**
- `IN_QUEUE`: Job is waiting to be executed
- `IN_PROGRESS`: Job is currently running
- `COMPLETED`: Job finished successfully
- `FAILED`: Job failed with an error
- `CANCELED`: Job was canceled by user

**Example Job Response:**
```json
{
  "id": "550e8400-e29b-41d4-a716-446655440000",
  "args": ["echo", "Hello, world!"],
  "mime_type": "text/plain",
  "webhook": "https://webhook.site/your-id",
  "slack_webhook": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
  "notify_on_start": true,
  "notify_on_finish": true,
  "status": "IN_QUEUE",
  "enqueued_at": "2024-01-01T12:00:00Z",
  "status_url": "/jobs/550e8400-e29b-41d4-a716-446655440000/status",
  "result_url": "/jobs/550e8400-e29b-41d4-a716-446655440000/result",
  "log_url": "/jobs/550e8400-e29b-41d4-a716-446655440000/log"
}
```

---

## 🔔 Notifications

The job queue supports two types of webhook notifications:

### Traditional Webhooks
- Triggered only when jobs complete (success, failure, or cancellation)
- Simple JSON payload with job metadata
- Backward compatible with existing integrations

**Payload format:**
```json
{
  "id": "job-id",
  "status": "COMPLETED|FAILED|CANCELED",
  "result_url": "/jobs/job-id/result",
  "log_url": "/jobs/job-id/log"
}
```

### Slack Webhooks
- Rich formatting with job details and status colors
- Configurable for job start and/or completion events
- Includes clickable links to logs and results

**Slack notification features:**
- **Start notifications**: Alert when a job begins execution
- **Completion notifications**: Alert when a job finishes with status-specific formatting
- **Rich formatting**: Command details, timing information, and direct links
- **Status colors**: Green for success, red for failure, blue for cancellation

**Example Slack notification setup:**
```json
{
  "args": ["python", "data_processor.py"],
  "slack_webhook": "https://hooks.slack.com/services/YOUR/SLACK/WEBHOOK",
  "notify_on_start": true,
  "notify_on_finish": true
}
```

Both notification types can be used simultaneously for the same job.

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

**Environment Variables:**
- `PUBLIC_DIR`: Set the public directory path (e.g., `/app/public`)
- `JOBS_DIR`: Set the jobs storage directory (e.g., `/data/jobs`)
- `BASE_URL`: Set the base URL for job URLs (e.g., `https://example.com`)

**Example with custom public directory:**
```bash
docker run -p 8080:8080 \
  -v $(pwd)/jobs:/app/jobs \
  -v $(pwd)/public:/app/public \
  -e PUBLIC_DIR=/app/public \
  shell-job-queue
```

---

## 📁 Directory Structure

```
shell-job-queue/
├── public/             ← Static files served at root
│   ├── index.html      ← Default homepage
│   └── style.css       ← Additional static assets
├── jobs/               ← Job storage (created automatically)
│   └── <job-id>/
│       ├── meta.json   ← job status + metadata
│       ├── stdout.txt  ← final output
│       └── stderr.txt  ← logs (live updates)
└── main.go             ← Server code
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