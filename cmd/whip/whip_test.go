package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindPlaybookPath(t *testing.T) {
	// Create a temporary directory structure
	tempDir, err := os.MkdirTemp("", "whip_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	// Resolve any symlinks in tempDir
	tempDir, err = filepath.EvalSymlinks(tempDir)
	if err != nil {
		t.Fatalf("Failed to resolve symlinks in temp dir: %v", err)
	}

	// Create a .whip directory with a playbook.yml file
	whipDir := filepath.Join(tempDir, "subdir", "subsubdir", ".whip")
	err = os.MkdirAll(whipDir, 0o755)
	if err != nil {
		t.Fatalf("Failed to create .whip dir: %v", err)
	}
	playbookPath := filepath.Join(whipDir, "playbook.yml")
	err = os.WriteFile(playbookPath, []byte("dummy content"), 0o644)
	if err != nil {
		t.Fatalf("Failed to create playbook.yml: %v", err)
	}

	// Change to the deepest subdirectory
	err = os.Chdir(filepath.Join(tempDir, "subdir", "subsubdir"))
	if err != nil {
		t.Fatalf("Failed to change directory: %v", err)
	}

	// Run the function
	result := findPlaybookPath()

	// Check the result
	expected := playbookPath
	if result != expected {
		t.Errorf("Expected path %s, but got %s", expected, result)
	}

	// Test when no playbook is found
	err = os.Chdir("/")
	if err != nil {
		t.Fatalf("Failed to change to root directory: %v", err)
	}

	result = findPlaybookPath()
	if result != "" {
		t.Errorf("Expected empty string when no playbook found, but got %s", result)
	}
}
