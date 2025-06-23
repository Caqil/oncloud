package storage

import (
	"oncloud/models"
)

// WasabiClient implements StorageInterface for Wasabi (S3-compatible)
type WasabiClient struct {
	*S3Client
}

// NewWasabiClient creates a new Wasabi client
func NewWasabiClient(provider *models.StorageProvider) (*WasabiClient, error) {
	// Set Wasabi-specific endpoint if not provided
	if provider.Endpoint == "" {
		provider.Endpoint = getWasabiEndpoint(provider.Region)
	}

	s3Client, err := NewS3Client(provider)
	if err != nil {
		return nil, err
	}

	return &WasabiClient{
		S3Client: s3Client,
	}, nil
}

// getWasabiEndpoint returns the appropriate Wasabi endpoint for a region
func getWasabiEndpoint(region string) string {
	endpoints := map[string]string{
		"us-east-1":      "https://s3.wasabisys.com",
		"us-east-2":      "https://s3.us-east-2.wasabisys.com",
		"us-west-1":      "https://s3.us-west-1.wasabisys.com",
		"eu-central-1":   "https://s3.eu-central-1.wasabisys.com",
		"eu-west-1":      "https://s3.eu-west-1.wasabisys.com",
		"eu-west-2":      "https://s3.eu-west-2.wasabisys.com",
		"ap-northeast-1": "https://s3.ap-northeast-1.wasabisys.com",
		"ap-northeast-2": "https://s3.ap-northeast-2.wasabisys.com",
		"ap-southeast-1": "https://s3.ap-southeast-1.wasabisys.com",
		"ap-southeast-2": "https://s3.ap-southeast-2.wasabisys.com",
	}

	if endpoint, exists := endpoints[region]; exists {
		return endpoint
	}

	// Default to US East 1
	return "https://s3.wasabisys.com"
}

// GetURL gets the public URL for a file (Wasabi-specific)
func (w *WasabiClient) GetURL(key string) (string, error) {
	if w.provider.CDNUrl != "" {
		return w.S3Client.GetURL(key)
	}

	endpoint := getWasabiEndpoint(w.region)
	return endpoint + "/" + w.bucket + "/" + key, nil
}

// GetProviderInfo returns Wasabi-specific provider information
func (w *WasabiClient) GetProviderInfo() *ProviderInfo {
	info := w.S3Client.GetProviderInfo()
	info.Type = "wasabi"
	info.Features = append(info.Features, "cost-effective", "no-egress-fees")
	return info
}
