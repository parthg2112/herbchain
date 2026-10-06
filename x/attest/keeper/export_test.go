package keeper

import "github.com/parthg2112/herbchain/x/attest/types"

// WithPacketSender returns a copy of k that sends packets through ps instead of the IBC keeper.
func (k Keeper) WithPacketSender(ps types.PacketSender) Keeper {
	k.packetSender = ps
	return k
}
