package azure

import (
	"github.com/aquilax/truncate"
	sanitize "github.com/mrz1836/go-sanitize"
)

// getStorageAccountName returns the sanitized bucket name if already computed or compute it and return it
func getStorageAccountName(bucketName string) string {
	return sanitizeAlphanumeric24(bucketName)
}

// sanitizeAlphanumeric24 returns the name following Azure rules (alphanumerical characters only + 24 characters MAX)
// more details https://learn.microsoft.com/en-us/rest/api/storagerp/storage-accounts/get-properties?view=rest-storagerp-2023-01-01&tabs=HTTP#uri-parameters
func sanitizeAlphanumeric24(name string) string {
	return truncate.Truncate(sanitize.AlphaNumeric(name, false), 24, "", truncate.PositionEnd)
}
