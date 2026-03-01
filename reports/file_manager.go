package reports

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	log "github.com/gophish/gophish/logger"
)

// FileManager handles report file storage, retrieval, and cleanup
type FileManager struct {
	baseStoragePath string
}

// NewFileManager creates a new file manager instance
func NewFileManager() *FileManager {
	// Default storage path - this should be configurable
	basePath := filepath.Join(".", "reports", "files")

	// Ensure the directory exists
	if err := os.MkdirAll(basePath, 0755); err != nil {
		log.Errorf("Error creating reports directory: %v", err)
	}

	return &FileManager{
		baseStoragePath: basePath,
	}
}

// NewFileManagerWithPath creates a new file manager with a custom storage path
func NewFileManagerWithPath(storagePath string) *FileManager {
	// Ensure the directory exists
	if err := os.MkdirAll(storagePath, 0755); err != nil {
		log.Errorf("Error creating reports directory: %v", err)
	}

	return &FileManager{
		baseStoragePath: storagePath,
	}
}

// SaveReport saves a report file to storage and returns the file path
func (fm *FileManager) SaveReport(reportId int64, fileName string, data []byte) (string, error) {
	// Create a directory structure: reports/files/year/month/
	now := time.Now()
	dirPath := filepath.Join(fm.baseStoragePath, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))

	// Ensure the directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", fmt.Errorf("error creating report directory: %v", err)
	}

	// Create the full file path
	filePath := filepath.Join(dirPath, fileName)

	// Write the file
	err := ioutil.WriteFile(filePath, data, 0644)
	if err != nil {
		return "", fmt.Errorf("error writing report file: %v", err)
	}

	log.Infof("Saved report file: %s (%d bytes)", filePath, len(data))
	return filePath, nil
}

// GetReportFile reads a report file from storage
func (fm *FileManager) GetReportFile(filePath string) ([]byte, error) {
	// Ensure the file path is within our storage directory for security
	absStoragePath, err := filepath.Abs(fm.baseStoragePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute storage path: %v", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return nil, fmt.Errorf("error getting absolute file path: %v", err)
	}

	// Check if the file is within our storage directory
	rel, err := filepath.Rel(absStoragePath, absFilePath)
	if err != nil || len(rel) == 0 || rel[0] == '.' && rel[1] == '.' {
		return nil, fmt.Errorf("file path outside of storage directory")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return nil, fmt.Errorf("report file not found: %s", filePath)
	}

	// Read the file
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("error reading report file: %v", err)
	}

	return data, nil
}

// DeleteReportFile removes a report file from storage
func (fm *FileManager) DeleteReportFile(filePath string) error {
	// Ensure the file path is within our storage directory for security
	absStoragePath, err := filepath.Abs(fm.baseStoragePath)
	if err != nil {
		return fmt.Errorf("error getting absolute storage path: %v", err)
	}

	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return fmt.Errorf("error getting absolute file path: %v", err)
	}

	// Check if the file is within our storage directory
	rel, err := filepath.Rel(absStoragePath, absFilePath)
	if err != nil || len(rel) == 0 || rel[0] == '.' && rel[1] == '.' {
		return fmt.Errorf("file path outside of storage directory")
	}

	// Check if file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		log.Warnf("Report file already deleted or not found: %s", filePath)
		return nil // Not an error if already deleted
	}

	// Delete the file
	err = os.Remove(filePath)
	if err != nil {
		return fmt.Errorf("error deleting report file: %v", err)
	}

	log.Infof("Deleted report file: %s", filePath)
	return nil
}

// GetFileSize returns the size of a file
func (fm *FileManager) GetFileSize(filePath string) (int64, error) {
	info, err := os.Stat(filePath)
	if err != nil {
		return 0, err
	}
	return info.Size(), nil
}

// FileExists checks if a file exists
func (fm *FileManager) FileExists(filePath string) bool {
	_, err := os.Stat(filePath)
	return !os.IsNotExist(err)
}

// CleanupOldFiles removes files older than the specified duration
func (fm *FileManager) CleanupOldFiles(maxAge time.Duration) (int, error) {
	cutoffTime := time.Now().Add(-maxAge)
	cleanedCount := 0

	err := filepath.Walk(fm.baseStoragePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			log.Errorf("Error walking directory %s: %v", path, err)
			return nil // Continue walking
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is older than cutoff
		if info.ModTime().Before(cutoffTime) {
			log.Infof("Cleaning up old report file: %s (modified: %s)", path, info.ModTime())

			err := os.Remove(path)
			if err != nil {
				log.Errorf("Error deleting old file %s: %v", path, err)
			} else {
				cleanedCount++
			}
		}

		return nil
	})

	if err != nil {
		return cleanedCount, fmt.Errorf("error walking directory: %v", err)
	}

	if cleanedCount > 0 {
		log.Infof("Cleaned up %d old report files", cleanedCount)
	}

	return cleanedCount, nil
}

// GetStorageStats returns statistics about the storage directory
func (fm *FileManager) GetStorageStats() (map[string]interface{}, error) {
	stats := map[string]interface{}{
		"total_files":  0,
		"total_size":   int64(0),
		"storage_path": fm.baseStoragePath,
	}

	fileCount := 0
	var totalSize int64

	err := filepath.Walk(fm.baseStoragePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Continue walking on errors
		}

		// Skip directories
		if !info.IsDir() {
			fileCount++
			totalSize += info.Size()
		}

		return nil
	})

	if err != nil {
		return stats, fmt.Errorf("error walking directory: %v", err)
	}

	stats["total_files"] = fileCount
	stats["total_size"] = totalSize

	return stats, nil
}

// GetStoragePath returns the base storage path
func (fm *FileManager) GetStoragePath() string {
	return fm.baseStoragePath
}

// CreateStorageStructure ensures the storage directory structure exists
func (fm *FileManager) CreateStorageStructure() error {
	// Create base directory
	if err := os.MkdirAll(fm.baseStoragePath, 0755); err != nil {
		return fmt.Errorf("error creating base storage directory: %v", err)
	}

	// Create year/month structure for current year
	now := time.Now()
	for month := 1; month <= 12; month++ {
		monthDir := filepath.Join(fm.baseStoragePath, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", month))
		if err := os.MkdirAll(monthDir, 0755); err != nil {
			log.Warnf("Error creating month directory %s: %v", monthDir, err)
		}
	}

	log.Infof("Created storage structure at: %s", fm.baseStoragePath)
	return nil
}

// ValidateFile performs basic validation on a report file
func (fm *FileManager) ValidateFile(filePath string, expectedMinSize int64) error {
	// Check if file exists
	info, err := os.Stat(filePath)
	if err != nil {
		return fmt.Errorf("file validation failed - file does not exist: %v", err)
	}

	// Check file size
	if info.Size() < expectedMinSize {
		return fmt.Errorf("file validation failed - file too small: %d bytes (expected at least %d)", info.Size(), expectedMinSize)
	}

	// Check file is readable
	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("file validation failed - cannot read file: %v", err)
	}
	file.Close()

	return nil
}
