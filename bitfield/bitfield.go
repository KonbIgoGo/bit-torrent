package bitfield

type Bitfield []byte

func (b Bitfield) HasPiece(idx int) bool {
	byteIndex := idx / 8
	offset := idx % 8
	return b[byteIndex]>>(7-offset)&1 != 0
}

func (b Bitfield) SetPiece(idx int) {
	byteIndex := idx / 8
	offset := idx % 8
	b[byteIndex] |= 1 << (7 - offset)
}
