package store

// Copyright 2023 rpcpool
// This file has been modified by github.com/gagliardetto
//
// Copyright 2020 IPLD Team and various authors and contributors
// See LICENSE for details.
import (
	"time"

	"github.com/rpcpool/yellowstone-faithful/gsfa/store/types"
)

const (
	defaultFileCacheSize   = 512
	defaultIndexSizeBits   = uint8(24)
	defaultIndexFileSize   = uint32(1024 * 1024 * 1024)
	defaultPrimaryFileSize = uint32(1024 * 1024 * 1024)
	defaultBurstRate       = 4 * 1024 * 1024
	defaultSyncInterval    = time.Second
	defaultGCInterval      = 30 * time.Minute
	defaultGCTimeLimit     = 5 * time.Minute
)

type config struct {
	fileCacheSize   int
	indexSizeBits   uint8
	indexFileSize   uint32
	primaryFileSize uint32
	syncInterval    time.Duration
	burstRate       types.Work
	gcInterval      time.Duration
	gcTimeLimit     time.Duration
	syncOnFlush     bool
}

type Option func(*config)

// apply applies the given options to this config.
func (c *config) apply(opts []Option) {
	for _, opt := range opts {
		opt(c)
	}
}

// FileCacheSize is the number of open files the index file cache may keep.
func FileCacheSize(size int) Option {
	return func(c *config) {
		c.fileCacheSize = size
	}
}

// IndexBitSize is the number of bits in an index prefix.
func IndexBitSize(indexBitSize uint8) Option {
	return func(c *config) {
		c.indexSizeBits = indexBitSize
	}
}

// IndexFileSize is the maximum offset an index record can have within an
// individual index file, before the record must be stored in another file.
func IndexFileSize(indexFileSize uint32) Option {
	return func(c *config) {
		c.indexFileSize = indexFileSize
	}
}

// PrimaryFileSize is the maximum offset a primary record can have within an
// individual primary file, before the record must be stored in another file.
func PrimaryFileSize(fileSize uint32) Option {
	return func(c *config) {
		c.primaryFileSize = fileSize
	}
}

// SyncInterval determines how frequently changes are flushed to disk.
func SyncInterval(syncInterval time.Duration) Option {
	return func(c *config) {
		c.syncInterval = syncInterval
	}
}

// BurstRate specifies how much data can accumulate in memory, at a rate faster
// than can be flushed, before causing a synchronous flush.
func BurstRate(burstRate uint64) Option {
	return func(c *config) {
		c.burstRate = types.Work(burstRate)
	}
}

// GCInterval is the amount of time to wait between GC cycles. A value of 0
// disables garbage collection.
func GCInterval(gcInterval time.Duration) Option {
	return func(c *config) {
		c.gcInterval = gcInterval
	}
}

// GCTimeLimit is the maximum amount of time that a GC cycle may run.
func GCTimeLimit(gcTimeLimit time.Duration) Option {
	return func(c *config) {
		c.gcTimeLimit = gcTimeLimit
	}
}

// SyncOnFlush, when set to true, causes fsync to be called as part of Flush.
func SyncOnFlush(syncOnFlush bool) Option {
	return func(c *config) {
		c.syncOnFlush = syncOnFlush
	}
}
