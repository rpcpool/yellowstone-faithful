package manifest

import (
	"bufio"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/rpcpool/yellowstone-faithful/indexmeta"
)

type Manifest struct {
	file   *os.File
	mu     sync.RWMutex
	header *Header
}

var (
	_MAGIC   = [...]byte{'g', 's', 'f', 'a', 'm', 'n', 'f', 's'}
	_Version = uint64(3)
)

var headerLenWithoutMeta = len(_MAGIC) + 8 // 8 bytes for the version

type Header struct {
	version      uint64
	metaByteSize int64
	meta         indexmeta.Meta
}

// Version returns the version of the manifest.
func (h *Header) Version() uint64 {
	return h.version
}

// Meta returns the metadata of the manifest.
func (h *Header) Meta() indexmeta.Meta {
	return h.meta
}

func readHeader(file *os.File) (*Header, error) {
	// seek to the beginning of the file
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return nil, err
	}
	var magic [8]byte
	_, err = io.ReadFull(file, magic[:])
	if err != nil {
		return nil, err
	}
	if magic != _MAGIC {
		return nil, fmt.Errorf("this is not a gsfa manifest file")
	}
	var version uint64
	err = binary.Read(file, binary.LittleEndian, &version)
	if err != nil {
		return nil, err
	}
	header := &Header{
		version: version,
	}
	if version >= 2 {
		var meta indexmeta.Meta
		err = meta.UnmarshalWithDecoder(bufio.NewReader(file))
		if err != nil {
			return nil, err
		}
		header.metaByteSize = int64(len(meta.Bytes()))
		header.meta = meta
	}
	return header, nil
}

func writeHeader(file *os.File, meta indexmeta.Meta, version uint64) error {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = file.Write(_MAGIC[:])
	if err != nil {
		return err
	}
	err = binary.Write(file, binary.LittleEndian, version)
	if err != nil {
		return err
	}
	metaBytes := meta.Bytes()
	_, err = file.Write(metaBytes)
	if err != nil {
		return err
	}
	return nil
}

// NewManifest creates a new manifest or opens an existing one.
func NewManifest(filename string, meta indexmeta.Meta) (*Manifest, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	man := &Manifest{
		file: file,
	}
	currentFileSize, err := man.getFileSize()
	if err != nil {
		return nil, err
	}
	if currentFileSize == 0 {
		err = writeHeader(file, meta, _Version)
		if err != nil {
			return nil, err
		}
		man.header = &Header{
			version:      _Version,
			metaByteSize: int64(len(meta.Bytes())),
			meta:         meta,
		}
	} else {
		header, err := readHeader(file)
		if err != nil {
			return nil, err
		}
		if header.Version() != _Version {
			return nil, fmt.Errorf("unsupported manifest version: %d", header.Version())
		}
		man.header = header
	}
	// seek to the end of the file
	_, err = file.Seek(0, io.SeekEnd)
	if err != nil {
		return nil, err
	}
	dataSizeWithoutHeaderAndMeta := currentFileSize - int64(headerLenWithoutMeta) - man.header.metaByteSize
	if currentFileSize > 0 && (dataSizeWithoutHeaderAndMeta)%16 != 0 {
		return nil, fmt.Errorf("manifest is corrupt: size=%d", currentFileSize)
	}
	return man, nil
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
	m.file = nil
	m.header = nil
	return
}

func (m *Manifest) Meta() indexmeta.Meta {
	return m.header.meta
}

// Flush flushes the cache to disk.
func (m *Manifest) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.file.Sync()
}

// ContentSizeBytes returns the size of the content in bytes
// (not including the header).
func (m *Manifest) ContentSizeBytes() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getContentLength()
}

// getFileSize returns the size of the file in bytes (header + content).
func (m *Manifest) getFileSize() (int64, error) {
	fi, err := m.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// getContentLength returns the length of the content in bytes.
func (m *Manifest) getContentLength() (int64, error) {
	currentFileSize, err := m.getFileSize()
	if err != nil {
		return 0, err
	}
	return currentFileSize - int64(headerLenWithoutMeta) - m.header.metaByteSize, nil
}

func (m *Manifest) Version() uint64 {
	return m.header.version
}

// Put appends the given uint64 tuple to the file.
func (m *Manifest) Put(key, value uint64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header == nil {
		err := writeHeader(m.file, indexmeta.Meta{}, _Version)
		if err != nil {
			return err
		}
		m.header = &Header{
			version: _Version,
		}
		_, err = m.file.Seek(0, io.SeekEnd)
		if err != nil {
			return err
		}
	}
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
func (m *Manifest) ReadAll() (Values, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readAllContent()
}

func (m *Manifest) getContentReader() (io.Reader, int64, error) {
	currentContentSize, err := m.getContentLength()
	if err != nil {
		return nil, -1, err
	}
	return io.NewSectionReader(m.file, int64(headerLenWithoutMeta)+m.header.metaByteSize, currentContentSize), currentContentSize, nil
}

// readAllContent reads all the uint64 tuples from the file.
func (m *Manifest) readAllContent() (Values, error) {
	sectionReader, currentContentSize, err := m.getContentReader()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 16)
	values := make([][2]uint64, 0, currentContentSize/16)
	for {
		_, err := io.ReadFull(sectionReader, buf)
		if errors.Is(err, io.EOF) {
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

type Values [][2]uint64

// First returns the first value in the slice.
func (v Values) First() ([2]uint64, bool) {
	if len(v) == 0 {
		return [2]uint64{}, false
	}
	return v[0], true
}

// Last returns the last value in the slice.
func (v Values) Last() ([2]uint64, bool) {
	if len(v) == 0 {
		return [2]uint64{}, false
	}
	return v[len(v)-1], true
}
