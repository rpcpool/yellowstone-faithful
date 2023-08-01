package epochlist

import (
	"encoding/binary"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"
)

type List struct {
	file   *os.File
	mu     sync.RWMutex
	header *Header
	values Values
}

var (
	_MAGIC   = [...]byte{'e', 'p', 'o', 'c', 'h', 'l', 's', 't'}
	_Version = uint64(1)
)

var headerLen = len(_MAGIC) + 8 // 8 bytes for the version

type Header struct {
	version uint64
}

// Version returns the version of the manifest.
func (h *Header) Version() uint64 {
	return h.version
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
		return nil, fmt.Errorf("this is not an epochlist file")
	}
	var version uint64
	err = binary.Read(file, binary.LittleEndian, &version)
	if err != nil {
		return nil, err
	}
	return &Header{
		version: version,
	}, nil
}

func writeHeader(file *os.File) error {
	_, err := file.Seek(0, io.SeekStart)
	if err != nil {
		return err
	}
	_, err = file.Write(_MAGIC[:])
	if err != nil {
		return err
	}
	err = binary.Write(file, binary.LittleEndian, _Version)
	if err != nil {
		return err
	}
	return nil
}

// New creates a new epochlist file or opens an existing one.
func New(filename string) (*List, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	man := &List{
		file: file,
	}
	currentFileSize, err := man.getFileSize()
	if err != nil {
		return nil, err
	}
	if currentFileSize == 0 {
		err = writeHeader(file)
		if err != nil {
			return nil, err
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
	if currentFileSize > 0 && (currentFileSize-int64(headerLen))%2 != 0 {
		return nil, fmt.Errorf("manifest is corrupt: size=%d", currentFileSize)
	}
	{ // read all the values
		currentvalues, err := man.Load()
		if err != nil {
			return nil, err
		}
		man.values = currentvalues
	}
	return man, nil
}

func (m *List) Close() error {
	return m.close()
}

func (m *List) close() (err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	err = m.file.Close()
	if err != nil {
		return err
	}
	return
}

// Flush flushes the cache to disk.
func (m *List) Flush() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.file.Sync()
}

// ContentSizeBytes returns the size of the content in bytes
// (not including the header).
func (m *List) ContentSizeBytes() (int64, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.getContentLength()
}

// getFileSize returns the size of the file in bytes (header + content).
func (m *List) getFileSize() (int64, error) {
	fi, err := m.file.Stat()
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// getContentLength returns the length of the content in bytes.
func (m *List) getContentLength() (int64, error) {
	currentFileSize, err := m.getFileSize()
	if err != nil {
		return 0, err
	}
	return currentFileSize - int64(headerLen), nil
}

// Put appends the given uint16 to the file.
func (m *List) Put(value uint16) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header == nil {
		err := writeHeader(m.file)
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
	return m.write(value)
}

func (m *List) HasOrPut(value uint16) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.header == nil {
		err := writeHeader(m.file)
		if err != nil {
			return false, err
		}
		m.header = &Header{
			version: _Version,
		}
		_, err = m.file.Seek(0, io.SeekEnd)
		if err != nil {
			return false, err
		}
	}
	if m.values.Has(value) {
		return true, nil
	}
	err := m.write(value)
	if err != nil {
		return false, err
	}
	return false, nil
}

// write appends the given uint64 tuple to the file.
func (m *List) write(value uint16) error {
	buf := uint16ToBytes(value)
	_, err := m.file.Write(buf)
	if err != nil {
		return err
	}
	m.values = append(m.values, value)
	return nil
}

// Load reads all the uint64 tuples from the file.
func (m *List) Load() (Values, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.readAllContent()
}

func (m *List) getContentReader() (io.Reader, int64, error) {
	currentContentSize, err := m.getContentLength()
	if err != nil {
		return nil, -1, err
	}
	return io.NewSectionReader(m.file, int64(headerLen), currentContentSize), currentContentSize, nil
}

// readAllContent reads all the uint64 tuples from the file.
func (m *List) readAllContent() (Values, error) {
	sectionReader, currentContentSize, err := m.getContentReader()
	if err != nil {
		return nil, err
	}
	buf := make([]byte, 2)
	values := make([]uint16, 0, currentContentSize/2)
	for {
		_, err := io.ReadFull(sectionReader, buf)
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}
		values = append(values, binary.LittleEndian.Uint16(buf[:2]))
	}
	return values, nil
}

func uint16ToBytes(i uint16) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b[0:2], i)
	return b
}

type Values []uint16

// First returns the first value in the slice.
func (v Values) First() (uint16, bool) {
	if len(v) == 0 {
		return 0, false
	}
	return v[0], true
}

// Last returns the last value in the slice.
func (v Values) Last() (uint16, bool) {
	if len(v) == 0 {
		return 0, false
	}
	if len(v) == 1 {
		return v[0], true
	}
	return v[len(v)-1], true
}

func (v Values) Sort() {
	sort.Slice(v, func(i, j int) bool {
		return v[i] < v[j]
	})
}

func (v Values) Has(value uint16) bool {
	for _, v := range v {
		if v == value {
			return true
		}
	}
	return false
}

// Unique returns a new slice with unique values, sorted.
func (v Values) Unique() Values {
	if len(v) == 0 {
		return nil
	}
	m := make(map[uint16]struct{}, len(v))
	for _, value := range v {
		m[value] = struct{}{}
	}
	unique := make([]uint16, 0, len(m))
	for value := range m {
		unique = append(unique, value)
	}
	sort.Slice(unique, func(i, j int) bool {
		return unique[i] < unique[j]
	})
	return unique
}
