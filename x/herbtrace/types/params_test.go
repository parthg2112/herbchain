package types_test

import (
	"strings"
	"testing"

	sdk "github.com/cosmos/cosmos-sdk/types"
	"github.com/stretchr/testify/require"

	"github.com/parthg2112/herbchain/x/herbtrace/types"
)

func testAddr(seed string) string {
	return sdk.AccAddress([]byte(seed + strings.Repeat("_", 20-len(seed)))).String()
}

func TestParamsValidate(t *testing.T) {
	certifier := testAddr("carol")
	manyCertifiers := make([]string, types.MaxCertifiers+1)
	for i := range manyCertifiers {
		manyCertifiers[i] = sdk.AccAddress([]byte{byte(i), byte(i >> 8), 1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16, 17, 18}).String()
	}

	tests := []struct {
		name   string
		params types.Params
		valid  bool
	}{
		{"default", types.DefaultParams(), true},
		{"with certifier", types.NewParams([]string{certifier}, 5), true},
		{"max certifications zero", types.NewParams(nil, 0), false},
		{"max certifications above limit", types.NewParams(nil, types.MaxCertificationsLimit+1), false},
		{"invalid certifier", types.NewParams([]string{"not-an-address"}, 10), false},
		{"empty certifier", types.NewParams([]string{""}, 10), false},
		{"non-canonical certifier", types.NewParams([]string{strings.ToUpper(certifier)}, 10), false},
		{"duplicate certifier", types.NewParams([]string{certifier, certifier}, 10), false},
		{"too many certifiers", types.NewParams(manyCertifiers, 10), false},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := tc.params.Validate()
			if tc.valid {
				require.NoError(t, err)
				return
			}
			require.ErrorIs(t, err, types.ErrInvalidParams)
		})
	}
	require.Equal(t, uint32(10), types.DefaultParams().MaxCertifications)
	require.Empty(t, types.DefaultParams().Certifiers)
}
