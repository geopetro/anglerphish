package reports

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	log "github.com/gophish/gophish/logger"
)

// DefaultStoragePath is used when no storage path is configured.
var DefaultStoragePath = filepath.Join("reports", "files")

// ErrOutsideStorage is returned for paths outside the report storage directory.
var ErrOutsideStorage = errors.New("file path outside of storage directory")

// FileManager handles report file storage, retrieval, and deletion
type FileManager struct {
	baseStoragePath string // absolute
}

// NewFileManager creates a file manager rooted at storagePath (or the
// default path when empty). The path is resolved to an absolute path once,
// so stored file paths keep working whatever the working directory is later.
func NewFileManager(storagePath string) *FileManager {
	if storagePath == "" {
		storagePath = DefaultStoragePath
	}
	abs, err := filepath.Abs(storagePath)
	if err != nil {
		log.Errorf("Error resolving reports directory %s: %v", storagePath, err)
		abs = storagePath
	}
	if err := os.MkdirAll(abs, 0750); err != nil {
		log.Errorf("Error creating reports directory: %v", err)
	}
	return &FileManager{baseStoragePath: abs}
}

// StoragePath returns the absolute base storage path
func (fm *FileManager) StoragePath() string {
	return fm.baseStoragePath
}

// SaveReport saves a report file under <storage>/<year>/<month>/ and
// returns its absolute path.
func (fm *FileManager) SaveReport(fileName string, data []byte) (string, error) {
	now := time.Now()
	dirPath := filepath.Join(fm.baseStoragePath, fmt.Sprintf("%d", now.Year()), fmt.Sprintf("%02d", now.Month()))
	if err := os.MkdirAll(dirPath, 0750); err != nil {
		return "", fmt.Errorf("error creating report directory: %v", err)
	}
	filePath := filepath.Join(dirPath, filepath.Base(fileName))
	if err := os.WriteFile(filePath, data, 0640); err != nil {
		return "", fmt.Errorf("error writing report file: %v", err)
	}
	log.Infof("Saved report file: %s (%d bytes)", filePath, len(data))
	return filePath, nil
}

// resolve returns the absolute form of filePath if it lies strictly inside
// the storage directory. Relative paths stored by older versions are
// resolved against the working directory, as they were when written.
func (fm *FileManager) resolve(filePath string) (string, error) {
	absFilePath, err := filepath.Abs(filePath)
	if err != nil {
		return "", fmt.Errorf("error getting absolute file path: %v", err)
	}
	rel, err := filepath.Rel(fm.baseStoragePath, absFilePath)
	if err != nil || rel == "." || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", ErrOutsideStorage
	}
	return absFilePath, nil
}

// OpenReportFile opens a report file for reading. The caller closes it.
func (fm *FileManager) OpenReportFile(filePath string) (*os.File, os.FileInfo, error) {
	abs, err := fm.resolve(filePath)
	if err != nil {
		return nil, nil, err
	}
	f, err := os.Open(abs)
	if err != nil {
		return nil, nil, err
	}
	info, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, nil, err
	}
	if info.IsDir() {
		f.Close()
		return nil, nil, fmt.Errorf("report path is a directory: %s", abs)
	}
	return f, info, nil
}

// DeleteReportFile removes a report file from storage. A file that is
// already gone is not an error.
func (fm *FileManager) DeleteReportFile(filePath string) error {
	abs, err := fm.resolve(filePath)
	if err != nil {
		return err
	}
	if err := os.Remove(abs); err != nil {
		if os.IsNotExist(err) {
			log.Warnf("Report file already deleted or not found: %s", abs)
			return nil
		}
		return fmt.Errorf("error deleting report file: %v", err)
	}
	log.Infof("Deleted report file: %s", abs)
	return nil
}
