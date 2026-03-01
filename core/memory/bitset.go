package memory

// Bitset is a compact representation of slot usage in a memory bucket.
type Bitset struct {
	bits []uint64
}

// NewBitset creates a new Bitset with the given number of bits.
func NewBitset(size int) *Bitset {
	return &Bitset{bits: make([]uint64, (size+63)/64)}
}

// Set sets the bit at the given index to value (0 or 1).
func (b *Bitset) Set(idx int, value int) {
	if value != 0 {
		b.bits[idx/64] |= 1 << (idx % 64)
	} else {
		b.bits[idx/64] &^= 1 << (idx % 64)
	}
}

// IsSet returns true if the bit at the given index is set.
func (b *Bitset) IsSet(idx int) bool {
	return (b.bits[idx/64] & (1 << (idx % 64))) != 0
}
