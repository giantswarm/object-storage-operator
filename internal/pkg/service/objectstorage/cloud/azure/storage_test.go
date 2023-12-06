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
