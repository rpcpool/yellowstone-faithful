package manifest

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sync"
)

type Manifest struct {
	file *os.File
	mu   sync.RWMutex
}

func NewManifest(filename string) (*Manifest, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	// seek to the end of the file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	m := &Manifest{
		file: file,
	}
	currentSize, err := m.getSize()
	if err != nil {
		return nil, err
	}
	if currentSize > 0 && currentSize%16 != 0 {
		return nil, fmt.Errorf("manifest is corrupt: size=%d", currentSize)
	}
	return m, nil
}

func (m *Manifest) Close() error {
	return m.close()
}

func (m *Manifest) close() (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	err = m.file.Close()
	if err != nil {
		return err
	}
	return
}

// Flush flushes the cache to disk.
func (m *Manifest) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.file.Sync()
}

// Size returns the size of the file in bytes.
func (m *Manifest) Size() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getSize()
}

// getSize returns the size of the file in bytes.
func (m *Manifest) getSize() (int64, error) {
	fi, err := m.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Put appends the given uint64 tuple to the file.
func (m *Manifest) Put(key, value uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.write(key, value)
}

// write appends the given uint64 tuple to the file.
func (m *Manifest) write(key, value uint64) error {
	// write the key
	buf := uint64ToBytes(key)
	buf = append(buf, uint64ToBytes(value)...)
	_, err := m.file.Write(buf)
	if err != nil {
		return err
	}
	return nil
}

// ReadAll reads all the uint64 tuples from the file.
func (m *Manifest) ReadAll() ([][2]uint64, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readAll()
}

// readAll reads all the uint64 tuples from the file.
func (m *Manifest) readAll() ([][2]uint64, error) {
	currentFileSize, err := m.getSize()
	if err != nil {
		return nil, err
	}
	sectionReader := io.NewSectionReader(m.file, 0, currentFileSize)
	buf := make([]byte, 16)
	values := make([][2]uint64, 0, currentFileSize/16)
	for {
		_, err := io.ReadFull(sectionReader, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		key := binary.LittleEndian.Uint64(buf[:8])
		value := binary.LittleEndian.Uint64(buf[8:])
		values = append(values, [2]uint64{key, value})
	}
	return values, nil
}

func uint64ToBytes(i uint64) []byte {
	b := make([]byte, 8)
	binary.LittleEndian.PutUint64(b, i)
	return b
}
