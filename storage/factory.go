package storage

import (
	"fmt"
	"oncloud/models"
)

// NewStorageClient creates a new storage client based on provider type
func NewStorageClient(provider *models.StorageProvider) (StorageInterface, error) {
	switch provider.Type {
	case "s3":
		return NewS3Client(provider)
	case "wasabi":
		return NewWasabiClient(provider)
	case "r2":
		return NewR2Client(provider)
	default:
		return nil, fmt.Errorf("unsupported storage provider type: %s", provider.Type)
	}
}

// ValidateProvider validates storage provider configuration
func ValidateProvider(provider *models.StorageProvider) error {
	if provider.Name == "" {
		return fmt.Errorf("provider name is required")
	}

	if provider.Bucket == "" {
		return fmt.Errorf("bucket name is required")
	}

	switch provider.Type {
	case "s3":
		return validateS3Provider(provider)
	case "wasabi":
		return validateWasabiProvider(provider)
	case "r2":
		return validateR2Provider(provider)
	default:
		return fmt.Errorf("unsupported provider type: %s", provider.Type)
	}
}

func validateS3Provider(provider *models.StorageProvider) error {
	if provider.AccessKey == "" || provider.SecretKey == "" {
		return fmt.Errorf("AWS access key and secret key are required")
	}

	if provider.Region == "" {
		return fmt.Errorf("AWS region is required")
	}

	return nil
}

func validateWasabiProvider(provider *models.StorageProvider) error {
	if provider.AccessKey == "" || provider.SecretKey == "" {
		return fmt.Errorf("Wasabi access key and secret key are required")
	}

	if provider.Region == "" {
		return fmt.Errorf("Wasabi region is required")
	}

	return nil
}

func validateR2Provider(provider *models.StorageProvider) error {
	if provider.AccessKey == "" || provider.SecretKey == "" {
		return fmt.Errorf("R2 access key and secret key are required")
	}

	accountID, ok := provider.Settings["account_id"].(string)
	if !ok || accountID == "" {
		return fmt.Errorf("R2 account ID is required in settings")
	}

	return nil
}
