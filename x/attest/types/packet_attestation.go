package types

// GetBytes is a helper for serialising
func (p AttestationPacketData) GetBytes() ([]byte, error) {
	var modulePacket AttestPacketData

	modulePacket.Packet = &AttestPacketData_AttestationPacket{&p}

	return modulePacket.Marshal()
}
