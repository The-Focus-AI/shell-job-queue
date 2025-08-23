# Process Tracking and Orphaned Job Detection

## Overview

The shell job queue now includes comprehensive process tracking and orphaned job detection to ensure that jobs that have been orphaned (their processes are no longer running but they're still marked as "IN_PROGRESS") are properly identified and marked as "ORPHANED".

## Features

### 1. Process ID Tracking
- Each job stores its Process ID (PID) in the metadata when it starts
- The PID is used to track whether the process is still running

### 2. Automatic Orphaned Job Detection
- A background goroutine runs every minute to check for orphaned processes
- Jobs that are marked as "IN_PROGRESS" but whose processes are no longer running are marked as "ORPHANED"
- The cleanup process also sets a `CompletedAt` timestamp for orphaned jobs

### 3. Manual Cleanup
- The `/status` endpoint supports a POST request to trigger manual cleanup
- The `/jobs` endpoint automatically checks for orphaned jobs before listing

### 4. Process Status Checking
- Uses `syscall.Signal(0)` to check if a process is still running
- This is a non-intrusive way to check process status without actually sending a signal

## API Endpoints

### GET /status
Returns server status including:
- Current number of running jobs
- Server uptime
- Jobs directory path

### POST /status
Triggers manual cleanup of orphaned jobs. Returns "Cleanup completed" when done.

## Job Status Values

The system now supports these job statuses:
- `IN_QUEUE` - Job is waiting to be processed
- `IN_PROGRESS` - Job is currently running
- `COMPLETED` - Job completed successfully
- `FAILED` - Job failed during execution
- `CANCELED` - Job was canceled by user
- `ORPHANED` - Job was running but process is no longer active

## Implementation Details

### Process Checking Function
```go
func isProcessRunning(pid int) bool {
    if pid == 0 {
        return false
    }
    process, err := os.FindProcess(pid)
    if err != nil {
        return false
    }
    err = process.Signal(syscall.Signal(0))
    if err == nil {
        return true
    }
    if err.Error() == "os: process already finished" {
        return false
    }
    return false
}
```

### Background Cleanup
The `cleanupOrphanedJobs()` function runs every minute and:
1. Iterates through all running jobs
2. Checks if each job's process is still running
3. Marks jobs as "ORPHANED" if their processes are no longer active
4. Updates the job metadata and removes from running jobs map

### Filesystem Cleanup
The `checkForOrphanedJobs()` function:
1. Scans the jobs directory for all job metadata
2. Identifies jobs marked as "IN_PROGRESS" that aren't in the running jobs map
3. Checks if their processes are still running
4. Marks orphaned jobs as "ORPHANED"

## Usage Examples

### Check server status
```bash
curl http://localhost:8080/status
```

### Trigger manual cleanup
```bash
curl -X POST http://localhost:8080/status
```

### List all jobs (automatically checks for orphaned jobs)
```bash
curl http://localhost:8080/jobs
```

## Benefits

1. **Reliability**: Ensures job status accurately reflects process state
2. **Resource Management**: Prevents accumulation of stale job entries
3. **Debugging**: Helps identify when jobs have been unexpectedly terminated
4. **Monitoring**: Provides clear status for all job states including orphaned ones
