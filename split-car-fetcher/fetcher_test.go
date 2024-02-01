package splitcarfetcher

import (
	"bytes"
	"fmt"
	"io"
	"testing"
)

func TestMulti(t *testing.T) {
	reader1 := bytes.NewReader([]byte("Hello "))
	reader2 := bytes.NewReader([]byte("Worlds"))
	multiReader := NewMultiReaderAt([]io.ReaderAt{reader1, reader2}, []int64{6, 5})
	{
		off := int64(0)
		accu := make([]byte, 0)
		for {
			buf := make([]byte, 1)
			n, err := multiReader.ReadAt(buf, off)
			if err != nil {
				if err == io.EOF {
					break
				}
				panic(err)
			}
			if n != 1 {
				panic(fmt.Errorf("unexpected size: %d", n))
			}
			accu = append(accu, buf[0])
			off++
		}
		if !bytes.Equal(accu, []byte("Hello Worlds")) {
			panic(fmt.Errorf("unexpected accu: %s", accu))
		}
		fmt.Printf("accu = %s\n", accu)
	}
	{
		off := int64(0)
		accu := make([]byte, 0)
		for {
			buf := make([]byte, 2)
			n, err := multiReader.ReadAt(buf, off)
			if err != nil {
				if err == io.EOF {
					break
				}
				panic(err)
			}
			if n != 2 {
				panic(fmt.Errorf("unexpected size: %d", n))
			}
			accu = append(accu, buf...)
			off += 2
		}
		if !bytes.Equal(accu, []byte("Hello Worlds")) {
			panic(fmt.Errorf("unexpected accu: %q", accu))
		}
		fmt.Printf("accu = %s\n", accu)
	}

	{
		buf := make([]byte, 11)
		n, err := multiReader.ReadAt(buf, 0)
		if err != nil {
			panic(err)
		}
		if n != 11 {
			panic(fmt.Errorf("unexpected size: %d", n))
		}
		fmt.Printf("buf = %s\n", buf)
	}
	{
		buf := make([]byte, 5)
		n, err := multiReader.ReadAt(buf, 0)
		if err != nil {
			panic(err)
		}
		if n != 5 {
			panic(fmt.Errorf("unexpected size: %d", n))
		}
		if !bytes.Equal(buf, []byte("Hello")) {
			panic(fmt.Errorf("unexpected buf: %s", buf))
		}
		fmt.Printf("buf = %s\n", buf)
	}
	{
		buf := make([]byte, 6)
		n, err := multiReader.ReadAt(buf, 0)
		if err != nil {
			panic(err)
		}
		if n != 6 {
			panic(fmt.Errorf("unexpected size: %d", n))
		}
		if !bytes.Equal(buf, []byte("Hello ")) {
			panic(fmt.Errorf("unexpected buf: %s", buf))
		}
		fmt.Printf("buf = %s\n", buf)
	}
	{
		buf := make([]byte, 7)
		n, err := multiReader.ReadAt(buf, 0)
		if err != nil {
			panic(err)
		}
		if n != 7 {
			panic(fmt.Errorf("unexpected size: %d", n))
		}
		if !bytes.Equal(buf, []byte("Hello W")) {
			panic(fmt.Errorf("unexpected buf: %s", buf))
		}
		fmt.Printf("buf = %s\n", buf)
	}
	{
		buf := make([]byte, 7)
		n, err := multiReader.ReadAt(buf, 2)
		if err != nil {
			panic(err)
		}
		if n != 7 {
			panic(fmt.Errorf("unexpected size: %d", n))
		}
		if !bytes.Equal(buf, []byte("llo Wor")) {
			panic(fmt.Errorf("unexpected buf: %s", buf))
		}
		fmt.Printf("buf = %s\n", buf)
	}
}
