Project Brief: Process-Based Job Queue System in Go

Overview

This project is a simple, persistent job queue server built in Go. It manages background jobs by executing them as separate OS-level processes. The system is file-backed—each job is stored in its own directory on disk, including logs, metadata, and output.

The purpose is to reliably run commands (e.g. model execution, media processing) in isolated subprocesses with:
	•	Job status tracking
	•	Streaming logs
	•	Persistent results
	•	Webhook callbacks
	•	RESTful API

Functional Requirements

1. Job Submission (POST /jobs)
	•	Accepts a JSON payload:

{
  "args": ["command", "--flag=value"],
  "mime_type": "text/plain",       // optional
  "webhook": "https://example.com" // optional
}


	•	Generates a unique job ID
	•	Creates a job directory under jobs/<job-id>/
	•	Persists metadata in meta.json
	•	Enqueues the job for execution

2. Job Execution
	•	Runs each job as a subprocess using the provided args
	•	Captures stdout into stdout.txt as the final result
	•	Streams stderr into stderr.txt for live logs
	•	Saves PID and timestamps in meta.json
	•	Supports cancellation via process kill

3. Job Lifecycle

Each job transitions through:
	•	IN_QUEUE
	•	IN_PROGRESS
	•	COMPLETED | FAILED | CANCELED

4. Job Status (GET /jobs/<id>/status)
	•	Returns the current metadata of the job (meta.json)

5. Job Result (GET /jobs/<id>/result)
	•	Returns the contents of stdout.txt (if COMPLETED)

6. Job Cancellation (PUT /jobs/<id>/cancel)
	•	Cancels a running job via context.CancelFunc
	•	Terminates the child process

7. Webhooks
	•	If a webhook URL is included in submission, a POST request is sent when the job completes with:

{
  "id": "job-id",
  "status": "COMPLETED",
  "result_url": "/jobs/job-id/result"
}


	•	Webhook is fired asynchronously

Technical Details

Filesystem Layout

jobs/
  <job-id>/
    meta.json       // job metadata
    stdout.txt      // final output
    stderr.txt      // log output

JobMeta Structure

type JobMeta struct {
  ID          string
  Args        []string
  MimeType    string
  Webhook     string
  Status      string
  PID         int
  EnqueuedAt  time.Time
  StartedAt   time.Time
  CompletedAt time.Time
}

Concurrency
	•	A central queue chan *JobMeta feeds a workerLoop()
	•	Each job spawns a goroutine with a context
	•	A runningJobs map tracks live processes

Goals
	•	Durable job queue using only local filesystem
	•	Easy to understand and deploy
	•	Suitable for batch inference or utility jobs

Future Enhancements
	•	Add retries or backoff for webhooks
	•	Stream stderr.txt live over SSE
	•	Support multiple concurrent workers
	•	Add job expiration / cleanup TTL
	•	Web UI or CLI client for easier usage