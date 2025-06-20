package models

import (
	"encoding/base64"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/skip2/go-qrcode"
)

// QRCode represents a QR code and its metadata
type QRCode struct {
	Id        int64     `json:"id" gorm:"column:id; primary_key:yes"`
	UserId    int64     `json:"user_id" gorm:"column:user_id"`
	URL       string    `json:"url" gorm:"column:url"`
	Size      string    `json:"size" gorm:"column:size"`
	Filename  string    `json:"filename,omitempty" gorm:"column:filename"` // Optional filename for download
	CreatedAt time.Time `json:"created_at" gorm:"column:created_at"`
}

// TableName specifies the database table name for the QRCode model
func (QRCode) TableName() string {
	return "qr_codes"
}

// GetQRCodes returns all QR codes in the database
func GetQRCodes(uid int64) ([]QRCode, error) {
	qr_codes := []QRCode{}
	err := db.Where("user_id=?", uid).Order("created_at desc").Find(&qr_codes).Error
	return qr_codes, err
}

// GetQRCode returns the QRCode with the given id and user_id
func GetQRCode(id int64, uid int64) (QRCode, error) {
	qr := QRCode{}
	err := db.Where("user_id=? and id=?", uid, id).First(&qr).Error
	return qr, err
}

// Save adds a QRCode to the database
func (qr *QRCode) Save(uid int64) error {
	qr.UserId = uid
	qr.CreatedAt = time.Now().UTC()
	return db.Save(qr).Error
}

// Delete removes a QRCode from the database
func (qr *QRCode) Delete() error {
	return db.Delete(qr).Error
}

func PostQRCode(qr *QRCode) ([]byte, string, error) {
	// Generate QR code and return the image data for download
	qrData, filename, err := generateQRImage(qr.URL, qr.Size)
	if err != nil {
		return nil, "", fmt.Errorf("%v", err)
	}

	// If no filename was provided, use the generated one
	if qr.Filename == "" {
		qr.Filename = filename
	}

	return qrData, qr.Filename, nil
}

func generateQRImage(url string, strSize string) ([]byte, string, error) {
	// Generate the QR code
	qrCode, err := qrcode.New(url, qrcode.Medium)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate QR code: %v", err)
	}
	qrCode.DisableBorder = true

	// Convert the size to an int
	size, err := strconv.Atoi(strSize)
	if err != nil {
		return nil, "", fmt.Errorf("failed to convert QR code size to int: %v", err)
	}

	// Generate a filename based on the URL
	// Create a safe filename from the URL
	safeURL := url
	// Remove protocol and special characters for filename
	safeURL = strings.TrimPrefix(safeURL, "http://")
	safeURL = strings.TrimPrefix(safeURL, "https://")
	safeURL = strings.TrimPrefix(safeURL, "www.")
	// Replace special characters with underscores
	re := regexp.MustCompile(`[^a-zA-Z0-9]`)
	safeURL = re.ReplaceAllString(safeURL, "_")
	// Limit filename length
	if len(safeURL) > 30 {
		safeURL = safeURL[:30]
	}
	filename := fmt.Sprintf("qrcode_%s.png", safeURL)

	// Get the PNG image as bytes
	qrCodeBytes, err := qrCode.PNG(size)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate QR code PNG: %v", err)
	}

	return qrCodeBytes, filename, nil
}

// generateQRCode generates a QR code with the given content and size, saves it to a temporary file,
// encodes the image to a base64 string, and then deletes the temporary file.
func generateQRCode(content string, stringSize string) (string, string, error) {
	// Set the temporary directory
	tempDir := "./temp_qr_codes"

	// Ensure the temporary directory exists
	if _, err := os.Stat(tempDir); os.IsNotExist(err) {
		err := os.MkdirAll(tempDir, os.ModePerm)
		if err != nil {
			return "", "", fmt.Errorf("failed to create temporary directory: %v", err)
		}
	}

	// Generate the QR code
	qrCode, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return "", "", fmt.Errorf("failed to generate QR code: %v", err)
	}
	qrCode.DisableBorder = true

	// Create a temporary file for the QR code. The pattern "qr-*.png" ensures a unique filename for each QR code.
	tempFile, err := ioutil.TempFile(tempDir, "*.png")
	if err != nil {
		return "", "", fmt.Errorf("failed to create temporary file: %v", err)
	}
	tempFilePath := tempFile.Name()
	tempFileName := filepath.Base(tempFilePath) // Extract just the filename
	tempFile.Close()                            // Close the file so it can be removed after reading

	// Convert the size to an int
	size, err := strconv.Atoi(stringSize)
	if err != nil {
		return "", "", fmt.Errorf("failed to convert QR code size to int: %v", err)
	}

	// Write the QR code to the file
	err = qrCode.WriteFile(size, tempFilePath)
	if err != nil {
		os.Remove(tempFilePath) // Attempt to remove the file in case of error
		return "", "", fmt.Errorf("failed to write QR code to file: %v", err)
	}

	// Read the file back to get the byte slice
	qrCodeBytes, err := ioutil.ReadFile(tempFilePath)
	if err != nil {
		os.Remove(tempFilePath) // Clean up
		return "", "", fmt.Errorf("failed to read temporary QR code file: %v", err)
	}

	// Delete the temporary file
	err = os.Remove(tempFilePath)
	if err != nil {
		// Log the error but proceed
		fmt.Println("Warning: Failed to delete temporary QR code file:", err)
	}

	// Encode the byte slice to a base64 string
	base64QRCode := base64.StdEncoding.EncodeToString(qrCodeBytes)

	return base64QRCode, tempFileName, nil
}
