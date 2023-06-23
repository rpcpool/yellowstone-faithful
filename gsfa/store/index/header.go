package index

import (
	"encoding/json"
	"os"
)

// Header contains information about the index. This is actually stored in a
// separate ".info" file, but is the first file read when the index is opened.
type Header struct {
	// A version number in case we change the header
	Version int
	// The number of bits used to determine the in-memory buckets
	BucketsBits byte
	// MaxFileSize is the size limit of each index file. This cannot be greater
	// than 4GiB.
	MaxFileSize uint32
	// First index file number
	FirstFile uint32
	// PrimaryFileSize is the primary's maximum size, if applicable.
	PrimaryFileSize uint32
}

func newHeader(bucketsBits byte, maxFileSize uint32) Header {
	return Header{
		Version:     IndexVersion,
		BucketsBits: bucketsBits,
		MaxFileSize: maxFileSize,
	}
}

func readHeader(filePath string) (Header, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return Header{}, err
	}

	var header Header
	err = json.Unmarshal(data, &header)
	if err != nil {
		return Header{}, err
	}

	return header, nil
}

func writeHeader(headerPath string, header Header) error {
	data, err := json.Marshal(&header)
	if err != nil {
		return err
	}

	return os.WriteFile(headerPath, data, 0o666)
}
