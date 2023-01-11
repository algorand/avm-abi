package address

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestAddress(t *testing.T) {
	t.Parallel()

	t.Run("valid", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			addressString   string
			addressBytes    [32]byte
			addressChecksum [4]byte
		}{
			{
				addressString:   "CAFFDSU6TYXNDC6V6R5XAOHBWBD4MH36TNUWCW4D6HKV7EKHP33Q74JAFM",
				addressBytes:    [32]byte{16, 10, 81, 202, 158, 158, 46, 209, 139, 213, 244, 123, 112, 56, 225, 176, 71, 198, 31, 126, 155, 105, 97, 91, 131, 241, 213, 95, 145, 71, 126, 247},
				addressChecksum: [4]byte{15, 241, 32, 43},
			},
			{
				addressString:   "OXV2VEY7QJUXGOHEVFSL7LTBMOTYI4VORBJ37CGCHKBPJSH6IZQMHDPFRA",
				addressBytes:    [32]byte{117, 235, 170, 147, 31, 130, 105, 115, 56, 228, 169, 100, 191, 174, 97, 99, 167, 132, 114, 174, 136, 83, 191, 136, 194, 58, 130, 244, 200, 254, 70, 96},
				addressChecksum: [4]byte{195, 141, 229, 136},
			},
			{
				addressString:   "BFLADE6DETJQU7DABJANLCKH3PEFTWQQGZ34YHRPRKWYNEYC3DRWKJWZZA",
				addressBytes:    [32]byte{9, 86, 1, 147, 195, 36, 211, 10, 124, 96, 10, 64, 213, 137, 71, 219, 200, 89, 218, 16, 54, 119, 204, 30, 47, 138, 173, 134, 147, 2, 216, 227},
				addressChecksum: [4]byte{101, 38, 217, 200},
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.addressString, func(t *testing.T) {
				require.Equal(t, testCase.addressChecksum[:], Checksum(testCase.addressBytes))
				require.Equal(t, testCase.addressString, ToString(testCase.addressBytes))

				actualBytes, err := FromString(testCase.addressString)
				require.NoError(t, err)
				require.Equal(t, testCase.addressBytes, actualBytes)
			})
		}
	})

	t.Run("invalid", func(t *testing.T) {
		t.Parallel()
		testCases := []struct {
			addressString string
			expectedError string
		}{
			{
				// incorrect checksum
				addressString: "CAFFDSU6TYXNDC6V6R5XAOHBWBD4MH36TNUWCW4D6HKV7EKHP33Q74JAQM",
				expectedError: "decoded checksum mismatch",
			},
			{
				// too many bytes
				addressString: "CAFFDSU6TYXNDC6V6R5XAOHBWBD4MH36TNUWCW4D6HKV7EKHP33Q74JAFMFM",
				expectedError: "decoded byte length should equal 36 with address and checksum",
			},
			{
				// too few bytes
				addressString: "CAFFDSU6TYXNDC6V6R5XAOHBWBD4MH36TNUWCW4D6HKV7EKHP33Q74JA",
				expectedError: "decoded byte length should equal 36 with address and checksum",
			},
			{
				// not base32
				addressString: "!!!",
				expectedError: "base32 decode error",
			},
		}

		for _, testCase := range testCases {
			t.Run(testCase.addressString, func(t *testing.T) {
				_, err := FromString(testCase.addressString)
				require.ErrorContains(t, err, testCase.expectedError)
			})
		}
	})
}
