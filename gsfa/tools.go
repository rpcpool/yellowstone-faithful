package gsfa

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/gagliardetto/solana-go"
)

func encodeUvarint(n uint64) []byte {
	buf := make([]byte, binary.MaxVarintLen64)
	written := binary.PutUvarint(buf, n)
	return buf[:written]
}

func encodeDeltaVarint(slice []uint64) []byte {
	var buffer bytes.Buffer
	var prev uint64 = 0
	for _, num := range slice {
		delta := num - prev
		buffer.Write(encodeUvarint(delta))
		prev = num
	}
	return buffer.Bytes()
}

func decodeDeltaVarint(encoded []byte) []uint64 {
	var nums []uint64
	var prev uint64 = 0
	for len(encoded) > 0 {
		delta, read := binary.Uvarint(encoded)
		if read <= 0 {
			panic("error decoding")
		}
		num := prev + delta
		nums = append(nums, num)
		prev = num
		encoded = encoded[read:]
	}
	return nums
}

func equalUint64Slices(a, b []uint64) bool {
	if len(a) != len(b) {
		return false
	}
	for i, num := range a {
		if num != b[i] {
			return false
		}
	}
	return true
}

func newRandomUint64(max uint64) uint64 {
	buf := make([]byte, 8)
	_, err := rand.Read(buf)
	if err != nil {
		panic(err)
	}
	return binary.LittleEndian.Uint64(buf) % max
}

func newRandomUint64Slice(size int, max uint64) []uint64 {
	slice := make([]uint64, size)
	// the slice must contain psitive integers in sorted order
	for i := 0; i < size; i++ {
		slice[i] = newRandomUint64(max)
	}
	sort.Slice(slice, func(i, j int) bool {
		return slice[i] < slice[j]
	})
	return slice
}

func uint64SliceToBytes(slice []uint64) []byte {
	buf := make([]byte, len(slice)*8)
	for i, num := range slice {
		binary.LittleEndian.PutUint64(buf[i*8:], num)
	}
	return buf
}

func uint64SliceFromBytes(buf []byte) []uint64 {
	if len(buf)%8 != 0 {
		panic("invalid length")
	}
	slice := make([]uint64, len(buf)/8)
	for i := 0; i < len(slice); i++ {
		slice[i] = binary.LittleEndian.Uint64(buf[i*8:])
	}
	return slice
}

func printCompressionRatio(original, compressed int) {
	fmt.Println("original size:", (uint64(original)))
	fmt.Println("compressed size:", (uint64(compressed)))
	fmt.Println("compression ratio:", float64(original)/float64(compressed))
}

func getRandomUint64WithMax(max uint64) uint64 {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(max)))
	if err != nil {
		panic(err)
	}
	return v.Uint64()
}

func generateRandomPubkeyWithinMax(max uint64) solana.PublicKey {
	var pk solana.PublicKey
	pkBytes := uint64ToBytes(getRandomUint64WithMax(max))
	copy(pk[:8], pkBytes)
	copy(pk[8:16], pkBytes)
	copy(pk[16:24], pkBytes)
	copy(pk[24:32], pkBytes)
	return pk
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}

func genRandomKeys(max uint64, n uint64) []solana.PublicKey {
	var keys []solana.PublicKey
	for i := uint64(0); i < n; i++ {
		keys = append(keys, generateRandomPubkeyWithinMax(max))
	}
	return keys
}

func genRandomNumberBetween(min, max uint64) uint64 {
	v, err := rand.Int(rand.Reader, big.NewInt(int64(max-min)))
	if err != nil {
		panic(err)
	}
	return min + v.Uint64()
}

func generateRandomSignature() solana.Signature {
	var sig solana.Signature
	rand.Read(sig[:])
	return sig
}

func sortByKey(keysToLocs [][48]byte) {
	// Sort keysToLocs by key, which is the first 32 bytes of each key.
	sort.Slice(keysToLocs, func(i, j int) bool {
		return bytes.Compare(keysToLocs[i][:32], keysToLocs[j][:32]) < 0
	})
}

func toBase64(b []byte) string {
	return strings.TrimRight(
		strings.TrimRight(
			base64.URLEncoding.EncodeToString(b),
			"=",
		),
		"=",
	)
}

func splitKey(hashedKey [32]byte) [][2]byte {
	chunks := make([][2]byte, 0)
	for i := 0; i < 32; i += 2 {
		chunks = append(chunks, [2]byte{hashedKey[i], hashedKey[i+1]})
	}
	return chunks
}

func hashKey(key []byte) [32]byte {
	hasher := sha256.New()
	hasher.Write(key)
	return [32]byte(hasher.Sum(nil))
}

var bufferPool = sync.Pool{
	New: func() interface{} {
		return [32]byte{}
	},
}

func newBuffer() [32]byte {
	return bufferPool.Get().([32]byte)
}

func putBuffer(b [32]byte) {
	bufferPool.Put(b)
}

func newRandomKey() [32]byte {
	key := newBuffer()
	rand.Read(key[:])
	return key
}

func getDirSize(path string) (uint64, error) {
	var size uint64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += uint64(info.Size())
		}
		return nil
	})
	return size, err
}
