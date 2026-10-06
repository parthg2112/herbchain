package types

import (
	errorsmod "cosmossdk.io/errors"
	sdk "github.com/cosmos/cosmos-sdk/types"
)

const (
	// DefaultMaxCertifications is the default per-batch certification cap.
	DefaultMaxCertifications uint32 = 10
	// MaxCertificationsLimit is the hard ceiling for params.max_certifications (brief §3).
	MaxCertificationsLimit uint32 = 10
	// MaxCertifiers bounds the certifier list scanned on every MsgRecordLabResult.
	MaxCertifiers = 100
)

// NewParams creates a new Params instance.
func NewParams(certifiers []string, maxCertifications uint32) Params {
	return Params{Certifiers: certifiers, MaxCertifications: maxCertifications}
}

// DefaultParams returns a default set of parameters.
func DefaultParams() Params {
	return NewParams(nil, DefaultMaxCertifications)
}

// Validate validates the set of params.
func (p Params) Validate() error {
	if p.MaxCertifications == 0 || p.MaxCertifications > MaxCertificationsLimit {
		return errorsmod.Wrapf(ErrInvalidParams, "max_certifications %d must be in [1, %d]",
			p.MaxCertifications, MaxCertificationsLimit)
	}
	if len(p.Certifiers) > MaxCertifiers {
		return errorsmod.Wrapf(ErrInvalidParams, "%d certifiers exceeds limit %d", len(p.Certifiers), MaxCertifiers)
	}
	seen := make(map[string]struct{}, len(p.Certifiers))
	for _, c := range p.Certifiers {
		if err := ValidateCanonicalAddress(c); err != nil {
			return errorsmod.Wrapf(ErrInvalidParams, "certifier: %s", err)
		}
		if _, dup := seen[c]; dup {
			return errorsmod.Wrapf(ErrInvalidParams, "duplicate certifier %s", c)
		}
		seen[c] = struct{}{}
	}
	return nil
}

// ValidateCanonicalAddress requires s to be a bech32 account address in canonical (lowercase) form,
// so string equality on stored addresses is equivalent to byte equality.
func ValidateCanonicalAddress(s string) error {
	addr, err := sdk.AccAddressFromBech32(s)
	if err != nil {
		return errorsmod.Wrapf(ErrInvalidAddress, "%q: %s", s, err)
	}
	if addr.String() != s {
		return errorsmod.Wrapf(ErrInvalidAddress, "%q is not in canonical form", s)
	}
	return nil
}
