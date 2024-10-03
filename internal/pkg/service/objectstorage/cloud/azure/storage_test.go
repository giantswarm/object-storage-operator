package azure

import (
	"strconv"
	"testing"

	"github.com/google/go-cmp/cmp"
)

func Test_sanitizeAlphanumeric24(t *testing.T) {
	testCases := []struct {
		name           string
		inputString    string
		expectedString string
	}{
		{
			name:           "case 0: 'giantswarm-glippy-loki' sanitized",
			inputString:    "giantswarm-glippy-loki",
			expectedString: "giantswarmglippyloki",
		},
		{
			name:           "case 1: 'giantswarm-verylonginstallationname-loki' sanitized",
			inputString:    "giantswarm-verylonginstallationname-loki",
			expectedString: "giantswarmverylonginstal",
		},
		{
			name:           "case 2: 'giantswarm-1111-loki' sanitized",
			inputString:    "giantswarm-1111-loki",
			expectedString: "giantswarm1111loki",
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			storageAccountName := sanitizeAlphanumeric24(tc.inputString)

			if !cmp.Equal(storageAccountName, tc.expectedString) {
				t.Fatalf("\n\n%s\n", cmp.Diff(tc.expectedString, storageAccountName))
			}
		})
	}
}

func Test_getStorageAccountName(t *testing.T) {
	testCases := []struct {
		name              string
		bucketName        string
		listStorageNames  []string
		expectedResult    string
		expectedListNames []string
	}{
		{
			name:              "case 0: bucket name exists in list",
			bucketName:        "giantswarm-glippy-loki",
			listStorageNames:  []string{"giantswarmglippyloki", "giantswarmverylonginstal"},
			expectedResult:    "giantswarmglippyloki",
			expectedListNames: []string{"giantswarmglippyloki", "giantswarmverylonginstal"},
		},
		{
			name:              "case 1: bucket name does not exist in list",
			bucketName:        "giantswarm-verylonginstallationname-loki",
			listStorageNames:  []string{"giantswarmglippyloki"},
			expectedResult:    "giantswarmverylonginstal",
			expectedListNames: []string{"giantswarmglippyloki", "giantswarmverylonginstal"},
		},
		{
			name:              "case 2: empty list",
			bucketName:        "giantswarm-1111-loki",
			listStorageNames:  []string{},
			expectedResult:    "giantswarm1111loki",
			expectedListNames: []string{"giantswarm1111loki"},
		},
	}

	for i, tc := range testCases {
		t.Run(strconv.Itoa(i), func(t *testing.T) {
			t.Log(tc.name)

			adapter := &AzureObjectStorageAdapter{
				listStorageAccountName: tc.listStorageNames,
			}

			result := adapter.getStorageAccountName(tc.bucketName)

			if result != tc.expectedResult {
				t.Fatalf("Expected result: %s, but got: %s", tc.expectedResult, result)
			}

			if !cmp.Equal(adapter.listStorageAccountName, tc.expectedListNames) {
				t.Fatalf("\n\n%s\n", cmp.Diff(tc.expectedListNames, adapter.listStorageAccountName))
			}
		})
	}
}
