/*
Package address provides the ability to convert between 32 byte Algorand addresses and their base32
string form.
*/
package address

import (
	"bytes"
	"crypto/sha512"
	"encoding/base32"
	"fmt"
)

// BytesSize is the size of an Algorand address in bytes. This is NOT the size of the base32 string
// form of an Algorand address.
const BytesSize = 32
const checksumBytesSize = 4

var base32Encoder = base32.StdEncoding.WithPadding(base32.NoPadding)

// Checksum computes the address checksum
func Checksum(addressBytes [BytesSize]byte) []byte {
	hashed := sha512.Sum512_256(addressBytes[:])
	return hashed[BytesSize-checksumBytesSize:]
}

// ToString converts a 32 byte Algorand address to a string
func ToString(addressBytes [BytesSize]byte) string {
	checksum := Checksum(addressBytes)

	var addressBytesAndChecksum [BytesSize + checksumBytesSize]byte
	copy(addressBytesAndChecksum[:], addressBytes[:])
	copy(addressBytesAndChecksum[BytesSize:], checksum)

	return base32Encoder.EncodeToString(addressBytesAndChecksum[:])
}

// FromString converts a string to a 32 byte Algorand address
func FromString(addressString string) ([BytesSize]byte, error) {
	decoded, err := base32Encoder.DecodeString(addressString)
	if err != nil {
		return [BytesSize]byte{},
			fmt.Errorf("cannot cast encoded address string (%s) to address: base32 decode error: %w", addressString, err)
	}
	if len(decoded) != BytesSize+checksumBytesSize {
		return [BytesSize]byte{},
			fmt.Errorf(
				"cannot cast encoded address string (%s) to address: "+
					"decoded byte length should equal %d with address and checksum",
				addressString, BytesSize+checksumBytesSize,
			)
	}
	var addressBytes [BytesSize]byte
	copy(addressBytes[:], decoded[:])

	checksum := Checksum(addressBytes)
	if !bytes.Equal(checksum, decoded[BytesSize:]) {
		return [BytesSize]byte{}, fmt.Errorf(
			"cannot cast encoded address string (%s) to address: decoded checksum mismatch, %v != %v",
			addressString, checksum, decoded[BytesSize:],
		)
	}

	return addressBytes, nil
}
