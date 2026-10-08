package domain

import (
	"crypto/rand"
	"time"
)

const crockford = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"

// ULID returns a 48-bit millisecond time plus 80 random bits in Crockford base32 (26 chars),
// so identifiers sort by creation time.
func ULID(now time.Time, random [10]byte) string {
	var out [26]byte
	t := uint64(now.UnixMilli())
	for i := 9; i >= 0; i-- {
		out[i] = crockford[t%32]
		t /= 32
	}
	// 80 bits = 16 base32 digits, most significant first.
	var hi uint16 = uint16(random[0])<<8 | uint16(random[1])
	var lo uint64
	for _, b := range random[2:] {
		lo = lo<<8 | uint64(b)
	}
	for i := 25; i >= 10; i-- {
		out[i] = crockford[lo&31]
		lo = lo>>5 | uint64(hi&31)<<59
		hi >>= 5
	}
	return string(out[:])
}

func newULID() string {
	var r [10]byte
	if _, err := rand.Read(r[:]); err != nil {
		panic(err) // crypto/rand never fails on supported platforms
	}
	return ULID(time.Now(), r)
}

// NewTaskID, NewRunID and NewHandoffID return prefixed ULIDs.
func NewTaskID() string    { return "tsk_" + newULID() }
func NewRunID() string     { return "run_" + newULID() }
func NewHandoffID() string { return "ho_" + newULID() }
