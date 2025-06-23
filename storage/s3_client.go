package storage

import (
	"bytes"
	"fmt"
	"io"
	"strings"
	"time"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"oncloud/models"
)

// S3Client implements StorageInterface for Amazon S3
type S3Client struct {
	client     *s3.S3
	uploader   *s3manager.Uploader
	downloader *s3manager.Downloader
	provider   *models.StorageProvider
	bucket     string
	region     string
}

// NewS3Client creates a new S3 client
func NewS3Client(provider *models.StorageProvider) (*S3Client, error) {
	config := &aws.Config{
		Region: aws.String(provider.Region),
	}

	// Set credentials if provided
	if provider.AccessKey != "" && provider.SecretKey != "" {
		config.Credentials = credentials.NewStaticCredentials(
			provider.AccessKey,
			provider.SecretKey,
			"",
		)
	}

	// Set custom endpoint if provided (for S3-compatible services)
	if provider.Endpoint != "" {
		config.Endpoint = aws.String(provider.Endpoint)
		config.S3ForcePathStyle = aws.Bool(true)
	}

	sess, err := session.NewSession(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create AWS session: %v", err)
	}

	client := s3.New(sess)
	
	return &S3Client{
		client:     client,
		uploader:   s3manager.NewUploader(sess),
		downloader: s3manager.NewDownloader(sess),
		provider:   provider,
		bucket:     provider.Bucket,
		region:     provider.Region,
	}, nil
}

// Upload uploads data to S3
func (s *S3Client) Upload(key string, data []byte) error {
	_, err := s.client.PutObject(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   bytes.NewReader(data),
	})
	
	if err != nil {
		return NewStorageError("s3", "UPLOAD_FAILED", err.Error(), key)
	}
	
	return nil
}

// UploadStream uploads data from a stream to S3
func (s *S3Client) UploadStream(key string, reader io.Reader, size int64) error {
	_, err := s.uploader.Upload(&s3manager.UploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
		Body:   reader,
	})
	
	if err != nil {
		return NewStorageError("s3", "UPLOAD_STREAM_FAILED", err.Error(), key)
	}
	
	return nil
}

// Download downloads data from S3
func (s *S3Client) Download(key string) ([]byte, error) {
	result, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return nil, NewStorageError("s3", "DOWNLOAD_FAILED", err.Error(), key)
	}
	defer result.Body.Close()
	
	data, err := io.ReadAll(result.Body)
	if err != nil {
		return nil, NewStorageError("s3", "READ_FAILED", err.Error(), key)
	}
	
	return data, nil
}

// DownloadStream returns a stream for downloading from S3
func (s *S3Client) DownloadStream(key string) (io.ReadCloser, error) {
	result, err := s.client.GetObject(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return nil, NewStorageError("s3", "DOWNLOAD_STREAM_FAILED", err.Error(), key)
	}
	
	return result.Body, nil
}

// Delete deletes a file from S3
func (s *S3Client) Delete(key string) error {
	_, err := s.client.DeleteObject(&s3.DeleteObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return NewStorageError("s3", "DELETE_FAILED", err.Error(), key)
	}
	
	return nil
}

// Exists checks if a file exists in S3
func (s *S3Client) Exists(key string) (bool, error) {
	_, err := s.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		if strings.Contains(err.Error(), "NotFound") {
			return false, nil
		}
		return false, NewStorageError("s3", "HEAD_FAILED", err.Error(), key)
	}
	
	return true, nil
}

// GetSize gets the size of a file in S3
func (s *S3Client) GetSize(key string) (int64, error) {
	result, err := s.client.HeadObject(&s3.HeadObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return 0, NewStorageError("s3", "HEAD_FAILED", err.Error(), key)
	}
	
	return *result.ContentLength, nil
}

// GetURL gets the public URL for a file
func (s *S3Client) GetURL(key string) (string, error) {
	if s.provider.CDNUrl != "" {
		return fmt.Sprintf("%s/%s", strings.TrimRight(s.provider.CDNUrl, "/"), key), nil
	}
	
	return fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", s.bucket, s.region, key), nil
}

