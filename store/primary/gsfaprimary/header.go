package gsfaprimary

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"encoding/json"
	"os"
)

// Header contains information about the primary. This is actually stored in a
// separate ".info" file, but is the first file read when the index is opened.
type Header struct {
	// A version number in case we change the header
	Version int
	// MaxFileSize is the size limit of each index file. This cannot be greater
	// than 4GiB.
	MaxFileSize uint32
	// First index file number
	FirstFile uint32
}

func newHeader(maxFileSize uint32) Header {
	return Header{
		Version:     PrimaryVersion,
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
