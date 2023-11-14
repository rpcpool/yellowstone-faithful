package compactindexsized

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindexsized` to avoid conflicts with the original package
// - The values it indexes are N-byte values instead of 8-byte values. This allows to index CIDs (in particular sha256+CBOR CIDs), and other values, directly.

import (
	"context"
	"encoding/binary"
	"errors"
	"io"
	"io/fs"
	"math/rand"
	"os"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbauerster/mpb/v8/decor"
)

var testValues48 = [][]byte{
	{0xcc, 0x0a, 0xd4, 0x66, 0x32, 0x50, 0xc3, 0x96, 0x8b, 0x5c, 0x77, 0x7e, 0xb8, 0xfd, 0x9c, 0x78, 0xea, 0xfb, 0xd3, 0x4f, 0x1a, 0x59, 0x4e, 0xda, 0x1d, 0x90, 0x2a, 0xcd, 0x79, 0xb6, 0x0b, 0x2d, 0xea, 0x76, 0x36, 0x54, 0x65, 0xe6, 0x53, 0x1b, 0x70, 0x38, 0x84, 0xb2, 0xbf, 0x5d, 0xf9, 0x30},
	{0x7c, 0x18, 0x51, 0xd7, 0x63, 0x83, 0xf9, 0xc5, 0xaa, 0x48, 0x3c, 0x8e, 0xff, 0xf0, 0xf1, 0xab, 0xee, 0xda, 0xb0, 0x2f, 0x92, 0xcc, 0xb8, 0x78, 0x11, 0x5b, 0xa0, 0xb9, 0xfa, 0xf5, 0x2e, 0xb4, 0xd7, 0x10, 0x2d, 0x7b, 0xe5, 0xb6, 0x9f, 0xd0, 0xb1, 0xff, 0xd0, 0xf2, 0xef, 0xcd, 0x72, 0x1a},
	{0x0b, 0x2f, 0xc2, 0x4d, 0xc5, 0x98, 0x8b, 0x13, 0xd9, 0x17, 0xf8, 0xc1, 0xb8, 0x59, 0xd4, 0x24, 0xad, 0xef, 0xe5, 0xb6, 0xb8, 0xb9, 0xba, 0x01, 0x9c, 0xe0, 0x7f, 0x96, 0x25, 0x83, 0xd6, 0xbf, 0xa3, 0xb2, 0xf2, 0x29, 0xb9, 0xa1, 0xa1, 0x92, 0xd0, 0xc0, 0xe5, 0x06, 0x94, 0xea, 0x6c, 0xb3},
	{0xbb, 0x12, 0x08, 0x5f, 0x73, 0xee, 0x39, 0x69, 0x9f, 0x6e, 0x5a, 0xd8, 0x21, 0x2d, 0x43, 0xbe, 0x01, 0xc1, 0x3f, 0xc5, 0xfa, 0x86, 0x09, 0x7e, 0x97, 0x61, 0x59, 0xb8, 0xc9, 0x16, 0x47, 0xe3, 0x18, 0xfe, 0x52, 0x1e, 0xa2, 0x98, 0x59, 0x83, 0x16, 0x88, 0x5b, 0x46, 0x83, 0x2b, 0xa3, 0x2a},
	{0xe5, 0x8f, 0x27, 0xfd, 0x2f, 0x24, 0xf3, 0x40, 0xe4, 0x0b, 0xb4, 0xcf, 0x8d, 0x5d, 0xc1, 0x36, 0x84, 0x2b, 0x64, 0x11, 0x8b, 0x29, 0x8c, 0x17, 0xe2, 0xa6, 0x8c, 0xfb, 0x57, 0xe7, 0xc7, 0x48, 0x38, 0x4e, 0x3a, 0xad, 0xd4, 0xac, 0xed, 0x65, 0x6c, 0xd5, 0xd3, 0x2d, 0x3d, 0x44, 0xea, 0xb0},
	{0xc6, 0x73, 0xd8, 0x4d, 0x55, 0xae, 0x7d, 0x0b, 0x2a, 0xe7, 0x21, 0x58, 0x0e, 0x11, 0xb5, 0x31, 0xff, 0xb1, 0x5c, 0xb2, 0x22, 0x89, 0xa5, 0x3e, 0x7a, 0x94, 0x48, 0xc5, 0x5c, 0x41, 0x3b, 0x2e, 0x2b, 0x44, 0xa4, 0x60, 0xc8, 0x78, 0xab, 0xb8, 0xac, 0x94, 0xcb, 0x4b, 0x17, 0x6f, 0x7c, 0x14},
	{0x5b, 0x60, 0x10, 0x51, 0x44, 0x61, 0xf8, 0x08, 0x24, 0xca, 0x38, 0x25, 0xf1, 0x03, 0x9a, 0x09, 0x9c, 0xa4, 0xf5, 0x6f, 0x7b, 0x78, 0x98, 0x00, 0xaf, 0xdb, 0x29, 0x5d, 0xdb, 0x8d, 0xc8, 0x89, 0x5e, 0xd0, 0x35, 0x7c, 0x8a, 0x4c, 0x61, 0x19, 0x7c, 0xa5, 0xe3, 0x19, 0xf1, 0x27, 0x11, 0x4b},
	{0x05, 0xfb, 0x22, 0xef, 0xc3, 0x75, 0xa4, 0x0c, 0x17, 0xa8, 0x3d, 0x55, 0xfb, 0x9c, 0x6b, 0xf5, 0xed, 0xc0, 0x23, 0x19, 0x3a, 0x90, 0x81, 0x9e, 0xa0, 0x64, 0x36, 0x2f, 0x17, 0xd7, 0xd1, 0x05, 0x65, 0x58, 0xe9, 0x0b, 0xcf, 0xbb, 0xcf, 0x91, 0xf7, 0x99, 0x26, 0x00, 0x2d, 0x41, 0x28, 0xf4},
	{0xa6, 0xdd, 0x09, 0x1e, 0x59, 0x8c, 0xf8, 0x5b, 0xa4, 0x52, 0x67, 0xa9, 0x9d, 0xbc, 0x4b, 0x3e, 0x85, 0x52, 0xf0, 0x1c, 0xda, 0xf8, 0x50, 0xee, 0x57, 0x19, 0xe4, 0xad, 0x96, 0xb9, 0xef, 0x2e, 0x8e, 0xba, 0x80, 0xa6, 0xd8, 0xdd, 0x3a, 0xd1, 0x4c, 0xe2, 0x74, 0xd9, 0xb3, 0xcb, 0xf5, 0x81},
	{0xe8, 0x94, 0x5f, 0xc8, 0x35, 0xf6, 0x80, 0x82, 0xe0, 0xdb, 0xbe, 0x5d, 0x6d, 0x9b, 0x98, 0x13, 0xe5, 0xd2, 0x4b, 0xa5, 0x66, 0x9c, 0x0f, 0x50, 0x74, 0x9e, 0x6f, 0xfe, 0xeb, 0x52, 0xd5, 0xfc, 0x35, 0x02, 0x2e, 0xfd, 0xc5, 0xf8, 0x14, 0xb8, 0x72, 0xb9, 0xb8, 0xd6, 0xc8, 0x71, 0x6c, 0x9b},
	{0x72, 0x75, 0xab, 0xc9, 0xfd, 0x20, 0x50, 0xb8, 0x65, 0x3f, 0x9f, 0x0d, 0xc7, 0xd4, 0xd3, 0x05, 0x9e, 0xf8, 0x83, 0x29, 0x53, 0x48, 0x60, 0xc8, 0x68, 0xb9, 0x27, 0x45, 0xdc, 0x98, 0x45, 0x8b, 0x4b, 0x50, 0xb4, 0x2b, 0xee, 0xd9, 0x40, 0x9d, 0x91, 0x48, 0x55, 0x22, 0xdd, 0x4e, 0x85, 0xe6},
	{0x80, 0xdf, 0x02, 0x03, 0xc9, 0x71, 0x99, 0x8d, 0x87, 0x77, 0x9c, 0xed, 0x06, 0xd9, 0x41, 0xe8, 0x27, 0xcb, 0xd0, 0xce, 0xb3, 0x17, 0x6f, 0x23, 0x51, 0xe0, 0x68, 0x1e, 0xac, 0x28, 0x60, 0x84, 0xa3, 0x9f, 0x7c, 0x50, 0xe8, 0xd8, 0xcf, 0x4d, 0xde, 0x1d, 0xbb, 0x1c, 0x36, 0xac, 0xbb, 0x19},
	{0xfd, 0xe3, 0x3b, 0x9d, 0x0b, 0xb8, 0x70, 0xa5, 0xd7, 0x27, 0x0a, 0x05, 0x3a, 0x21, 0x2d, 0x74, 0xfd, 0xe2, 0xed, 0x2f, 0x33, 0x33, 0x42, 0x75, 0xf8, 0x69, 0x66, 0xc7, 0xf4, 0xf5, 0xf9, 0x8c, 0x74, 0xe2, 0x84, 0x77, 0x88, 0x34, 0x20, 0x9f, 0x1f, 0xef, 0x69, 0xfd, 0x23, 0x0b, 0x2d, 0x59},
	{0x8c, 0x5c, 0xc0, 0x72, 0xde, 0xca, 0x10, 0x62, 0xdd, 0x43, 0xe2, 0x02, 0x52, 0xe7, 0x64, 0x55, 0xf8, 0xa9, 0xf9, 0x0b, 0x98, 0x0f, 0xc0, 0x1c, 0x17, 0xc4, 0x60, 0xa6, 0x7c, 0x15, 0x8f, 0xa0, 0xa9, 0x92, 0xf5, 0xb3, 0x65, 0x31, 0x06, 0xd6, 0x20, 0xb1, 0x46, 0xe3, 0x90, 0x03, 0xe1, 0x0a},
	{0x30, 0xc3, 0x6e, 0x7b, 0x6f, 0xf1, 0x65, 0x0b, 0x8f, 0x7e, 0xa4, 0xaf, 0x65, 0x49, 0x67, 0xc6, 0x5b, 0x55, 0xfe, 0x58, 0xde, 0x41, 0x42, 0x8b, 0x6c, 0x84, 0x6e, 0xac, 0x9d, 0xb4, 0xe5, 0x61, 0x57, 0x0b, 0x94, 0xb8, 0x19, 0xc2, 0x9d, 0x17, 0xcd, 0xd0, 0x09, 0xd9, 0x36, 0x2c, 0xe0, 0x44},
	{0x16, 0x47, 0xf2, 0xba, 0x4c, 0xeb, 0xdf, 0x74, 0x5c, 0x33, 0x6b, 0xae, 0xb6, 0xd5, 0x0c, 0x5a, 0x1a, 0xb0, 0x9c, 0xf8, 0xa8, 0x46, 0xc2, 0x8c, 0x1e, 0x26, 0x8c, 0x8f, 0xc1, 0xfe, 0xd8, 0x18, 0x35, 0x27, 0xbd, 0xf7, 0x6d, 0x0d, 0xb1, 0xbb, 0x7f, 0xc4, 0x40, 0xd1, 0xa9, 0x15, 0xd3, 0xf2},
	{0xc5, 0x6f, 0x90, 0x80, 0x3c, 0x70, 0x98, 0xc3, 0xb8, 0x43, 0x5e, 0xe9, 0x3a, 0xbd, 0xe9, 0xcb, 0x0c, 0x54, 0xd1, 0xd2, 0x2b, 0x0e, 0xa3, 0x11, 0x48, 0xfc, 0x6e, 0x8f, 0xb3, 0x63, 0x02, 0xcf, 0x4c, 0x74, 0x85, 0x5f, 0x70, 0x1d, 0x05, 0xb2, 0x83, 0x92, 0x7b, 0x18, 0x9b, 0x8f, 0x7c, 0x96},
	{0x9d, 0xdb, 0x06, 0x39, 0x04, 0xf3, 0x25, 0x8e, 0xe1, 0xcc, 0xfa, 0xfc, 0xda, 0x97, 0xee, 0x3a, 0x81, 0x57, 0x7d, 0x69, 0x34, 0x76, 0x0e, 0x10, 0xc2, 0x61, 0xd5, 0xa0, 0x6d, 0xfd, 0x30, 0x42, 0x5f, 0x34, 0x24, 0xb0, 0x90, 0x7e, 0x29, 0x6f, 0x9b, 0x12, 0x71, 0xd6, 0x8b, 0x9e, 0x9e, 0x80},
	{0xe1, 0xf6, 0x85, 0x83, 0x84, 0x17, 0x6c, 0xcf, 0x47, 0x2f, 0x45, 0x42, 0x10, 0xef, 0x45, 0xe2, 0x6b, 0x6c, 0x36, 0x0c, 0x6a, 0x02, 0x15, 0xb2, 0x84, 0x7a, 0x81, 0xe9, 0xd0, 0x78, 0xf3, 0x8e, 0x2a, 0x9f, 0xf5, 0x3c, 0xeb, 0x4c, 0xb9, 0x8d, 0xd1, 0x7b, 0x66, 0xae, 0xf2, 0x10, 0x52, 0x62},
	{0x53, 0x53, 0x35, 0x69, 0xa0, 0x5b, 0x02, 0x0b, 0x0c, 0xb1, 0xc0, 0x37, 0xfc, 0xe0, 0xf3, 0xfa, 0xcc, 0x7f, 0x77, 0x6b, 0x14, 0xb3, 0xd5, 0xfb, 0x5a, 0x8e, 0x5e, 0x1a, 0xbd, 0xf5, 0xd5, 0x80, 0xf1, 0x33, 0x2c, 0x23, 0x63, 0x7d, 0x2e, 0xbb, 0x6d, 0x29, 0x00, 0x84, 0xee, 0x81, 0xa3, 0x42},
	{0x7e, 0xf7, 0x84, 0xd5, 0x4a, 0x59, 0xb2, 0x0f, 0xea, 0x5c, 0x41, 0x13, 0xb5, 0x6e, 0x36, 0x59, 0x46, 0x81, 0xfe, 0x2a, 0x73, 0xc7, 0x01, 0x84, 0x6b, 0x12, 0xcc, 0xb3, 0xe2, 0x79, 0x75, 0x7d, 0x0a, 0x01, 0xc3, 0xae, 0xf2, 0xb5, 0x52, 0x12, 0x5f, 0xe0, 0xe9, 0x9c, 0x1b, 0x95, 0x7d, 0x31},
	{0xb6, 0xdc, 0xc4, 0xc0, 0xfb, 0xbb, 0xe3, 0x24, 0x62, 0xa5, 0x4f, 0x11, 0x17, 0x0a, 0x0c, 0x58, 0x2f, 0x32, 0xf1, 0x26, 0x54, 0xb2, 0x0a, 0xff, 0xfd, 0xb0, 0x2d, 0xd2, 0x67, 0xad, 0x48, 0x50, 0x3e, 0x9d, 0x26, 0x34, 0xc3, 0xbb, 0x32, 0x81, 0x8f, 0xf8, 0x83, 0xe8, 0x5c, 0x8c, 0xd4, 0x39},
	{0x15, 0x5a, 0xb0, 0xda, 0x0d, 0xbb, 0xa5, 0xa2, 0x66, 0xf9, 0x22, 0x33, 0xef, 0xc7, 0x59, 0x50, 0x7e, 0xaa, 0xb3, 0xe8, 0x0a, 0x42, 0xe7, 0xab, 0xa0, 0x29, 0xa2, 0x9f, 0x4e, 0x53, 0x9d, 0x95, 0x4a, 0xea, 0x63, 0xd2, 0xd3, 0xd1, 0x63, 0x2e, 0x18, 0x59, 0x6e, 0xdd, 0xa4, 0xc7, 0x67, 0xeb},
	{0x78, 0x0e, 0xba, 0x3e, 0x10, 0x6b, 0x27, 0xf8, 0x39, 0x92, 0x4a, 0x01, 0x6d, 0x20, 0xc8, 0x70, 0xd6, 0x40, 0xcd, 0xc0, 0x05, 0x91, 0x09, 0xa6, 0xb2, 0x84, 0xba, 0x53, 0x36, 0xe4, 0x00, 0x01, 0x02, 0xd9, 0x17, 0xaf, 0xe5, 0x0c, 0xfd, 0xae, 0xf6, 0x03, 0x69, 0x72, 0x34, 0x35, 0x31, 0x6c},
	{0xb9, 0x4c, 0xa5, 0x09, 0x6c, 0x9d, 0x52, 0x7b, 0xb9, 0x2c, 0x2c, 0x3e, 0xc5, 0x49, 0x80, 0x14, 0xe1, 0x8a, 0xf1, 0x2e, 0xa2, 0x1e, 0x9a, 0x11, 0x00, 0x85, 0xed, 0x43, 0x4f, 0x00, 0xf9, 0x2b, 0x26, 0x77, 0x2c, 0xe0, 0x5e, 0x63, 0x66, 0x53, 0x86, 0x87, 0xa2, 0x68, 0x71, 0xdc, 0x32, 0x41},
	{0xd5, 0xa1, 0x1a, 0x05, 0xba, 0xba, 0x33, 0x00, 0x55, 0x36, 0x2a, 0xfe, 0x8b, 0x80, 0xb1, 0x54, 0x08, 0x6f, 0x6f, 0x8c, 0x0a, 0x64, 0x80, 0xeb, 0x68, 0xc7, 0xba, 0x12, 0x4a, 0xa1, 0x42, 0xba, 0xac, 0x5e, 0x1d, 0xfc, 0xa0, 0x5c, 0x98, 0x84, 0x76, 0xd1, 0xa4, 0x25, 0xd5, 0xd2, 0x97, 0x77},
	{0x20, 0x99, 0xf6, 0x3d, 0xbc, 0xc8, 0x7a, 0x51, 0x18, 0x8c, 0xc3, 0x36, 0xbe, 0x04, 0xc0, 0x85, 0xfe, 0x2e, 0x89, 0xad, 0x2a, 0x7d, 0x77, 0x53, 0x12, 0x5c, 0x41, 0x2b, 0xc7, 0x41, 0x93, 0x54, 0xd8, 0x5c, 0xc4, 0xe9, 0xe0, 0x8d, 0xba, 0x2d, 0xc6, 0x8a, 0xf8, 0x7e, 0x55, 0xfa, 0x26, 0xb5},
	{0xfd, 0x0c, 0x70, 0xe7, 0x89, 0x89, 0xcd, 0x34, 0x28, 0x71, 0x74, 0xde, 0xf8, 0x82, 0xd3, 0xb9, 0x4e, 0xae, 0x30, 0x88, 0xc4, 0x42, 0xc8, 0x75, 0x54, 0x6e, 0x58, 0x8c, 0xea, 0x62, 0x15, 0x8c, 0x2d, 0xd2, 0x92, 0x55, 0xdb, 0xf4, 0x62, 0xe6, 0xae, 0x42, 0xf9, 0xb5, 0xd7, 0xe8, 0x74, 0xef},
	{0x69, 0xb4, 0x06, 0x18, 0x27, 0x7a, 0x55, 0x2a, 0x7e, 0x19, 0xb0, 0xab, 0xff, 0xf3, 0x4b, 0xeb, 0x0d, 0xd8, 0x67, 0x51, 0x9e, 0x9b, 0x9e, 0x99, 0x33, 0x2b, 0xf5, 0xa2, 0x65, 0x7d, 0x8a, 0x37, 0xde, 0x7c, 0x48, 0x94, 0x74, 0xc1, 0xe1, 0xcf, 0x60, 0x70, 0x92, 0xdf, 0x78, 0xe1, 0xac, 0x34},
	{0x20, 0x2e, 0x04, 0x8b, 0x9a, 0xe9, 0x50, 0x20, 0x44, 0x59, 0xb7, 0xc6, 0xd7, 0xd3, 0x1a, 0xa5, 0x2d, 0xb7, 0x7b, 0x4d, 0x6e, 0x73, 0x07, 0x80, 0xdf, 0x46, 0xeb, 0x25, 0xc2, 0xf0, 0xc4, 0x84, 0x28, 0x23, 0x80, 0x01, 0x69, 0x8a, 0x4d, 0x5c, 0x81, 0x2e, 0xeb, 0x81, 0xde, 0xe6, 0x9c, 0xe7},
	{0xe1, 0xc2, 0x69, 0x76, 0x6e, 0x0e, 0x04, 0x01, 0xfc, 0xc4, 0x9c, 0xc0, 0xad, 0x14, 0x39, 0xc1, 0x30, 0xeb, 0xf2, 0x80, 0x1d, 0x9d, 0xf2, 0x2e, 0x3a, 0x1b, 0x60, 0x6b, 0x5b, 0xde, 0xd9, 0xca, 0xc5, 0x74, 0x57, 0xc2, 0x30, 0x05, 0xf4, 0x91, 0x4a, 0xf1, 0xae, 0x5f, 0x4f, 0x95, 0x2c, 0xac},
	{0x8f, 0x59, 0xc3, 0xb3, 0x06, 0x3c, 0x0f, 0x4e, 0x5a, 0x19, 0xb8, 0x58, 0xc1, 0x7d, 0x77, 0xf8, 0xaa, 0xf3, 0xff, 0x96, 0xbe, 0x4e, 0x10, 0xff, 0x30, 0x94, 0x95, 0x3a, 0x27, 0xcd, 0xba, 0x4c, 0x18, 0x2b, 0x08, 0x74, 0xa5, 0x39, 0xcf, 0xc2, 0x32, 0x46, 0x58, 0x4e, 0x31, 0x89, 0x0c, 0xc9},
	{0x5e, 0x5e, 0x84, 0xdb, 0xc4, 0x3e, 0xd8, 0xcc, 0x85, 0x3b, 0x49, 0xf1, 0x0f, 0x11, 0x02, 0xa9, 0x84, 0xbe, 0x1c, 0x48, 0xd2, 0xda, 0xd6, 0x93, 0xd2, 0x7f, 0x46, 0xb9, 0xb4, 0x8f, 0xd6, 0x80, 0x31, 0x9f, 0x51, 0x78, 0x63, 0xcf, 0x04, 0x07, 0x0f, 0xed, 0xe6, 0x7a, 0xfe, 0xd0, 0x46, 0x2f},
	{0x09, 0x66, 0x2f, 0x64, 0x9a, 0x02, 0x60, 0xb6, 0xf5, 0x37, 0xc2, 0x89, 0x5e, 0xf9, 0xbf, 0x02, 0xc6, 0x8b, 0x7a, 0xfe, 0xec, 0x50, 0xc2, 0x9a, 0xc7, 0xf2, 0x47, 0x00, 0x72, 0x13, 0x38, 0x05, 0x52, 0xcd, 0x00, 0x70, 0x4f, 0x3b, 0x58, 0xe8, 0x35, 0x7e, 0xc1, 0x24, 0x70, 0x19, 0x36, 0xf0},
	{0xb3, 0x2e, 0xe9, 0x6c, 0xa9, 0x3c, 0x94, 0x8d, 0x6c, 0xdf, 0x18, 0x57, 0xcd, 0x28, 0x5f, 0x90, 0x2f, 0x87, 0xc0, 0xf1, 0x76, 0xb4, 0x91, 0x2a, 0xdb, 0x9e, 0xea, 0x66, 0x08, 0x39, 0x2a, 0xbe, 0xf8, 0x03, 0x4d, 0x26, 0x4b, 0xe3, 0x16, 0xa7, 0xd3, 0xe7, 0x45, 0x8d, 0x71, 0xb8, 0xd3, 0x66},
	{0x5f, 0x07, 0x86, 0xb0, 0x81, 0x09, 0x75, 0x43, 0x0e, 0x66, 0x2e, 0x1d, 0x11, 0x9b, 0x75, 0x71, 0x46, 0xa9, 0x71, 0x89, 0x7f, 0xf4, 0x73, 0x1a, 0x0b, 0xa5, 0x17, 0x2c, 0xb8, 0x6c, 0xdf, 0x19, 0xe4, 0x1d, 0x72, 0xc8, 0x63, 0x2e, 0xc1, 0x57, 0x38, 0x5a, 0x8c, 0x3f, 0x6f, 0x54, 0xdb, 0x2b},
	{0x57, 0xde, 0x52, 0x20, 0x82, 0x3e, 0x40, 0xa3, 0x84, 0xe0, 0xd0, 0x1f, 0x1a, 0xd8, 0x9f, 0x8a, 0x6d, 0xf9, 0x33, 0xd3, 0x49, 0x1f, 0x0f, 0x69, 0x11, 0xa7, 0x69, 0xdd, 0x05, 0xed, 0xce, 0x5a, 0x52, 0xa5, 0x9d, 0xf8, 0x1e, 0xcb, 0xdf, 0xda, 0x6d, 0x58, 0x90, 0x59, 0x10, 0xe0, 0xfa, 0x72},
	{0xae, 0x98, 0x20, 0x94, 0xfe, 0xfa, 0xe5, 0x20, 0x99, 0xf0, 0xc3, 0xe1, 0xed, 0x97, 0x8d, 0x94, 0x23, 0x05, 0xaf, 0x5b, 0x00, 0x68, 0x57, 0xcd, 0xf6, 0x55, 0x0d, 0xe0, 0x83, 0x13, 0x22, 0xf3, 0xbf, 0x3e, 0xe4, 0xb8, 0x5d, 0xbd, 0x5f, 0x02, 0xac, 0x63, 0x42, 0xed, 0x71, 0xcd, 0xa6, 0x45},
	{0xdf, 0x7f, 0xa3, 0x9c, 0x91, 0x63, 0xee, 0x5a, 0x03, 0x6c, 0x16, 0x9b, 0xc3, 0x9e, 0x8e, 0xfb, 0x57, 0x24, 0xbc, 0x58, 0xa4, 0xda, 0x3c, 0x93, 0xbd, 0x29, 0xd7, 0xc9, 0x4d, 0x22, 0xbe, 0x8b, 0x7a, 0xe0, 0x3f, 0x12, 0x1c, 0x5f, 0xf1, 0x91, 0xb0, 0xe0, 0x53, 0xf1, 0xac, 0xc4, 0x55, 0x6b},
	{0xae, 0x41, 0xe7, 0x29, 0x1d, 0x56, 0x4d, 0x68, 0x19, 0xa3, 0xfe, 0xe6, 0xc5, 0xb7, 0x12, 0x22, 0x52, 0x4f, 0x79, 0x9c, 0x35, 0xef, 0x89, 0x1e, 0xbf, 0xca, 0xb9, 0x7d, 0x72, 0x55, 0xc6, 0x8c, 0x28, 0x2f, 0x71, 0xbc, 0x0a, 0x69, 0xef, 0x53, 0x96, 0x63, 0x1f, 0x2b, 0xed, 0xc0, 0xec, 0x56},
	{0xbe, 0xfa, 0x1e, 0x04, 0x44, 0x4a, 0x73, 0x35, 0x82, 0xf2, 0xe7, 0x65, 0xe9, 0x67, 0x78, 0x56, 0x01, 0xe1, 0x62, 0x45, 0x3c, 0xd7, 0x62, 0xf5, 0xd1, 0x29, 0xbd, 0x98, 0x4f, 0x57, 0xfa, 0x58, 0xea, 0x9d, 0xc5, 0x41, 0xca, 0x11, 0x11, 0x15, 0x2a, 0xff, 0xa1, 0x84, 0x5f, 0x94, 0x7f, 0x8f},
	{0x92, 0x1e, 0xef, 0x68, 0x4b, 0x75, 0x5e, 0x0a, 0x92, 0xbb, 0xe6, 0x2b, 0x06, 0x1b, 0x38, 0xf4, 0x89, 0x03, 0x88, 0x9f, 0x61, 0x96, 0xc2, 0x55, 0xa0, 0x27, 0x6d, 0x02, 0x70, 0x0b, 0x94, 0xce, 0x47, 0x4e, 0x4c, 0xe0, 0x55, 0xa0, 0xcc, 0x47, 0xc8, 0xee, 0xb1, 0x51, 0x80, 0x01, 0x30, 0x29},
	{0xcf, 0x98, 0xf1, 0x22, 0x83, 0x4d, 0x90, 0x94, 0x49, 0xf8, 0xbc, 0xa3, 0x81, 0xb4, 0x3e, 0x11, 0x9e, 0x78, 0xe6, 0xd4, 0x26, 0xdf, 0x79, 0xdb, 0xe2, 0x5b, 0xee, 0x76, 0x1b, 0x65, 0x82, 0x8d, 0x9d, 0x59, 0x14, 0xd0, 0x11, 0x2b, 0xd4, 0x9a, 0xfd, 0x09, 0xe2, 0x0e, 0x57, 0xe3, 0xa3, 0xdb},
	{0x90, 0xd9, 0x58, 0x8c, 0xc9, 0x24, 0x52, 0xc0, 0x88, 0xac, 0x8e, 0xd3, 0x63, 0x36, 0xda, 0x8a, 0xf7, 0xf8, 0x30, 0xfd, 0xc5, 0x30, 0x32, 0x1b, 0x4a, 0x8c, 0xcf, 0x82, 0xd9, 0x54, 0xf1, 0xce, 0xfe, 0x55, 0x05, 0x27, 0x96, 0x15, 0x8e, 0x46, 0xa6, 0xf5, 0x44, 0xd0, 0x94, 0xf7, 0x97, 0x63},
	{0x07, 0x9c, 0x80, 0x15, 0xe7, 0x31, 0xba, 0x9c, 0xeb, 0xb2, 0x80, 0x40, 0xd2, 0x67, 0x3a, 0x02, 0xce, 0x4c, 0xbe, 0xe6, 0x6f, 0xea, 0xec, 0x62, 0x86, 0x9b, 0x3e, 0xde, 0x14, 0xcf, 0xd0, 0x8d, 0xaf, 0xeb, 0x7b, 0x84, 0x78, 0xab, 0x79, 0x2a, 0xc7, 0x4b, 0x54, 0x99, 0xc6, 0x2e, 0xb0, 0x5d},
	{0x82, 0x2d, 0x0c, 0x6a, 0x7f, 0x5b, 0x0a, 0xd1, 0xb4, 0x4a, 0xe7, 0x36, 0xc0, 0xc5, 0xcb, 0x90, 0x55, 0x8b, 0x36, 0x4e, 0x33, 0x8a, 0xef, 0xf9, 0x7a, 0x9f, 0x29, 0xf7, 0x18, 0xad, 0xd4, 0x3a, 0xfc, 0x03, 0x55, 0xf5, 0x41, 0xca, 0xbd, 0xe2, 0x82, 0xc8, 0xae, 0x8e, 0x84, 0x6d, 0xda, 0x42},
	{0xc3, 0x74, 0xbd, 0x74, 0x87, 0xd0, 0x85, 0xd6, 0x2f, 0x48, 0xd8, 0xb8, 0x0b, 0xf1, 0x89, 0xb9, 0x53, 0x1a, 0xf0, 0x72, 0x34, 0x77, 0x2e, 0x8f, 0x09, 0x48, 0xd9, 0x15, 0xdf, 0xe1, 0x64, 0xfd, 0xfd, 0xa5, 0x42, 0xb2, 0x66, 0xbe, 0x72, 0x76, 0x36, 0xcb, 0x4c, 0xa5, 0xf4, 0x85, 0xf8, 0x91},
	{0x45, 0x66, 0x51, 0x2d, 0x7a, 0x47, 0xc9, 0x73, 0xc3, 0x35, 0x70, 0x4f, 0xce, 0x06, 0x7e, 0xd6, 0x1e, 0x67, 0x1c, 0x10, 0xc9, 0x9c, 0x0a, 0x87, 0x95, 0x73, 0x97, 0x1a, 0xfd, 0x2a, 0xce, 0xc8, 0xf2, 0x4f, 0x03, 0x30, 0xc7, 0x26, 0xd8, 0xb4, 0x29, 0xf8, 0xa4, 0x29, 0xf1, 0xdb, 0x3a, 0x42},
	{0xfa, 0x9b, 0x9a, 0xa1, 0x7f, 0xce, 0x65, 0x5a, 0x72, 0x4c, 0x02, 0x86, 0x52, 0x1f, 0x5a, 0x6b, 0x0d, 0xa6, 0x15, 0xdb, 0x4e, 0x6a, 0xea, 0xc9, 0x8b, 0xde, 0xa2, 0x51, 0xcf, 0x88, 0xfb, 0xcb, 0x14, 0x67, 0x9d, 0x34, 0x76, 0x6e, 0x6e, 0x12, 0x44, 0x22, 0xb9, 0x44, 0xe6, 0xea, 0x1d, 0xa0},
	{0x22, 0xb6, 0x78, 0x74, 0x37, 0x8b, 0x63, 0x92, 0x2a, 0x00, 0xf5, 0x7a, 0xf3, 0x15, 0xa9, 0xf8, 0x51, 0xd0, 0x92, 0x60, 0x2d, 0x44, 0x28, 0x04, 0x2b, 0x8f, 0x8f, 0xfe, 0x7a, 0x1f, 0x32, 0xe0, 0x24, 0x05, 0x36, 0x13, 0x02, 0x49, 0xd5, 0x11, 0x47, 0x7d, 0x7c, 0xe4, 0x02, 0x82, 0xfc, 0x6b},
	{0x88, 0x3c, 0x96, 0xda, 0x83, 0x2f, 0x6f, 0xc5, 0xf2, 0xb4, 0x6c, 0xab, 0x78, 0x9d, 0x7c, 0x4d, 0x83, 0x44, 0x74, 0x9e, 0x0a, 0x10, 0xd7, 0xf9, 0x3b, 0x39, 0xb0, 0xc0, 0xc8, 0x20, 0x6e, 0x62, 0xd7, 0x18, 0x13, 0x49, 0xde, 0x7e, 0x33, 0x90, 0x03, 0x84, 0x64, 0x84, 0xfa, 0x9b, 0x68, 0x9a},
	{0x2e, 0xd3, 0x4f, 0xe1, 0x7f, 0x60, 0x5c, 0x9e, 0x99, 0xdf, 0x34, 0x8b, 0xe9, 0xc6, 0x63, 0xa7, 0x2e, 0x02, 0xd3, 0xe9, 0x73, 0xc6, 0xf7, 0x23, 0xf5, 0xe6, 0xb9, 0x08, 0x4e, 0x9e, 0xe7, 0xf7, 0x9b, 0xd5, 0x57, 0x7a, 0xf6, 0x4e, 0x42, 0x07, 0x97, 0x0b, 0xfe, 0xc2, 0xd1, 0xa5, 0xe7, 0xba},
	{0x90, 0x05, 0xc9, 0x5a, 0x1b, 0x93, 0x8c, 0xda, 0xd2, 0x34, 0xcc, 0xac, 0x4f, 0xa6, 0x11, 0x4c, 0xef, 0x3f, 0xe3, 0xcc, 0x5d, 0x5a, 0x9a, 0x5e, 0xe9, 0xa1, 0x05, 0x29, 0x8c, 0x1d, 0x48, 0xb2, 0x5a, 0xcf, 0xac, 0x83, 0x40, 0xdf, 0xc3, 0x4c, 0xdf, 0xa0, 0x1e, 0x25, 0x73, 0x20, 0x2f, 0x54},
	{0x33, 0x7e, 0x2c, 0xc0, 0x41, 0x73, 0xb1, 0x50, 0x44, 0x55, 0x9c, 0x46, 0x0e, 0x5b, 0x35, 0x68, 0x67, 0x88, 0x8c, 0x41, 0x9a, 0x51, 0x38, 0xf0, 0xe1, 0xf6, 0xdb, 0x06, 0xae, 0x8e, 0xed, 0x6c, 0x53, 0x02, 0xf5, 0xd3, 0xcb, 0x76, 0x36, 0xdf, 0x88, 0x6e, 0xaf, 0xc4, 0xc0, 0x5e, 0x52, 0x09},
	{0x6f, 0x40, 0xfc, 0xc3, 0x2d, 0x48, 0xa6, 0x90, 0x25, 0x27, 0x21, 0x73, 0xb4, 0x48, 0xce, 0x51, 0x06, 0x2d, 0x51, 0xb9, 0xb7, 0xd6, 0x1a, 0x6a, 0x17, 0xb0, 0x5c, 0xf0, 0x36, 0x91, 0xfc, 0x6e, 0x10, 0xde, 0x97, 0x60, 0x2a, 0x75, 0x74, 0xd2, 0x13, 0xe8, 0xf8, 0x8b, 0xe3, 0xee, 0x71, 0x40},
	{0x7e, 0x8e, 0x7d, 0x45, 0xeb, 0x49, 0xcd, 0x3c, 0x35, 0x24, 0x68, 0x16, 0xaf, 0x2d, 0xe7, 0x62, 0xe0, 0x89, 0x23, 0x8e, 0xde, 0x76, 0xf8, 0x85, 0xc4, 0x06, 0xb1, 0x9e, 0xc7, 0xdb, 0x32, 0x6f, 0x22, 0xe8, 0x4a, 0xd5, 0x69, 0x04, 0xf0, 0xe6, 0x41, 0x6b, 0xf1, 0xd3, 0x78, 0xcc, 0x05, 0x93},
	{0xc4, 0xe2, 0x4d, 0xa7, 0x69, 0xae, 0x0c, 0xdd, 0xd4, 0xc8, 0x3e, 0x54, 0x76, 0xbf, 0x33, 0xf1, 0xe0, 0x91, 0x6e, 0x02, 0x20, 0x82, 0x95, 0x53, 0xa1, 0x73, 0x93, 0x63, 0x35, 0x35, 0x16, 0x60, 0x36, 0xdb, 0xe0, 0xf0, 0x85, 0x11, 0xc8, 0xe0, 0x84, 0xde, 0x9d, 0xf1, 0x62, 0xe0, 0xad, 0x3b},
	{0x3c, 0xf8, 0x5d, 0xf3, 0x8e, 0xb4, 0x26, 0x18, 0x0c, 0x2c, 0xdf, 0x50, 0xa7, 0x25, 0x6d, 0xaa, 0x8e, 0x6e, 0x2e, 0x45, 0xa4, 0x77, 0xa6, 0x6a, 0x78, 0x58, 0xf7, 0x3b, 0x5e, 0x6f, 0x92, 0xa5, 0x09, 0x5c, 0x53, 0x99, 0xbe, 0x24, 0xa1, 0xda, 0xf8, 0xee, 0x41, 0x4b, 0x36, 0xbf, 0x02, 0xef},
	{0x6d, 0x3e, 0x80, 0x33, 0xb4, 0x47, 0xb8, 0xc1, 0x36, 0x27, 0xe4, 0xe1, 0x04, 0x9e, 0x11, 0xa1, 0x5a, 0x41, 0xbd, 0x7c, 0x3d, 0x26, 0x71, 0xc0, 0xa1, 0xed, 0x03, 0xd9, 0x3f, 0x4c, 0x09, 0x59, 0xb5, 0xe3, 0xd7, 0xfb, 0x0c, 0x32, 0xa6, 0x6b, 0x36, 0xfd, 0x05, 0xe1, 0xd5, 0x94, 0xf9, 0xd6},
	{0x77, 0x50, 0x30, 0xc2, 0x72, 0x38, 0xc0, 0x3d, 0xa8, 0x2e, 0xe8, 0x32, 0x18, 0xfb, 0x84, 0x8d, 0xe5, 0x5d, 0xac, 0x17, 0xb5, 0x68, 0xdd, 0x31, 0x6a, 0x4b, 0xea, 0xee, 0xa2, 0x7d, 0x61, 0x0d, 0xb0, 0x86, 0x4f, 0x60, 0xe4, 0x3f, 0x3b, 0x97, 0xc8, 0xb6, 0x40, 0xc9, 0x5c, 0x0b, 0x02, 0xc5},
	{0x0a, 0x1f, 0x1c, 0xc7, 0xb5, 0xea, 0xda, 0xcb, 0x08, 0xc3, 0x8a, 0x9b, 0x6e, 0x3c, 0x55, 0x4b, 0xb8, 0x4f, 0x71, 0x8d, 0x31, 0xef, 0xc7, 0x0f, 0xa7, 0x17, 0xa2, 0xdd, 0xa2, 0xf8, 0xf3, 0xa3, 0x6e, 0x6e, 0xf9, 0xa1, 0x53, 0xe7, 0x9a, 0xc1, 0xa0, 0xbe, 0x5f, 0x5b, 0xe5, 0xfa, 0x0c, 0x4d},
	{0x26, 0xcd, 0xba, 0x61, 0xef, 0x79, 0xc1, 0x3a, 0x61, 0xbd, 0x85, 0x0d, 0xb7, 0x2e, 0x14, 0x3d, 0x3e, 0x4a, 0x07, 0x3d, 0x01, 0xc8, 0x1f, 0x92, 0xfc, 0x73, 0x24, 0xcd, 0xe4, 0x23, 0x99, 0xb2, 0x2c, 0xba, 0x43, 0x73, 0xcd, 0x01, 0x49, 0xcb, 0x26, 0x2f, 0x1c, 0x01, 0xcc, 0x96, 0x57, 0xc8},
	{0x6a, 0x68, 0x23, 0x0c, 0xb6, 0x0f, 0xff, 0x28, 0x6e, 0x22, 0xb6, 0xc6, 0x5e, 0xc3, 0xda, 0x39, 0xde, 0xe5, 0x10, 0x24, 0x36, 0x80, 0x8d, 0x0a, 0x97, 0xfc, 0xc2, 0x5c, 0x0d, 0xa5, 0x55, 0x0f, 0x6f, 0x10, 0x28, 0x35, 0x75, 0xfe, 0xf9, 0x76, 0xac, 0x90, 0x2f, 0xac, 0x1c, 0x1e, 0x26, 0xa7},
	{0x89, 0x04, 0xc8, 0xcc, 0x4c, 0x22, 0xe2, 0x69, 0x9d, 0xa3, 0x13, 0x86, 0x10, 0xf2, 0xd8, 0x70, 0x1f, 0xb4, 0x5e, 0x3c, 0x60, 0xbf, 0xa4, 0x11, 0x27, 0x41, 0xf6, 0x19, 0xcb, 0x85, 0x96, 0xfd, 0x2b, 0x4e, 0xb3, 0x96, 0x0e, 0x78, 0x8b, 0x9c, 0xd6, 0x3b, 0xff, 0x4c, 0x1e, 0x7e, 0xcb, 0xb0},
	{0x7e, 0x31, 0x6e, 0xb8, 0x5d, 0xc6, 0xdd, 0x2b, 0xf5, 0xbe, 0x4d, 0x65, 0xc9, 0x88, 0x7b, 0x65, 0xa8, 0xeb, 0xef, 0x7a, 0x99, 0x27, 0x62, 0xb5, 0x52, 0xe5, 0x2d, 0xce, 0x07, 0x53, 0xe2, 0x6d, 0x77, 0xe5, 0x0f, 0xc5, 0x18, 0x0b, 0x52, 0x9b, 0xb4, 0xc3, 0x1c, 0xbe, 0x16, 0x2b, 0xca, 0x64},
	{0x2c, 0xb8, 0xca, 0x33, 0xb1, 0xf8, 0x20, 0x23, 0x48, 0xbf, 0xf3, 0x0d, 0xd3, 0x32, 0x9d, 0x58, 0xa2, 0x90, 0x1c, 0x8f, 0x20, 0x07, 0x2b, 0xb0, 0x74, 0x45, 0x58, 0xf0, 0x37, 0x95, 0xbb, 0x03, 0x1d, 0x42, 0x5a, 0xae, 0x76, 0x7f, 0x8f, 0x01, 0x70, 0x4b, 0xa1, 0xa4, 0xd2, 0xb2, 0x80, 0x0e},
	{0x2c, 0x98, 0x59, 0x79, 0xfe, 0xa7, 0x48, 0xdd, 0xfa, 0x71, 0xaa, 0x85, 0xa9, 0xa4, 0x8b, 0x5c, 0x26, 0x08, 0x3a, 0xbd, 0x0c, 0x2a, 0xf1, 0xa4, 0x07, 0x34, 0x87, 0xa9, 0xe0, 0xa8, 0x94, 0x41, 0x62, 0x9e, 0x62, 0x72, 0xd8, 0x09, 0x98, 0x0e, 0x37, 0xd1, 0x5c, 0xc9, 0x66, 0x47, 0x5b, 0xd6},
	{0x4a, 0x66, 0x7d, 0x63, 0x48, 0xe7, 0xfb, 0x34, 0xf7, 0x9b, 0x25, 0x23, 0xe4, 0x87, 0x3b, 0x55, 0x13, 0x58, 0xcb, 0x2a, 0x4b, 0x64, 0xe3, 0xff, 0x29, 0x95, 0xa2, 0x1a, 0xfa, 0x74, 0xf8, 0x99, 0x42, 0xe6, 0x3b, 0x4d, 0xb8, 0x4a, 0x37, 0xd9, 0x87, 0x46, 0x07, 0x93, 0x20, 0xd6, 0xa4, 0x88},
	{0x5c, 0x57, 0x90, 0x8f, 0x5d, 0x49, 0xc2, 0xd6, 0x64, 0x97, 0xd5, 0xd1, 0xd8, 0x31, 0x30, 0xe8, 0x96, 0x7c, 0xdc, 0xbe, 0xca, 0x35, 0x05, 0x74, 0x53, 0xaf, 0x4a, 0xae, 0xd7, 0xc4, 0x88, 0x1f, 0xf3, 0x1f, 0xe4, 0x0e, 0xfe, 0x35, 0x8e, 0x2d, 0x64, 0x6a, 0x32, 0x9c, 0x46, 0x12, 0xf4, 0xd0},
	{0x85, 0xdb, 0x16, 0x44, 0xae, 0xbf, 0xcf, 0x7a, 0x84, 0x1e, 0x32, 0x94, 0x48, 0x08, 0x91, 0x02, 0xa4, 0xb7, 0xd1, 0xfc, 0xd7, 0x27, 0x70, 0xa8, 0xff, 0x1d, 0x5f, 0x87, 0x72, 0x96, 0x2e, 0xfd, 0xcc, 0x17, 0x14, 0x20, 0xcb, 0xb6, 0xff, 0x1f, 0xe2, 0xc7, 0xec, 0x05, 0x95, 0x04, 0x30, 0x06},
	{0x46, 0x60, 0xf7, 0x14, 0x85, 0x27, 0xce, 0x78, 0xc2, 0x54, 0xc3, 0x0d, 0x10, 0xc0, 0x64, 0x79, 0xd7, 0xdc, 0x42, 0x94, 0x5f, 0x0d, 0xad, 0xdb, 0x40, 0x78, 0x0c, 0x18, 0xa7, 0xcc, 0x90, 0xa4, 0xd8, 0xef, 0x9c, 0xa6, 0x6a, 0xa1, 0x8d, 0xdb, 0xe9, 0x21, 0xc2, 0x28, 0x17, 0x67, 0x07, 0x6f},
	{0xbc, 0x4a, 0x8e, 0x7a, 0x60, 0xb1, 0xf3, 0x48, 0x85, 0x63, 0x13, 0xd8, 0x25, 0x55, 0xeb, 0xed, 0xbd, 0x0c, 0x4b, 0x1d, 0x40, 0x53, 0xfd, 0xca, 0xb2, 0x43, 0x6a, 0x96, 0x5b, 0x96, 0xb2, 0x32, 0x66, 0x8c, 0x9b, 0xfc, 0x46, 0x07, 0xec, 0xb2, 0xaa, 0x7f, 0x27, 0x5b, 0x84, 0xf5, 0xc9, 0x86},
	{0x9a, 0xe9, 0xc6, 0xd5, 0x46, 0xbe, 0x9f, 0xb6, 0xd4, 0xc4, 0x5f, 0x45, 0xf0, 0xf2, 0x28, 0x0c, 0xdb, 0xa0, 0x0e, 0x4f, 0xe6, 0xac, 0x93, 0x0e, 0x06, 0xad, 0xd5, 0x70, 0x86, 0x7c, 0x3e, 0x82, 0x04, 0x8a, 0x84, 0x87, 0x2c, 0x7e, 0xf7, 0xf6, 0xd2, 0xfd, 0x09, 0x63, 0x5f, 0x20, 0xe6, 0x03},
	{0xde, 0x29, 0xf0, 0xa7, 0x98, 0x1c, 0x10, 0xe3, 0x5f, 0x7d, 0x95, 0x06, 0xb1, 0x71, 0xa8, 0x69, 0x9b, 0x4b, 0x0e, 0x0e, 0x32, 0xb0, 0xb8, 0x2f, 0x3c, 0xd6, 0x28, 0xb0, 0x4e, 0x0b, 0xd1, 0x09, 0x36, 0x60, 0x61, 0x67, 0xb5, 0xf1, 0xe9, 0x87, 0xbb, 0xed, 0xdf, 0x38, 0x9c, 0xf7, 0x58, 0xc3},
	{0x7f, 0xe5, 0xfa, 0xc4, 0xf8, 0x5a, 0x14, 0x5c, 0x33, 0x7d, 0xb2, 0x87, 0x26, 0xaf, 0x52, 0xf7, 0xf2, 0x5e, 0xeb, 0x63, 0x9a, 0x38, 0xc2, 0x03, 0x46, 0x61, 0xc8, 0xbd, 0x37, 0x1f, 0x67, 0x04, 0x27, 0xd6, 0xf2, 0x85, 0xfa, 0x9a, 0xab, 0x36, 0xc0, 0xc0, 0x68, 0x7b, 0x70, 0x3a, 0x01, 0x65},
	{0x79, 0x86, 0x8b, 0xb8, 0x0d, 0x75, 0x16, 0xa7, 0x9b, 0x6a, 0xe0, 0x82, 0x95, 0xb7, 0xe3, 0x9b, 0xde, 0x66, 0x7f, 0xa7, 0xe4, 0x45, 0x92, 0xaf, 0xe8, 0xf7, 0x6c, 0xa7, 0x5e, 0xdf, 0x1b, 0xc5, 0x99, 0xa5, 0xbc, 0x4a, 0x77, 0x97, 0x91, 0x2c, 0x43, 0xc5, 0xc2, 0xfa, 0xcb, 0xd3, 0x5f, 0xd6},
	{0x8e, 0xe7, 0xb2, 0x60, 0x10, 0xa2, 0x55, 0x3a, 0x52, 0xee, 0x21, 0xc4, 0x7a, 0x90, 0x07, 0x60, 0xb5, 0x8e, 0xbb, 0x1a, 0x5f, 0x30, 0x59, 0x1e, 0x85, 0xef, 0x00, 0xff, 0x23, 0x5c, 0x7a, 0xa7, 0x02, 0xbf, 0x72, 0xde, 0x49, 0x21, 0xd7, 0xfc, 0x29, 0x2c, 0x9e, 0x7f, 0x8b, 0xe8, 0xb3, 0x5e},
	{0x1b, 0x16, 0x75, 0x6f, 0xfb, 0xac, 0x84, 0x6c, 0x36, 0x3a, 0xde, 0x95, 0xf2, 0x7a, 0xa5, 0x09, 0x79, 0x34, 0xfd, 0x0d, 0xd1, 0x1e, 0x34, 0x3e, 0x29, 0x94, 0x2a, 0x00, 0xf0, 0x81, 0xfe, 0x8b, 0xef, 0xc9, 0x19, 0x62, 0xae, 0x96, 0x6a, 0x1e, 0xc5, 0x23, 0x79, 0x96, 0x26, 0x26, 0xb9, 0xf8},
	{0xc2, 0xec, 0xc9, 0x6c, 0xf5, 0xb3, 0x0e, 0xa1, 0x70, 0x29, 0x38, 0xc9, 0xcc, 0x63, 0xf1, 0xce, 0xf4, 0x76, 0x5b, 0x67, 0x13, 0xec, 0x83, 0xb3, 0xcb, 0xd5, 0x05, 0x51, 0xad, 0x1e, 0x17, 0xce, 0xf6, 0x80, 0x4d, 0x5f, 0x55, 0xed, 0x8c, 0x4e, 0x4e, 0xe7, 0xd6, 0x2f, 0xff, 0x4f, 0x83, 0x74},
	{0x8f, 0x9e, 0x50, 0xba, 0x45, 0x1c, 0xf3, 0x04, 0x5d, 0x5f, 0xc3, 0x0e, 0x1e, 0xe2, 0x6c, 0x9d, 0x38, 0x36, 0x3e, 0xe7, 0xbb, 0x17, 0x75, 0x54, 0x12, 0xb6, 0xc4, 0x8f, 0xd3, 0x70, 0xbc, 0x87, 0xd6, 0x4e, 0xad, 0x46, 0x7d, 0x58, 0x4f, 0x68, 0x6e, 0x28, 0xfe, 0x36, 0x5a, 0xc3, 0x72, 0x84},
	{0x4a, 0xac, 0x21, 0x69, 0x08, 0xef, 0x62, 0x93, 0x54, 0x22, 0x9e, 0xb3, 0x0e, 0x72, 0x41, 0x91, 0x0f, 0x0a, 0x63, 0xef, 0x9e, 0x28, 0xf6, 0x85, 0x7a, 0x65, 0x3a, 0x41, 0xe9, 0x6b, 0x98, 0x00, 0xd6, 0x06, 0x12, 0x9c, 0xdb, 0xf2, 0xe5, 0x41, 0xc6, 0x54, 0xf6, 0x05, 0x16, 0xd6, 0x38, 0x6e},
	{0xe6, 0xad, 0xe6, 0x59, 0x28, 0xd3, 0x7f, 0x76, 0x59, 0x32, 0x32, 0x13, 0xea, 0xf3, 0xf8, 0xee, 0xcd, 0x98, 0x73, 0x90, 0xc6, 0x3e, 0xfa, 0x8e, 0xd8, 0xff, 0xec, 0xd7, 0xbf, 0x5a, 0x17, 0x18, 0x33, 0x80, 0x7d, 0x54, 0x37, 0x6c, 0xc3, 0x1a, 0x71, 0x90, 0xf9, 0x68, 0xca, 0x1e, 0x43, 0x25},
	{0x51, 0xcf, 0x34, 0x61, 0x60, 0xa7, 0xf8, 0xf5, 0xb8, 0xcf, 0xa0, 0x12, 0xa2, 0x4b, 0xf7, 0x0b, 0x18, 0xed, 0xd4, 0xad, 0x77, 0xb3, 0x78, 0x1d, 0x2f, 0x4c, 0xe7, 0x73, 0x73, 0x08, 0x47, 0x3d, 0x2e, 0x03, 0x65, 0x7e, 0xe4, 0xbb, 0x26, 0xdb, 0xb2, 0x4d, 0xb4, 0x8c, 0x5e, 0x01, 0x1e, 0x49},
	{0x6e, 0x1b, 0x19, 0x59, 0x7c, 0x2b, 0xe3, 0x00, 0x12, 0x28, 0x43, 0x49, 0xbf, 0xd8, 0xfe, 0x34, 0x73, 0x26, 0x89, 0xbb, 0x7f, 0x59, 0xca, 0xc2, 0xc7, 0x1a, 0x88, 0x4a, 0x5a, 0x97, 0xcb, 0xb4, 0xa4, 0xa0, 0x19, 0x5f, 0xaa, 0x6e, 0x9f, 0x48, 0x97, 0xfd, 0xec, 0x0a, 0xcf, 0x9e, 0xcc, 0x10},
	{0x46, 0x74, 0x91, 0x9d, 0xf7, 0x61, 0x82, 0xa6, 0xb2, 0xd4, 0x07, 0x68, 0x88, 0x7e, 0x1c, 0xbe, 0x07, 0xac, 0x7e, 0xc1, 0xf7, 0xf1, 0x6f, 0x6f, 0x10, 0x3a, 0xdd, 0xd3, 0x82, 0x27, 0x13, 0x2c, 0xde, 0xbd, 0x00, 0x0c, 0xa5, 0xd7, 0x89, 0xbc, 0x91, 0xd2, 0x20, 0xfd, 0x0c, 0x62, 0xcd, 0x9a},
	{0xf5, 0xb7, 0xcf, 0x31, 0x31, 0x7f, 0x79, 0x51, 0x54, 0xf8, 0x50, 0xa3, 0xb7, 0x88, 0x27, 0x42, 0x61, 0x74, 0x29, 0xfd, 0x00, 0x0c, 0x32, 0x84, 0xfe, 0x69, 0x2c, 0xb1, 0xdc, 0x66, 0x33, 0x70, 0x89, 0x9d, 0xd6, 0xc5, 0xef, 0x51, 0xa7, 0x01, 0x22, 0x73, 0x9c, 0x22, 0xfa, 0xfd, 0xb7, 0x00},
	{0x01, 0x59, 0x8f, 0x63, 0x9d, 0x37, 0x57, 0x3d, 0x20, 0x76, 0x78, 0xe1, 0xe2, 0x26, 0x6a, 0x7b, 0xe3, 0x4f, 0x25, 0xc5, 0x18, 0x7a, 0xda, 0xb0, 0xd4, 0x34, 0x88, 0x24, 0xd8, 0xbb, 0x30, 0x40, 0x2f, 0x4b, 0x2f, 0xab, 0x6d, 0x47, 0x7b, 0x51, 0x76, 0xad, 0xd6, 0xac, 0xd4, 0xf1, 0x31, 0x10},
	{0x9a, 0xdd, 0x5d, 0x4e, 0xc5, 0xc9, 0xaf, 0xef, 0x50, 0x9a, 0x0e, 0xb3, 0x97, 0x2d, 0x93, 0x0a, 0x36, 0xe2, 0x86, 0xdd, 0xe4, 0x2d, 0x3f, 0x58, 0x96, 0x68, 0x14, 0x66, 0x92, 0x83, 0x05, 0x59, 0xcf, 0xac, 0x59, 0x66, 0x85, 0xce, 0x71, 0x81, 0x1b, 0xa5, 0x18, 0x57, 0x01, 0x72, 0x49, 0xbf},
	{0x57, 0xaf, 0x78, 0x35, 0xd3, 0xcf, 0x9c, 0x33, 0x91, 0xe4, 0x23, 0x3f, 0xa5, 0x42, 0x4b, 0x67, 0x9f, 0x9f, 0x38, 0xad, 0xc2, 0x9a, 0xa2, 0x7e, 0x53, 0x69, 0x3f, 0x4a, 0xd5, 0xa0, 0x62, 0x07, 0xb8, 0x44, 0xa1, 0x5d, 0x69, 0xd6, 0x9d, 0xbe, 0x2e, 0x63, 0x70, 0x0b, 0xdb, 0x7d, 0xeb, 0x78},
	{0xda, 0x12, 0x0b, 0x1e, 0xe3, 0x22, 0xaf, 0x5d, 0xfd, 0x7c, 0xd7, 0x63, 0x98, 0x8d, 0xea, 0x7e, 0xaa, 0xff, 0x36, 0xfe, 0xd1, 0xf1, 0x3d, 0x1e, 0x36, 0xb2, 0x2b, 0x53, 0x20, 0x39, 0x95, 0x40, 0x43, 0x26, 0x2d, 0x3f, 0x10, 0xfc, 0x5c, 0x7a, 0xe7, 0x84, 0xb5, 0x34, 0x5f, 0x01, 0x92, 0xaa},
	{0x2d, 0x7e, 0x06, 0x19, 0x57, 0x77, 0x42, 0xb2, 0xf8, 0x4b, 0xaf, 0x37, 0x8b, 0xbc, 0xb0, 0xdb, 0x09, 0x62, 0xc0, 0x99, 0x12, 0x5e, 0x30, 0x4a, 0x33, 0x1e, 0x78, 0xa0, 0xcc, 0xf8, 0x28, 0x8f, 0x97, 0x3d, 0x9b, 0xf2, 0x5d, 0x21, 0xf4, 0x02, 0xf8, 0xfc, 0xd0, 0x65, 0x17, 0x15, 0x58, 0x54},
	{0xe0, 0xd0, 0x6b, 0x69, 0x5d, 0x89, 0x2a, 0x04, 0xc2, 0x76, 0x9c, 0x66, 0xf0, 0xb0, 0xc4, 0x96, 0x79, 0x8a, 0xc9, 0x33, 0xa5, 0x7b, 0xe2, 0x08, 0x89, 0x13, 0x8d, 0xfe, 0xad, 0xb5, 0xf5, 0xd0, 0x74, 0x9d, 0x31, 0x2b, 0x7e, 0x09, 0xe8, 0xd3, 0xc3, 0xca, 0x1b, 0x5b, 0x58, 0x87, 0x61, 0xdc},
	{0xb2, 0x63, 0x0e, 0xa9, 0x5e, 0xdb, 0x62, 0x36, 0x22, 0xfc, 0xca, 0xb0, 0x78, 0x3c, 0xf7, 0x42, 0x25, 0x1c, 0xb1, 0xe7, 0x63, 0xcb, 0xdd, 0xfd, 0xf9, 0x91, 0x13, 0x68, 0xdc, 0xf2, 0x70, 0xd2, 0xe1, 0x02, 0x31, 0x27, 0xef, 0xbc, 0xb1, 0xf4, 0xb4, 0xb0, 0xeb, 0xa5, 0x84, 0x3c, 0x7a, 0xd0},
	{0x6c, 0x7e, 0xec, 0x9a, 0x56, 0x8d, 0x4f, 0x2f, 0xd1, 0xc4, 0x8f, 0xd4, 0xfe, 0x0c, 0x9d, 0xcf, 0x5b, 0x48, 0xdc, 0x81, 0xbc, 0x2a, 0xb1, 0x3d, 0xb3, 0xbb, 0x47, 0xa4, 0xc4, 0x8b, 0x8d, 0x06, 0x7a, 0xd9, 0xab, 0xe2, 0xb6, 0x60, 0x1a, 0x24, 0x72, 0xfb, 0x75, 0x8a, 0xfa, 0xc8, 0x19, 0xcb},
	{0x65, 0x9b, 0x07, 0xfc, 0x0d, 0x2b, 0x93, 0x41, 0x5b, 0x7c, 0xfd, 0x5b, 0x37, 0xe3, 0xe2, 0xc2, 0x68, 0x08, 0x85, 0x02, 0xdd, 0x13, 0xde, 0x8c, 0xf2, 0xf2, 0xc0, 0x3a, 0xc6, 0xb8, 0x33, 0x0b, 0xf1, 0x1c, 0x56, 0x1c, 0x14, 0x63, 0x43, 0x18, 0x98, 0x19, 0xea, 0xe0, 0xb9, 0x48, 0xb5, 0xcd},
	{0x24, 0x17, 0xf2, 0xd8, 0x8a, 0xc2, 0xf0, 0xa8, 0x13, 0x82, 0x5e, 0x13, 0xf3, 0x82, 0xd2, 0x25, 0x20, 0x2d, 0x43, 0xab, 0xf5, 0x23, 0x97, 0x33, 0x33, 0x4f, 0xd0, 0x8c, 0xf3, 0xb3, 0x5c, 0xe6, 0x77, 0x08, 0x7c, 0xb6, 0x21, 0x01, 0x30, 0x2f, 0xd8, 0x3b, 0x68, 0x8d, 0xc7, 0x46, 0x6d, 0x56},
	{0x67, 0xdf, 0xb8, 0xac, 0xee, 0x4a, 0xc8, 0xb2, 0x97, 0xd3, 0x5a, 0x09, 0x8d, 0x64, 0x0f, 0x71, 0x05, 0x8b, 0xfd, 0x5b, 0x35, 0x43, 0xa1, 0x5e, 0xcf, 0x87, 0xcd, 0x8f, 0x2f, 0x21, 0x6d, 0x7d, 0x3a, 0x41, 0x44, 0xca, 0x89, 0x4b, 0xf8, 0x87, 0x36, 0xe3, 0x45, 0x7f, 0xd1, 0x00, 0x11, 0x5c},
	{0x1a, 0x37, 0x6a, 0x8b, 0xd3, 0x03, 0x63, 0x7e, 0x9d, 0xbc, 0xff, 0x24, 0x1e, 0x7e, 0x48, 0xb3, 0x29, 0x44, 0x1e, 0xd3, 0x48, 0x46, 0x71, 0xff, 0xdd, 0x3b, 0x2e, 0x1d, 0xed, 0xd6, 0xd0, 0x79, 0x71, 0x22, 0x26, 0xf5, 0x1c, 0x70, 0x8a, 0x06, 0x52, 0x65, 0x06, 0x00, 0xed, 0x1e, 0x5e, 0x46},
	{0x77, 0x58, 0x80, 0x74, 0x9c, 0xfd, 0xf3, 0x2f, 0x3e, 0xf3, 0x7e, 0x6b, 0x23, 0x37, 0xb8, 0xb6, 0xa5, 0x76, 0x92, 0xf8, 0x22, 0xd1, 0xca, 0x21, 0x7f, 0x4a, 0x71, 0xb8, 0xfb, 0xa3, 0x8c, 0x86, 0xb5, 0x6b, 0x20, 0x74, 0xa2, 0xc0, 0xab, 0x4b, 0x56, 0xce, 0xba, 0xf6, 0x61, 0x13, 0x86, 0x08},
	{0xf7, 0x50, 0xec, 0x3f, 0x40, 0x61, 0x26, 0x2b, 0xe8, 0xf7, 0x67, 0x69, 0x94, 0xcb, 0x0b, 0xaa, 0xe8, 0x7b, 0x37, 0xf8, 0x9f, 0x3e, 0x5b, 0x35, 0x6d, 0xd0, 0x3f, 0xe7, 0x42, 0x04, 0x09, 0x62, 0x44, 0xc8, 0x40, 0xf9, 0xf8, 0x08, 0x9e, 0x4c, 0x07, 0x44, 0x92, 0x85, 0xb9, 0x8e, 0xe3, 0x77},
}

func TestBuilder48(t *testing.T) {
	const numBuckets = 3
	const valueSize = 48

	// Create a table with 3 buckets.
	builder, err := NewBuilderSized("", numBuckets*targetEntriesPerBucket, valueSize)
	require.NoError(t, err)
	require.NotNil(t, builder)
	assert.Len(t, builder.buckets, 3)
	defer builder.Close()

	// Insert a few entries.
	keys := []string{"hello", "world", "blub", "foo"}
	for i, key := range keys {
		require.NoError(t, builder.Insert([]byte(key), []byte(testValues48[i])))
	}
	{
		// print test values
		for _, tc := range testValues48 {
			spew.Dump(FormatByteSlice(tc))
		}
	}

	// Create index file.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal index.
	require.NoError(t, builder.Seal(context.TODO(), targetFile))

	// Assert binary content.
	buf, err := os.ReadFile(targetFile.Name())
	require.NoError(t, err)
	expected := concatBytes(
		// --- File header
		// magic
		[]byte{0x72, 0x64, 0x63, 0x65, 0x63, 0x69, 0x64, 0x78}, // 0
		// value size (48 bytes in this case)
		[]byte{0x30, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 1
		// num buckets
		[]byte{0x03, 0x00, 0x00, 0x00}, // 2
		// padding
		[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 3

		// --- Bucket header 0
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 4
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00}, // 5
		// hash len
		[]byte{0x03}, // 6
		// padding
		[]byte{0x00}, // 7
		// file offset
		[]byte{0x50, 0x00, 0x00, 0x00, 0x00, 0x00}, // 8

		// --- Bucket header 1
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 9
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00}, // 10
		// hash len
		[]byte{0x03}, // 11
		// padding
		[]byte{0x00}, // 12
		// file offset
		[]byte{0x83, 0x00, 0x00, 0x00, 0x00, 0x00}, // 13

		// --- Bucket header 2
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 14
		// num entries
		[]byte{0x02, 0x00, 0x00, 0x00}, // 15
		// hash len
		[]byte{0x03}, // 16
		// padding
		[]byte{0x00}, // 17
		// file offset
		[]byte{0xb6, 0x00, 0x00, 0x00, 0x00, 0x00}, // 18

		// --- Bucket 0
		// hash
		[]byte{0xe2, 0xdb, 0x55}, // 19
		// value
		[]byte{0xcc, 0x0a, 0xd4, 0x66, 0x32, 0x50, 0xc3, 0x96, 0x8b, 0x5c, 0x77, 0x7e, 0xb8, 0xfd, 0x9c, 0x78, 0xea, 0xfb, 0xd3, 0x4f, 0x1a, 0x59, 0x4e, 0xda, 0x1d, 0x90, 0x2a, 0xcd, 0x79, 0xb6, 0x0b, 0x2d, 0xea, 0x76, 0x36, 0x54, 0x65, 0xe6, 0x53, 0x1b, 0x70, 0x38, 0x84, 0xb2, 0xbf, 0x5d, 0xf9, 0x30}, // 20

		// --- Bucket 2
		// hash
		[]byte{0x92, 0xcd, 0xbb}, // 21
		// value
		[]byte{0x7c, 0x18, 0x51, 0xd7, 0x63, 0x83, 0xf9, 0xc5, 0xaa, 0x48, 0x3c, 0x8e, 0xff, 0xf0, 0xf1, 0xab, 0xee, 0xda, 0xb0, 0x2f, 0x92, 0xcc, 0xb8, 0x78, 0x11, 0x5b, 0xa0, 0xb9, 0xfa, 0xf5, 0x2e, 0xb4, 0xd7, 0x10, 0x2d, 0x7b, 0xe5, 0xb6, 0x9f, 0xd0, 0xb1, 0xff, 0xd0, 0xf2, 0xef, 0xcd, 0x72, 0x1a}, // 22
		// hash
		[]byte{0x98, 0x3d, 0xbd}, // 23
		// value
		[]byte{0xbb, 0x12, 0x08, 0x5f, 0x73, 0xee, 0x39, 0x69, 0x9f, 0x6e, 0x5a, 0xd8, 0x21, 0x2d, 0x43, 0xbe, 0x01, 0xc1, 0x3f, 0xc5, 0xfa, 0x86, 0x09, 0x7e, 0x97, 0x61, 0x59, 0xb8, 0xc9, 0x16, 0x47, 0xe3, 0x18, 0xfe, 0x52, 0x1e, 0xa2, 0x98, 0x59, 0x83, 0x16, 0x88, 0x5b, 0x46, 0x83, 0x2b, 0xa3, 0x2a}, // 24
		// hash
		[]byte{0xe3, 0x09, 0x6b}, // 25
		// value
		[]byte{0x0b, 0x2f, 0xc2, 0x4d, 0xc5, 0x98, 0x8b, 0x13, 0xd9, 0x17, 0xf8, 0xc1, 0xb8, 0x59, 0xd4, 0x24, 0xad, 0xef, 0xe5, 0xb6, 0xb8, 0xb9, 0xba, 0x01, 0x9c, 0xe0, 0x7f, 0x96, 0x25, 0x83, 0xd6, 0xbf, 0xa3, 0xb2, 0xf2, 0x29, 0xb9, 0xa1, 0xa1, 0x92, 0xd0, 0xc0, 0xe5, 0x06, 0x94, 0xea, 0x6c, 0xb3}, // 26
	)
	assert.Equal(t, expected, buf)

	{
		splitSizes := []int{
			// --- File header
			8, 8, 4, 12,
			// --- Bucket header 0
			4, 4, 1, 1, 6,
			// --- Bucket header 1
			4, 4, 1, 1, 6,
			// --- Bucket header 2
			4, 4, 1, 1, 6,
			// --- Bucket 0
			3, valueSize,
			// --- Bucket 2
			3, valueSize, 3, valueSize, 3, valueSize,
		}
		splitExpected := splitBufferWithProvidedSizes(expected, splitSizes)
		splitGot := splitBufferWithProvidedSizes(buf, splitSizes)

		comparations := compareBufferArrays(splitExpected, splitGot)

		for i, equal := range comparations {
			if !equal {
				t.Errorf("%d: \nexpected: %v, \n     got: %v", i, FormatByteSlice(splitExpected[i]), FormatByteSlice(splitGot[i]))
			}
		}
	}

	// Reset file offset.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)

	// Open index.
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")
	require.NotNil(t, db)

	// File header assertions.
	assert.Equal(t, Header{
		ValueSize:  valueSize,
		NumBuckets: numBuckets,
	}, db.Header)

	// Get bucket handles.
	buckets := make([]*Bucket, numBuckets)
	for i := range buckets {
		buckets[i], err = db.GetBucket(uint(i))
		require.NoError(t, err)
	}

	// Ensure out-of-bounds bucket accesses fail.
	_, wantErr := db.GetBucket(numBuckets)
	assert.EqualError(t, wantErr, "out of bounds bucket index: 3 >= 3")

	// Bucket header assertions.
	assert.Equal(t, BucketDescriptor{
		BucketHeader: BucketHeader{
			HashDomain: 0x00,
			NumEntries: 1,
			HashLen:    3,
			FileOffset: 0x50,
		},
		Stride:      3 + valueSize, // 3 + 36
		OffsetWidth: valueSize,
	}, buckets[0].BucketDescriptor)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 1,
		HashLen:    3,
		FileOffset: 131,
	}, buckets[1].BucketHeader)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 2,
		HashLen:    3,
		FileOffset: 182,
	}, buckets[2].BucketHeader)

	assert.Equal(t, uint8(3+valueSize), buckets[2].Stride)
	// Test lookups.
	entries, err := buckets[2].Load( /*batchSize*/ 3)
	require.NoError(t, err)
	assert.Equal(t, []Entry{
		{
			Hash:  12402072,
			Value: []byte(testValues48[3]),
		},
		{
			Hash:  7014883,
			Value: []byte(testValues48[2]),
		},
	}, entries)

	{
		for i, keyString := range keys {
			key := []byte(keyString)
			bucket, err := db.LookupBucket(key)
			require.NoError(t, err)

			value, err := bucket.Lookup(key)
			require.NoError(t, err)
			assert.Equal(t, []byte(testValues48[i]), value)
		}
	}
}

func TestBuilder48_Random(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}

	numKeys := uint(len(testValues48))
	const keySize = uint(16)
	const valueSize = 48
	const queries = int(10000)

	// Create new builder session.
	builder, err := NewBuilderSized("", numKeys, valueSize)
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.NotEmpty(t, builder.buckets)

	// Ensure we cleaned up after ourselves.
	defer func() {
		_, statErr := os.Stat(builder.dir)
		assert.Truef(t, errors.Is(statErr, fs.ErrNotExist), "Delete failed: %v", statErr)
	}()
	defer builder.Close()

	// Insert items to temp buckets.
	preInsert := time.Now()
	key := make([]byte, keySize)
	for i := uint(0); i < numKeys; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		err := builder.Insert(key, []byte(testValues48[i]))
		require.NoError(t, err)
	}
	t.Logf("Inserted %d keys in %s", numKeys, time.Since(preInsert))

	// Create file for final index.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal to final index.
	preSeal := time.Now()
	sealErr := builder.Seal(context.TODO(), targetFile)
	require.NoError(t, sealErr, "Seal failed")
	t.Logf("Sealed in %s", time.Since(preSeal))

	// Print some stats.
	targetStat, err := targetFile.Stat()
	require.NoError(t, err)
	t.Logf("Index size: %d (% .2f)", targetStat.Size(), decor.SizeB1000(targetStat.Size()))
	t.Logf("Bytes per entry: %f", float64(targetStat.Size())/float64(numKeys))
	t.Logf("Indexing speed: %f/s", float64(numKeys)/time.Since(preInsert).Seconds())

	// Open index.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")

	// Run query benchmark.
	preQuery := time.Now()
	for i := queries; i != 0; i-- {
		keyN := uint64(rand.Int63n(int64(numKeys)))
		binary.LittleEndian.PutUint64(key, keyN)

		bucket, err := db.LookupBucket(key)
		require.NoError(t, err)

		value, err := bucket.Lookup(key)
		require.NoError(t, err)
		require.Equal(t, []byte(testValues48[keyN]), value)
	}
	t.Logf("Queried %d items", queries)
	t.Logf("Query speed: %f/s", float64(queries)/time.Since(preQuery).Seconds())
}