// GetPresignedURL generates a presigned URL for downloading
func (s *S3Client) GetPresignedURL(key string, expiry time.Duration) (string, error) {
	req, _ := s.client.GetObjectRequest(&s3.GetObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	url, err := req.Presign(expiry)
	if err != nil {
		return "", NewStorageError("s3", "PRESIGN_FAILED", err.Error(), key)
	}
	
	return url, nil
}

// GetPresignedUploadURL generates a presigned URL for uploading
func (s *S3Client) GetPresignedUploadURL(key string, expiry time.Duration, maxSize int64) (string, error) {
	req, _ := s.client.PutObjectRequest(&s3.PutObjectInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	url, err := req.Presign(expiry)
	if err != nil {
		return "", NewStorageError("s3", "PRESIGN_UPLOAD_FAILED", err.Error(), key)
	}
	
	return url, nil
}

// InitiateMultipartUpload starts a multipart upload
func (s *S3Client) InitiateMultipartUpload(key string) (*MultipartUpload, error) {
	result, err := s.client.CreateMultipartUpload(&s3.CreateMultipartUploadInput{
		Bucket: aws.String(s.bucket),
		Key:    aws.String(key),
	})
	
	if err != nil {
		return nil, NewStorageError("s3", "MULTIPART_INIT_FAILED", err.Error(), key)
	}
	
	return &MultipartUpload{
		UploadID: *result.UploadId,
		Key:      key,
		Provider: "s3",
	}, nil
}

// UploadPart uploads a part in multipart upload
func (s *S3Client) UploadPart(uploadID, key string, partNumber int, data []byte) (*UploadPart, error) {
	result, err := s.client.UploadPart(&s3.UploadPartInput{
		Bucket:     aws.String(s.bucket),
		Key:        aws.String(key),
		UploadId:   aws.String(uploadID),
		PartNumber: aws.Int64(int64(partNumber)),
		Body:       bytes.NewReader(data),
	})
	
	if err != nil {
		return nil, NewStorageError("s3", "MULTIPART_UPLOAD_FAILED", err.Error(), key)
	}
	
	return &UploadPart{
		PartNumber: partNumber,
		ETag:       strings.Trim(*result.ETag, "\""),
		Size:       int64(len(data)),
	}, nil
}

// CompleteMultipartUpload completes a multipart upload
func (s *S3Client) CompleteMultipartUpload(uploadID, key string, parts []UploadPart) error {
	completedParts := make([]*s3.CompletedPart, len(parts))
	
	for i, part := range parts {
		completedParts[i] = &s3.CompletedPart{
			ETag:       aws.String(part.ETag),
			PartNumber: aws.Int64(int64(part.PartNumber)),
		}
	}
	
	_, err := s.client.CompleteMultipartUpload(&s3.CompleteMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
		MultipartUpload: &s3.CompletedMultipartUpload{
			Parts: completedParts,
		},
	})
	
	if err != nil {
		return NewStorageError("s3", "MULTIPART_COMPLETE_FAILED", err.Error(), key)
	}
	
	return nil
}

// AbortMultipartUpload aborts a multipart upload
func (s *S3Client) AbortMultipartUpload(uploadID, key string) error {
	_, err := s.client.AbortMultipartUpload(&s3.AbortMultipartUploadInput{
		Bucket:   aws.String(s.bucket),
		Key:      aws.String(key),
		UploadId: aws.String(uploadID),
	})
	
	if err != nil {
		return NewStorageError("s3", "MULTIPART_ABORT_FAILED", err.Error(), key)
	}
	
	return nil
}

// DeleteMultiple deletes multiple files
func (s *S3Client) DeleteMultiple(keys []string) error {
	objects := make([]*s3.ObjectIdentifier, len(keys))
	for i, key := range keys {
		objects[i] = &s3.ObjectIdentifier{Key: aws.String(key)}
	}
	
	_, err := s.client.DeleteObjects(&s3.DeleteObjectsInput{
		Bucket: aws.String(s.bucket),
		Delete: &s3.Delete{Objects: objects},
	})
	
	if err != nil {
		return NewStorageError("s3", "BULK_DELETE_FAILED", err.Error(), "")
	}
	
	return nil
}

// CopyFile copies a file within S3
func (s *S3Client) CopyFile(sourceKey, destKey string) error {
	_, err := s.client.CopyObject(&s3.CopyObjectInput{
		Bucket:     aws.String(s.bucket),
		CopySource: aws.String(fmt.Sprintf("%s/%s", s.bucket, sourceKey)),
		Key:        aws.String(destKey),
	})
	
	if err != nil {
		return NewStorageError("s3", "COPY_FAILED", err.Error(), sourceKey)
	}
	
	return nil
}

// MoveFile moves a file within S3 (copy then delete)
func (s *S3Client) MoveFile(sourceKey, destKey string) error {
	if err := s.CopyFile(sourceKey, destKey); err != nil {
		return err
	}
	
	return s.Delete(sourceKey)
}

// GetProviderInfo returns provider information
func (s *S3Client) GetProviderInfo() *ProviderInfo {
	return &ProviderInfo{
		Name:        s.provider.Name,
		Type:        "s3",
		Region:      s.region,
		Endpoint:    s.provider.Endpoint,
		CDNUrl:      s.provider.CDNUrl,
		MaxFileSize: s.provider.MaxFileSize,
		Features:    []string{"multipart", "presigned", "cdn", "encryption"},
		Metadata: map[string]string{
			"bucket": s.bucket,
		},
	}
}

// HealthCheck checks if the S3 service is accessible
func (s *S3Client) HealthCheck() error {
	_, err := s.client.HeadBucket(&s3.HeadBucketInput{
		Bucket: aws.String(s.bucket),
	})
	
	if err != nil {
		return NewStorageError("s3", "HEALTH_CHECK_FAILED", err.Error(), "")
	}
	
	return nil
}

// GetStats returns storage statistics (not available in S3)
func (s *S3Client) GetStats() (*StorageStats, error) {
	// S3 doesn't provide direct stats, would need CloudWatch
	return &StorageStats{
		TotalFiles: -1, // Unknown
		TotalSize:  -1, // Unknown
		UsedSpace:  -1, // Unknown
	}, nil
}
