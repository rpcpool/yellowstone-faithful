package tooling

import (
	"bufio"
	"os"
)

type BufferedWritableFile struct {
	file *os.File
	buf  *bufio.Writer
}

// NewBufferedWritableFile creates a new file for writing, with a buffer.
// The file is created at the given path; if the file already exists, it will be overwritten.
func NewBufferedWritableFile(path string) (*BufferedWritableFile, error) {
	file, err := os.Create(path)
	if err != nil {
		return nil, err
	}
	return &BufferedWritableFile{
		file: file,
		buf:  bufio.NewWriterSize(file, 1024*1024),
	}, nil
}

func (bwf *BufferedWritableFile) WriteString(s string) error {
	_, err := bwf.buf.WriteString(s)
	return err
}

func (bwf *BufferedWritableFile) Close() error {
	if err := bwf.buf.Flush(); err != nil {
		return err
	}
	return bwf.file.Close()
}
