package main

import (
	"compress/gzip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/pierrec/lz4/v4"
)

func TestFindVolumeBackupPath(t *testing.T) {
	tmpDir := t.TempDir()
	volumePath := filepath.Join(tmpDir, "volumes", "ab", "cd", "volume1")
	err := os.MkdirAll(volumePath, 0755)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		backupStore   string
		volumeName    string
		expectedPath  string
		expectedError bool
	}{
		{
			name:          "Valid volume backup",
			backupStore:   tmpDir,
			volumeName:    "volume1",
			expectedPath:  volumePath,
			expectedError: false,
		},
		{
			name:          "Non-existent volume",
			backupStore:   tmpDir,
			volumeName:    "nonexistent",
			expectedPath:  "",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path, err := findVolumeBackupPath(tt.backupStore, tt.volumeName)
			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if path != tt.expectedPath {
				t.Errorf("Expected path %s, got %s", tt.expectedPath, path)
			}
		})
	}
}

func TestReadBackups(t *testing.T) {
	tmpDir := t.TempDir()
	backupsDir := filepath.Join(tmpDir, "backups")
	err := os.MkdirAll(backupsDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	// Create mock backup config file
	mockConfig := `{
        "CreatedTime": "2023-01-01T00:00:00Z",
        "Size": "1024",
        "CompressionMethod": "lz4",
        "Blocks": [
            {
                "Offset": 0,
                "BlockChecksum": "test123"
            }
        ]
    }`

	err = os.WriteFile(filepath.Join(backupsDir, "backup1.cfg"), []byte(mockConfig), 0644)
	if err != nil {
		t.Fatal(err)
	}

	volumeBackup, err := readBackups(tmpDir)
	if err != nil {
		t.Errorf("Expected no error, got %v", err)
	}

	if volumeBackup == nil {
		t.Fatal("Expected non-nil VolumeBackup")
	}

	if len(volumeBackup.Backups) != 1 {
		t.Errorf("Expected 1 backup, got %d", len(volumeBackup.Backups))
	}
}

func TestResolveBlockPath(t *testing.T) {
	// Create temporary test directory with mock block
	tmpDir := t.TempDir()
	blocksDir := filepath.Join(tmpDir, "blocks", "ab", "cd")
	err := os.MkdirAll(blocksDir, 0755)
	if err != nil {
		t.Fatal(err)
	}

	mockBlockPath := filepath.Join(blocksDir, "testchecksum.blk")
	err = os.WriteFile(mockBlockPath, []byte("test"), 0644)
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name          string
		backupPath    string
		checksum      string
		expectedError bool
	}{
		{
			name:          "Valid block",
			backupPath:    tmpDir,
			checksum:      "testchecksum",
			expectedError: false,
		},
		{
			name:          "Non-existent block",
			backupPath:    tmpDir,
			checksum:      "nonexistent",
			expectedError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := resolveBlockPath(tt.backupPath, tt.checksum)
			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestWriteBlockToBuffer(t *testing.T) {
	tmpFile, err := os.CreateTemp("", "test-write-block")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	testData := []byte("test data")
	writeBlockToBuffer(testData, 10, tmpFile)

	// Verify written data
	tmpFile.Seek(10, 0)
	readData := make([]byte, len(testData))
	_, err = tmpFile.Read(readData)
	if err != nil {
		t.Fatal(err)
	}

	if string(readData) != string(testData) {
		t.Errorf("Expected %s, got %s", string(testData), string(readData))
	}
}

func TestDecompression(t *testing.T) {
	test_string := "hello world"
	r := strings.NewReader(test_string)

	pr, pw := io.Pipe()
	zw := lz4.NewWriter(pw)
	defer zw.Close()

	go func() {
		_, _ = io.Copy(zw, r)
		_ = zw.Close()
		_ = pw.Close()
	}()

	compressed_data_lz4, _ := io.ReadAll(pr)

	pr2, pw2 := io.Pipe()
	zw2 := gzip.NewWriter(pw2)
	r2 := strings.NewReader(test_string)

	go func() {
		_, _ = io.Copy(zw2, r2)
		zw2.Close()
		pw2.Close()
	}()

	compressed_data_gzip, _ := io.ReadAll(pr2)

	t.Log(string(compressed_data_gzip))
	tests := []struct {
		name          string
		data          []byte
		compression   string
		expectedError bool
	}{
		{
			name:          "Decompress LZ4",
			data:          compressed_data_lz4,
			compression:   "lz4",
			expectedError: false,
		},

		{
			name:          "Decompress GZIP",
			data:          compressed_data_gzip,
			compression:   "gzip",
			expectedError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			decompressed_data, err := decompressLZ4(tt.data)
			if tt.compression == "gzip" {
				decompressed_data, err = decompressGZIP(tt.data)
			} else if tt.compression == "lz4" {
				decompressed_data, err = decompressLZ4(tt.data)
			}
			if tt.expectedError && err == nil {
				t.Error("Expected error but got none")
			}
			if !tt.expectedError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
			if string(decompressed_data) != test_string {
				t.Errorf("Expected '%s', got '%s'", test_string, string(decompressed_data))
			}
		})
	}
}
