package storage

import (
	"bytes"
	"fmt"
	"io"
	"oncloud/models"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
)

// R2Client implements StorageInterface for Cloudflare R2
type R2Client struct {
	client     *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
	provider   *models.StorageProvider
	bucket     string
	accountID  string
}

// NewR2Client creates a new Cloudflare R2 client
func NewR2Client(provider *models.StorageProvider) (*R2Client, error) {
	// Extract account ID from settings
	accountID, ok := provider.Settings["account_id"].(string)
	if !ok || accountID == "" {
		return nil, fmt.Errorf("account_id is required for Cloudflare R2")
	}

	// Set R2 endpoint
	endpoint := fmt.Sprintf("https://%s.r2.cloudflarestorage.com", accountID)
	if provider.Endpoint != "" {
		endpoint = provider.Endpoint
	}

	config := &aws.Config{
		Region:           aws.String("auto"),
		Endpoint:         aws.String(endpoint),
		S3ForcePathStyle: aws.Bool(true),
	}

	// Set credentials
	if provider.AccessKey != "" && provider.SecretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(
			provider.AccessKey,
			provider.SecretKey,
			"",
		)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create R2 session: %v", err)
	}

	client := s3.New(sess)

	return &R2Client{
		client:     client,
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
		provider:   provider,
		bucket:     provider.Bucket,
		accountID:  accountID,
	}, nil
}

// Upload uploads data to R2
func (r *R2Client) Upload(key string, data []byte) error {
	_, err := r.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})

	if err != nil {
		return NewStorageError("r2", "UPLOAD_FAILED", err.Error(), key)
	}

	return nil
}

// UploadStream uploads data from a stream to R2
func (r *R2Client) UploadStream(key string, reader io.Reader, size int64) error {
	_, err := r.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
		Body:   reader,
	})

	if err != nil {
		return NewStorageError("r2", "UPLOAD_STREAM_FAILED", err.Error(), key)
	}

	return nil
}

// Download downloads data from R2
func (r *R2Client) Download(key string) ([]byte, error) {
	result, err := r.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, NewStorageError("r2", "DOWNLOAD_FAILED", err.Error(), key)
	}
	defer result.Body.Close()

	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, NewStorageError("r2", "READ_FAILED", err.Error(), key)
	}

	return data, nil
}

// DownloadStream returns a stream for downloading from R2
func (r *R2Client) DownloadStream(key string) (io.ReadCloser, error) {
	result, err := r.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, NewStorageError("r2", "DOWNLOAD_STREAM_FAILED", err.Error(), key)
	}

	return result.Body, nil
}

// Delete deletes a file from R2
func (r *R2Client) Delete(key string) error {
	_, err := r.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return NewStorageError("r2", "DELETE_FAILED", err.Error(), key)
	}

	return nil
}

// Exists checks if a file exists in R2
func (r *R2Client) Exists(key string) (bool, error) {
	_, err := r.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, NewStorageError("r2", "HEAD_FAILED", err.Error(), key)
	}

	return true, nil
}

// GetSize gets the size of a file in R2
func (r *R2Client) GetSize(key string) (int64, error) {
	result, err := r.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return 0, NewStorageError("r2", "HEAD_FAILED", err.Error(), key)
	}

	return *result.ContentLength, nil
}

// GetURL gets the public URL for a file
func (r *R2Client) GetURL(key string) (string, error) {
	if r.provider.CDNUrl != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(r.provider.CDNUrl, "/"), key), nil
	}

	// R2 public URL format
	return fmt.Sprintf("https://%s.r2.cloudflarestorage.com/%s", r.accountID, key), nil
}

// GetPresignedURL generates a presigned URL for downloading
func (r *R2Client) GetPresignedURL(key string, expiry time.Duration) (string, error) {
	req, _ := r.client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	url, err := req.Presign(expiry)
	if err != nil {
		return "", NewStorageError("r2", "PRESIGN_FAILED", err.Error(), key)
	}

	return url, nil
}

// GetPresignedUploadURL generates a presigned URL for uploading
func (r *R2Client) GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error) {
	req, _ := r.client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	url, err := req.Presign(expiry)
	if err != nil {
		return "", NewStorageError("r2", "PRESIGN_UPLOAD_FAILED", err.Error(), key)
	}

	return url, nil
}

