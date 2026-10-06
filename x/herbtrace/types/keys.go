package types

import "cosmossdk.io/collections"

const (
	// ModuleName defines the module name
	ModuleName = "herbtrace"

	// StoreKey defines the primary module store key
	StoreKey = ModuleName

	// GovModuleName duplicates the gov module's name to avoid a dependency with x/gov.
	GovModuleName = "gov"
)

// Store prefixes. Single-byte prefixes cannot collide with ParamsKey ("p_...").
var (
	ParamsKey      = collections.NewPrefix("p_herbtrace")
	BatchesKey     = collections.NewPrefix(1)
	HistoryKey     = collections.NewPrefix(2)
	HistorySeqKey  = collections.NewPrefix(3)
	OwnerIndexKey  = collections.NewPrefix(4)
	StatusIndexKey = collections.NewPrefix(5)
)
