package compactindexsized

import "io"

type SeekableBuffer struct {
	buf []byte
	pos int
}

func NewSeekableBuffer(buf []byte) *SeekableBuffer {
	return &SeekableBuffer{buf: buf}
}

func (sb *SeekableBuffer) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case 0:
		sb.pos = int(offset)
	case 1:
		sb.pos += int(offset)
	case 2:
		sb.pos = len(sb.buf) + int(offset)
	}
	return int64(sb.pos), nil
}

func (sb *SeekableBuffer) Read(p []byte) (n int, err error) {
	if sb.pos >= len(sb.buf) {
		return 0, io.EOF
	}
	n = copy(p, sb.buf[sb.pos:])
	sb.pos += n
	return
}

func (sb *SeekableBuffer) ReadAt(p []byte, off int64) (n int, err error) {
	if off < 0 || off >= int64(len(sb.buf)) {
		return 0, io.EOF
	}
	n = copy(p, sb.buf[off:])
	return
}

func (sb *SeekableBuffer) Write(p []byte) (n int, err error) {
	if sb.pos >= len(sb.buf) {
		return 0, io.EOF
	}
	n = copy(sb.buf[sb.pos:], p)
	sb.pos += n
	return
}
