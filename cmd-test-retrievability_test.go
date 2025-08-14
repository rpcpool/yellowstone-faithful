package main

import (
	"os"
	"testing"
	"time"
)

func TestReadCIDsFromInput(t *testing.T) {
	tests := []struct {
		name          string
		input         string
		expectedCIDs  []string
		expectedError bool
	}{
		{
			name: "valid CIDs",
			input: `QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG`,
			expectedCIDs: []string{
				"QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
				"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			},
			expectedError: false,
		},
		{
			name: "CIDs with comments and empty lines",
			input: `# This is a comment
QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o

# Another comment
QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG
`,
			expectedCIDs: []string{
				"QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
				"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			},
			expectedError: false,
		},
		{
			name: "invalid CID mixed with valid",
			input: `QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o
invalid-cid
QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG`,
			expectedCIDs: []string{
				"QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
				"QmYwAPJzv5CZsnA625s3Xf2nemtYgPpHdWEz79ojWnPbdG",
			},
			expectedError: false,
		},
		{
			name:          "empty input",
			input:         "",
			expectedCIDs:  []string{},
			expectedError: false,
		},
		{
			name: "only comments",
			input: `# Comment 1
# Comment 2`,
			expectedCIDs:  []string{},
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create temporary file
			tmpFile, err := os.CreateTemp("", "test-cids-*.txt")
			if err != nil {
				t.Fatalf("Failed to create temp file: %v", err)
			}
			defer os.Remove(tmpFile.Name())

			// Write test input
			if _, err := tmpFile.WriteString(tt.input); err != nil {
				t.Fatalf("Failed to write to temp file: %v", err)
			}
			tmpFile.Close()

			// Test the function
			cids, err := readCIDsFromInput(tmpFile.Name())

			if tt.expectedError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if len(cids) != len(tt.expectedCIDs) {
				t.Errorf("Expected %d CIDs, got %d", len(tt.expectedCIDs), len(cids))
			}

			for i, expectedCID := range tt.expectedCIDs {
				if i >= len(cids) || cids[i] != expectedCID {
					t.Errorf("Expected CID %d to be %s, got %s", i, expectedCID, cids[i])
				}
			}
		})
	}
}

func TestEscapeCSV(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "no special characters",
			input:    "simple text",
			expected: "simple text",
		},
		{
			name:     "contains comma",
			input:    "text, with comma",
			expected: "\"text, with comma\"",
		},
		{
			name:     "contains quote",
			input:    "text with \"quote\"",
			expected: "\"text with \"\"quote\"\"\"",
		},
		{
			name:     "contains newline",
			input:    "text with\nnewline",
			expected: "\"text with\nnewline\"",
		},
		{
			name:     "contains multiple special chars",
			input:    "text, with \"quote\" and\nnewline",
			expected: "\"text, with \"\"quote\"\" and\nnewline\"",
		},
		{
			name:     "empty string",
			input:    "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := escapeCSV(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestSetupOutputWriter(t *testing.T) {
	tests := []struct {
		name         string
		outputFile   string
		expectStdout bool
		expectError  bool
	}{
		{
			name:         "stdout",
			outputFile:   "-",
			expectStdout: true,
			expectError:  false,
		},
		{
			name:         "regular file",
			outputFile:   "test-output.csv",
			expectStdout: false,
			expectError:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			writer, err := setupOutputWriter(tt.outputFile)

			if tt.expectError && err == nil {
				t.Errorf("Expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if tt.expectStdout {
				if writer != os.Stdout {
					t.Errorf("Expected stdout but got different writer")
				}
			} else {
				if writer == os.Stdout {
					t.Errorf("Expected file writer but got stdout")
				}
				if writer != nil {
					defer writer.Close()
					defer os.Remove(tt.outputFile)
				}
			}
		})
	}
}

func TestRetrievabilityResult(t *testing.T) {
	// Test the RetrievabilityResult struct initialization
	result := RetrievabilityResult{
		CID:         "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
		Retrievable: true,
		Duration:    time.Second * 2,
		Error:       "",
	}

	if result.CID != "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o" {
		t.Errorf("Expected CID to be set correctly")
	}
	if !result.Retrievable {
		t.Errorf("Expected Retrievable to be true")
	}
	if result.Duration != time.Second*2 {
		t.Errorf("Expected Duration to be 2 seconds")
	}
	if result.Error != "" {
		t.Errorf("Expected Error to be empty")
	}
}

func TestLogResult(t *testing.T) {
	// Test that logResult doesn't panic with different inputs
	result := RetrievabilityResult{
		CID:         "QmT78zSuBmuS4z925WZfrqQ1qHaJ56DQaTfyMUF7F8ff5o",
		Retrievable: true,
		Duration:    time.Second,
		Error:       "",
	}

	// Test verbose mode
	logResult(result, true)

	// Test non-verbose mode
	logResult(result, false)

	// Test with error
	result.Retrievable = false
	result.Error = "test error"

	logResult(result, true)
	logResult(result, false)
}
