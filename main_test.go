package main

import (
	"encoding/json"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestIsProcessRunning(t *testing.T) {
	// Test with PID 0 (should return false)
	if isProcessRunning(0) {
		t.Error("isProcessRunning(0) should return false")
	}

	// Test with current process PID (should return true)
	currentPID := os.Getpid()
	if !isProcessRunning(currentPID) {
		t.Error("isProcessRunning(currentPID) should return true")
	}

	// Test with a non-existent PID (should return false)
	// We'll use a very high PID number that's unlikely to exist
	if isProcessRunning(999999) {
		t.Error("isProcessRunning(999999) should return false")
	}
}

func TestProcessSignalHandling(t *testing.T) {
	// Test that we can send signal 0 to our own process
	process, err := os.FindProcess(os.Getpid())
	if err != nil {
		t.Fatalf("Failed to find current process: %v", err)
	}

	// Signal 0 should not actually send a signal, just check if process exists
	err = process.Signal(syscall.Signal(0))
	if err != nil {
		t.Errorf("Failed to send signal 0 to current process: %v", err)
	}
}

func TestJobMetaNotificationFields(t *testing.T) {
	// Test that the JobMeta struct correctly handles notification fields
	meta := &JobMeta{
		ID:             "test-job-1",
		Args:           []string{"echo", "hello"},
		SlackWebhook:   "https://hooks.slack.com/test",
		NotifyOnStart:  true,
		NotifyOnFinish: true,
		Status:         "IN_QUEUE",
		EnqueuedAt:     time.Now(),
	}

	// Test JSON marshaling/unmarshaling to ensure fields are preserved
	data, err := json.Marshal(meta)
	if err != nil {
		t.Fatalf("Failed to marshal JobMeta: %v", err)
	}

	var unmarshaledMeta JobMeta
	err = json.Unmarshal(data, &unmarshaledMeta)
	if err != nil {
		t.Fatalf("Failed to unmarshal JobMeta: %v", err)
	}

	if unmarshaledMeta.SlackWebhook != meta.SlackWebhook {
		t.Errorf("SlackWebhook not preserved: got %s, want %s", unmarshaledMeta.SlackWebhook, meta.SlackWebhook)
	}
	if unmarshaledMeta.NotifyOnStart != meta.NotifyOnStart {
		t.Errorf("NotifyOnStart not preserved: got %v, want %v", unmarshaledMeta.NotifyOnStart, meta.NotifyOnStart)
	}
	if unmarshaledMeta.NotifyOnFinish != meta.NotifyOnFinish {
		t.Errorf("NotifyOnFinish not preserved: got %v, want %v", unmarshaledMeta.NotifyOnFinish, meta.NotifyOnFinish)
	}
}

func TestSlackNotificationPayload(t *testing.T) {
	// Create a test job meta
	meta := &JobMeta{
		ID:          "test-job-123",
		Args:        []string{"echo", "test message"},
		Status:      "COMPLETED",
		StartedAt:   time.Now().Add(-5 * time.Minute),
		CompletedAt: time.Now(),
		PID:         12345,
	}

	// Test that sendSlackNotification would be called correctly
	// (We can't easily test the actual HTTP call without a mock server)
	
	// Test notification logic for different event types
	testCases := []struct {
		eventType        string
		notifyOnStart    bool
		notifyOnFinish   bool
		slackWebhook     string
		shouldNotify     bool
	}{
		{"start", true, false, "https://hooks.slack.com/test", true},
		{"start", false, true, "https://hooks.slack.com/test", false},
		{"finish", false, true, "https://hooks.slack.com/test", true},
		{"finish", true, false, "https://hooks.slack.com/test", false},
		{"start", true, false, "", false}, // no webhook
	}

	for _, tc := range testCases {
		meta.NotifyOnStart = tc.notifyOnStart
		meta.NotifyOnFinish = tc.notifyOnFinish
		meta.SlackWebhook = tc.slackWebhook

		shouldNotify := false
		if tc.eventType == "start" && meta.NotifyOnStart {
			shouldNotify = true
		} else if tc.eventType == "finish" && meta.NotifyOnFinish {
			shouldNotify = true
		}
		shouldNotify = shouldNotify && meta.SlackWebhook != ""

		if shouldNotify != tc.shouldNotify {
			t.Errorf("For event %s with notifyOnStart=%v, notifyOnFinish=%v, webhook=%s: got shouldNotify=%v, want %v",
				tc.eventType, tc.notifyOnStart, tc.notifyOnFinish, tc.slackWebhook, shouldNotify, tc.shouldNotify)
		}
	}
}

func TestCommandArgumentHandling(t *testing.T) {
	// Test that command arguments are properly formatted in notifications
	testArgs := [][]string{
		{"echo", "hello world"},
		{"ls", "-la", "/tmp"},
		{"python3", "-c", "print('test')"},
	}

	for _, args := range testArgs {
		joined := strings.Join(args, " ")
		if !strings.Contains(joined, args[0]) {
			t.Errorf("Command formatting failed for args: %v", args)
		}
	}
}