// InitiateMultipartUpload starts a multipart upload
func (r *R2Client) InitiateMultipartUpload(key string) (*MultipartUpload, error) {
	result, err := r.client.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(r.bucket),
		Key:    aws.String(key),
	})

	if err != nil {
		return nil, NewStorageError("r2", "MULTIPART_INIT_FAILED", err.Error(), key)
	}

	return &MultipartUpload{
		UploadID: *result.UploadId,
		Key:      key,
		Provider: "r2",
	}, nil
}

// UploadPart uploads a part in multipart upload
func (r *R2Client) UploadPart(uploadID, key string, partNumber int, data []byte) (*UploadPart, error) {
	result, err := r.client.UploadPart(&s3.UploadPartInput{
		Bucket:     aws.String(r.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int64(int64(partNumber)),
		Body:       bytes.NewReader(data),
	})

	if err != nil {
		return nil, NewStorageError("r2", "MULTIPART_UPLOAD_FAILED", err.Error(), key)
	}

	return &UploadPart{
		PartNumber: partNumber,
		ETag:       strings.Trim(*result.ETag, "\""),
		Size:       int64(len(data)),
	}, nil
}

// CompleteMultipartUpload completes a multipart upload
func (r *R2Client) CompleteMultipartUpload(uploadID, key string, parts []UploadPart) error {
	completedParts := make([]*s3.CompletedPart, len(parts))

	for i, part := range parts {
		completedParts[i] = &s3.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int64(int64(part.PartNumber)),
		}
	}

	_, err := r.client.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})

	if err != nil {
		return NewStorageError("r2", "MULTIPART_COMPLETE_FAILED", err.Error(), key)
	}

	return nil
}

// AbortMultipartUpload aborts a multipart upload
func (r *R2Client) AbortMultipartUpload(uploadID, key string) error {
	_, err := r.client.AbortMultipartUpload(&s3.AbortMultipartUploadInput{
		Bucket:   aws.String(r.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})

	if err != nil {
		return NewStorageError("r2", "MULTIPART_ABORT_FAILED", err.Error(), key)
	}

	return nil
}

// DeleteMultiple deletes multiple files
func (r *R2Client) DeleteMultiple(keys []string) error {
	objects := make([]*s3.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = &s3.ObjectIdentifier{Key: aws.String(key)}
	}

	_, err := r.client.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(r.bucket),
		Delete: &s3.Delete{Objects: objects},
	})

	if err != nil {
		return NewStorageError("r2", "BULK_DELETE_FAILED", err.Error(), "")
	}

	return nil
}

// CopyFile copies a file within R2
func (r *R2Client) CopyFile(sourceKey, destKey string) error {
	_, err := r.client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(r.bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", r.bucket, sourceKey)),
		Key:        aws.String(destKey),
	})

	if err != nil {
		return NewStorageError("r2", "COPY_FAILED", err.Error(), sourceKey)
	}

	return nil
}

// MoveFile moves a file within R2 (copy then delete)
func (r *R2Client) MoveFile(sourceKey, destKey string) error {
	if err := r.CopyFile(sourceKey, destKey); err != nil {
		return err
	}

	return r.Delete(sourceKey)
}

// GetProviderInfo returns provider information
func (r *R2Client) GetProviderInfo() *ProviderInfo {
	return &ProviderInfo{
		Name:        r.provider.Name,
		Type:        "r2",
		Region:      "auto",
		Endpoint:    fmt.Sprintf("https://%s.r2.cloudflarestorage.com", r.accountID),
		CDNUrl:      r.provider.CDNUrl,
		MaxFileSize: r.provider.MaxFileSize,
		Features:    []string{"multipart", "presigned", "cdn", "edge-locations", "zero-egress"},
		Metadata: map[string]string{
			"bucket":     r.bucket,
			"account_id": r.accountID,
		},
	}
}

// HealthCheck checks if the R2 service is accessible
func (r *R2Client) HealthCheck() error {
	_, err := r.client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(r.bucket),
	})

	if err != nil {
		return NewStorageError("r2", "HEALTH_CHECK_FAILED", err.Error(), "")
	}

	return nil
}

// GetStats returns storage statistics (not available in R2)
func (r *R2Client) GetStats() (*StorageStats, error) {
	// R2 doesn't provide direct stats
	return &StorageStats{
		TotalFiles: -1, // Unknown
		TotalSize:  -1, // Unknown
		UsedSpace:  -1, // Unknown
	}, nil
}
