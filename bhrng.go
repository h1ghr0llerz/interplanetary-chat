package main

import (
        "crypto/elliptic"
        "crypto/rand"
		"encoding/binary"
        "math/big"
        "sync"
       )


type bh_rng struct {
    curve   elliptic.Curve
    Px, Py   *big.Int
    Qx, Qy   *big.Int
    state   *big.Int
	byte_buffer []byte
    byte_buffer_lock sync.Mutex
}

func (bh *bh_rng) OutputSize() (n int) {
	n = bh.curve.Params().BitSize / 8
	return
}

// Implement rand source interface
func (bh *bh_rng) Uint64() (v uint64) {
	binary.Read(bh, binary.BigEndian, &v)
	return
}

func (bh *bh_rng) Int63() int64 {
	return int64(bh.Uint64() & ^uint64(1<<63))
}
func (bh *bh_rng) Seed(seed int64) {
    bh.state = big.NewInt(seed)
}

func (bh *bh_rng) Read(p []byte) (n int, err error) { // impl Reader inteface
    bh.byte_buffer_lock.Lock()
	// read bytes into buffer until we have enough
	for (len(bh.byte_buffer) < len(p)) {
		bh.byte_buffer = append(bh.byte_buffer, bh.Next()...)
	}
	n = copy(p, bh.byte_buffer)
	bh.byte_buffer = bh.byte_buffer[n:]
    bh.byte_buffer_lock.Unlock()
	err = nil
	return
}

func (bh *bh_rng) Next() []byte {
    sPx, _ := bh.curve.ScalarMult(bh.Px, bh.Py, bh.state.Bytes())

    bh.state = sPx
    rQx, _ := bh.curve.ScalarMult(bh.Qx, bh.Qy, bh.state.Bytes())

    next := new(big.Int).Set(rQx)
    return next.Bytes()
}

func NewBHRNG() *bh_rng {
	curve := elliptic.P256()

	seed := make([]byte, 32)
	rand.Read(seed)

	Qx := new(big.Int)
	Qx.UnmarshalText([]byte("30046258942025479478968106289094232627227023358340390702609987405752326700508"))
	Qy := new(big.Int)
	Qy.UnmarshalText([]byte("14875206515834225192648645391355104652599352857531953045993041941027863219610"))
    return &bh_rng {
        curve: curve,
        Px: curve.Params().Gx,
        Py: curve.Params().Gy,
        Qx: Qx,
        Qy: Qy,
        state: new(big.Int).SetBytes(seed),
    }
}
