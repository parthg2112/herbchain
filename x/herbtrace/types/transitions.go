package types

import errorsmod "cosmossdk.io/errors"

// IsValidStatus reports whether s is a concrete lifecycle status (not UNSPECIFIED or unknown).
func IsValidStatus(s BatchStatus) bool {
	return s >= StatusHarvested && s <= StatusRejected
}

// IsTerminal reports whether no further transitions are allowed from s.
func (s BatchStatus) IsTerminal() bool {
	return s == StatusDelivered || s == StatusRejected
}

// IsOwnerDrivenStatus reports whether the batch owner may move a batch into s via MsgUpdateStatus.
func IsOwnerDrivenStatus(s BatchStatus) bool {
	switch s {
	case StatusProcessed, StatusPackaged, StatusShipped, StatusDelivered:
		return true
	default:
		return false
	}
}

// IsLabStatus reports whether s is only reachable via MsgRecordLabResult.
func IsLabStatus(s BatchStatus) bool {
	return s == StatusTested || s == StatusRejected
}

// ValidateTransition checks that to is a legal single step from from, regardless of who drives it.
// Callers enforce the actor split with IsOwnerDrivenStatus / IsLabStatus.
func ValidateTransition(from, to BatchStatus) error {
	if !IsValidStatus(from) {
		return errorsmod.Wrapf(ErrInvalidTransition, "unknown current status %s", from)
	}
	if from.IsTerminal() {
		return errorsmod.Wrapf(ErrTerminalStatus, "%s is terminal", from)
	}
	if !IsValidStatus(to) {
		return errorsmod.Wrapf(ErrInvalidTransition, "unknown target status %s", to)
	}
	if !isNextStatus(from, to) {
		return errorsmod.Wrapf(ErrInvalidTransition, "%s -> %s", from, to)
	}
	return nil
}

func isNextStatus(from, to BatchStatus) bool {
	switch from {
	case StatusHarvested:
		return to == StatusProcessed
	case StatusProcessed:
		return to == StatusTested || to == StatusRejected
	case StatusTested:
		return to == StatusPackaged
	case StatusPackaged:
		return to == StatusShipped
	case StatusShipped:
		return to == StatusDelivered
	default:
		return false
	}
}
