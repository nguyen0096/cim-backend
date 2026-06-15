package services

import (
	"cim-backend/internal/config"
	"cim-backend/internal/models"
	"cim-backend/pkg"
	"cim-backend/pkg/log"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/sirupsen/logrus"
)

//go:generate mockery --name=FileStorageService --structname=FileStorageService --output=../mocks/servicemocks --outpkg=servicemocks
type FileStorageService interface {
	SaveFile(ctx context.Context, file multipart.File, filename string, category pkg.FileCategory) (fileUID string, extension string, err error)
	GetFilePath(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) (string, error)
	DeleteFile(ctx context.Context, fileUID string, extension string, category pkg.FileCategory) error
	EnsureUploadDirectories() error

	// v1 - AGENTS MUST CONFIRM BEFORE MODIFYING SECTION BELOW THIS LINE

	// PopulateExportURL populates the DownloadURL field of an ExportFile with a presigned URL.
	PopulateExportURL(ctx context.Context, export *models.ExportFile) error
}

type fileStorageService struct {
	uploadsBasePath string
	s3Client        S3Client
	r2Enabled       bool
	exportPrefix    string
}

// NewFileStorageService creates a new file storage service
func NewFileStorageService(cfg *config.Config, s3Client S3Client) FileStorageService {
	service := &fileStorageService{
		uploadsBasePath: cfg.UploadsBasePath,
		s3Client:        s3Client,
		r2Enabled:       cfg.R2.Enabled,
		exportPrefix:    cfg.R2.ExportPrefix,
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

func (s *fileStorageService) PopulateExportURL(ctx context.Context, export *models.ExportFile) error {
	log.WithFields(logrus.Fields{
		"operation":  "PopulateExportURL",
		"r2_enabled": s.r2Enabled,
	}).Info("Populating export URL")

	if !s.r2Enabled {
		log.Warn("R2 is disabled, skipping export URL population")
		return nil
	}

	// Validate export file
	if export == nil {
		return pkg.ErrValidation("export file cannot be nil", nil)
	}
	if len(export.Content) == 0 {
		return pkg.ErrValidation("export file content cannot be empty", nil)
	}

	// Generate file key with date organization. This legacy path has no
	// inventory/period identity, so it uses the config-prefixed fallback shape
	// (<prefix>/inventory/YYYY/MM/DD/<uuid>.xlsx).
	now := time.Now()
	fileKey := buildExportFallbackKey(s.exportPrefix, now)

	log.WithFields(logrus.Fields{
		"file_key":     fileKey,
		"content_size": len(export.Content),
	}).Debug("Generated file key")

	// Determine content type from FileType MIME
	contentType := "application/vnd.openxmlformats-officedocument.spreadsheetml.sheet"
	if fileTypeStr := export.FileType.String(); fileTypeStr != "" {
		contentType = fileTypeStr
	}

	// Upload to R2
	if err := s.s3Client.UploadFile(ctx, fileKey, export.Content, contentType); err != nil {
		log.WithFields(logrus.Fields{
			"error":    err,
			"file_key": fileKey,
		}).Error("Failed to upload file to R2")
		return fmt.Errorf("failed to upload file to R2: %w", err)
	}

	log.WithFields(logrus.Fields{
		"file_key": fileKey,
	}).Info("Successfully uploaded file to R2")

	// Generate presigned URL with 15-minute expiration
	presignedURL, err := s.s3Client.GeneratePresignedURL(ctx, fileKey, 15*time.Minute)
	if err != nil {
		log.WithFields(logrus.Fields{
			"error":    err,
			"file_key": fileKey,
		}).Error("Failed to generate presigned URL")
		return fmt.Errorf("failed to generate presigned URL: %w", err)
	}

	export.DownloadURL = presignedURL

	log.WithFields(logrus.Fields{
		"file_key":   fileKey,
		"url_length": len(presignedURL),
	}).Info("Successfully populated export URL")

	return nil
}
