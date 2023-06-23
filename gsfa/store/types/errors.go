package types

import "fmt"

type errorType string

func (e errorType) Error() string {
	return string(e)
}

// ErrOutOfBounds indicates the bucket index was greater than the number of bucks
const ErrOutOfBounds = errorType("Buckets out of bound error")

// ErrIndexTooLarge indicates the maximum supported bucket size is 32-bits
const ErrIndexTooLarge = errorType("Index size cannot be more than 32-bits")

const ErrKeyTooShort = errorType("Key must be at least 4 bytes long")

const ErrKeyExists = errorType("key exists")

type ErrIndexWrongBitSize [2]byte

func (e ErrIndexWrongBitSize) Error() string {
	return fmt.Sprintf("Index bit size for buckets is %d, expected %d", e[0], e[1])
}

type ErrIndexWrongFileSize [2]uint32

func (e ErrIndexWrongFileSize) Error() string {
	return fmt.Sprintf("Index file size limit is %d, expected %d", e[0], e[1])
}

type ErrPrimaryWrongFileSize [2]uint32

func (e ErrPrimaryWrongFileSize) Error() string {
	return fmt.Sprintf("Primary file size limit is %d, expected %d", e[0], e[1])
}
