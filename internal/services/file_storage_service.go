package services

import (
	"cim-backend/internal/config"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

//go:generate mockery --name=FileStorageService --structname=FileStorageService --output=../mocks/servicemocks --outpkg=servicemocks
type FileStorageService interface {
	SaveFile(ctx context.Context, file multipart.File, filename string, category pkg.FileCategory) (fileUID string, extension string, err error)
	GetFilePath(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) (string, error)
	DeleteFile(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) error
	EnsureUploadDirectories() error
}

type fileStorageService struct {
	uploadsBasePath string
}

// NewFileStorageService creates a new file storage service
func NewFileStorageService(cfg *config.Config) FileStorageService {
	service := &fileStorageService{
		uploadsBasePath: cfg.UploadsBasePath,
	}

	// Ensure upload directories exist on initialization
	if err := service.EnsureUploadDirectories(); err != nil {
		log.WithFields(logrus.Fields{
			"error":             err,
			"uploads_base_path": cfg.UploadsBasePath,
		}).Error("Failed to ensure upload directories exist")
	}

	return service
}

// SaveFile saves an uploaded file to the appropriate category subdirectory
func (s *fileStorageService) SaveFile(ctx context.Context, file multipart.File, filename string, category pkg.FileCategory) (string, string, error) {
	log.WithFields(logrus.Fields{
		"operation": "SaveFile",
		"filename":  filename,
		"category":  category,
	}).Info("Saving uploaded file")

	// Generate UUID for file UID
	fileUID := uuid.New().String()

	// Extract file extension
	extension := strings.ToLower(filepath.Ext(filename))
	if extension != "" {
		extension = extension[1:] // Remove the leading dot
	}

	if extension == "" {
		return "", "", pkg.ErrValidation("file must have an extension", nil)
	}

	// Get subdirectory for category
	subdir, err := category.GetSubdirectory()
	if err != nil {
		return "", "", fmt.Errorf("failed to get subdirectory for category: %w", err)
	}

	// Build full directory path
	dirPath := filepath.Join(s.uploadsBasePath, subdir)

	// Ensure directory exists
	if err := os.MkdirAll(dirPath, 0755); err != nil {
		return "", "", fmt.Errorf("failed to create directory %s: %w", dirPath, err)
	}

	// Build full file path
	filePath := filepath.Join(dirPath, fmt.Sprintf("%s.%s", fileUID, extension))

	// Create destination file
	destFile, err := os.Create(filePath)
	if err != nil {
		return "", "", fmt.Errorf("failed to create file %s: %w", filePath, err)
	}
	defer destFile.Close()

	// Copy uploaded file to destination
	if _, err := io.Copy(destFile, file); err != nil {
		// Clean up partially written file
		os.Remove(filePath)
		return "", "", fmt.Errorf("failed to copy file content: %w", err)
	}

	log.WithFields(logrus.Fields{
		"operation": "SaveFile",
		"file_uid":  fileUID,
		"extension": extension,
		"path":      filePath,
	}).Info("Successfully saved file")

	return fileUID, extension, nil
}

// GetFilePath returns the full path for a stored file
func (s *fileStorageService) GetFilePath(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) (string, error) {
	log.WithFields(logrus.Fields{
		"operation": "GetFilePath",
		"file_uid":  fileUID,
		"extension": extension,
		"category":  category,
	}).Debug("Getting file path")

	// Get subdirectory for category
	subdir, err := category.GetSubdirectory()
	if err != nil {
		return "", fmt.Errorf("failed to get subdirectory for category: %w", err)
	}

	// Build full file path
	filePath := filepath.Join(s.uploadsBasePath, subdir, fmt.Sprintf("%s.%s", fileUID, extension))

	// Verify file exists
	if _, err := os.Stat(filePath); os.IsNotExist(err) {
		return "", pkg.NewAppError(pkg.ErrorCodeNotFound, fmt.Sprintf("file not found: %s", fileUID), nil)
	} else if err != nil {
		return "", fmt.Errorf("failed to check file existence: %w", err)
	}

	return filePath, nil
}

// DeleteFile deletes a stored file (idempotent)
func (s *fileStorageService) DeleteFile(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) error {
	log.WithFields(logrus.Fields{
		"operation": "DeleteFile",
		"file_uid":  fileUID,
		"extension": extension,
		"category":  category,
	}).Info("Deleting file")

	// Get subdirectory for category
	subdir, err := category.GetSubdirectory()
	if err != nil {
		return fmt.Errorf("failed to get subdirectory for category: %w", err)
	}

	// Build full file path
	filePath := filepath.Join(s.uploadsBasePath, subdir, fmt.Sprintf("%s.%s", fileUID, extension))

	// Delete file (ignore if doesn't exist - idempotent)
	if err := os.Remove(filePath); err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("failed to delete file %s: %w", filePath, err)
	}

	log.WithFields(logrus.Fields{
		"operation": "DeleteFile",
		"file_uid":  fileUID,
		"path":      filePath,
	}).Info("Successfully deleted file")

	return nil
}

// EnsureUploadDirectories creates all subdirectories for defined file categories
func (s *fileStorageService) EnsureUploadDirectories() error {
	log.WithFields(logrus.Fields{
		"operation":         "EnsureUploadDirectories",
		"uploads_base_path": s.uploadsBasePath,
	}).Info("Ensuring upload directories exist")

	// Create each category subdirectory
	for category, subdir := range pkg.FileCategorySubdirs {
		dirPath := filepath.Join(s.uploadsBasePath, subdir)
		if err := os.MkdirAll(dirPath, 0755); err != nil {
			return fmt.Errorf("failed to create directory for category %s at %s: %w", category, dirPath, err)
		}
		log.WithFields(logrus.Fields{
			"category": category,
			"path":     dirPath,
		}).Debug("Ensured directory exists")
	}

	log.Info("Successfully ensured all upload directories exist")
	return nil
}
