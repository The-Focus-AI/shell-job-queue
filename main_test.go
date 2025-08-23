package main

import (
	"os"
	"syscall"
	"testing"
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
