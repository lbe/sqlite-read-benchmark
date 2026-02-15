package main

import (
	"crypto/rand"
	"math/big"
)

func RandInt(n int64) int64 {
	val, _ := rand.Int(rand.Reader, big.NewInt(n))
	return val.Int64()
}

func ShuffleIDs(ids []int64) {
	for i := len(ids) - 1; i > 0; i-- {
		j := int(RandInt(int64(i + 1)))
		ids[i], ids[j] = ids[j], ids[i]
	}
}
