package main

import (
	"bytes"
	"flag"
	"fmt"
	"io"
	"os"
)

// for file in $(find /media/runner/solana-2/indexes -name "*.index" | grep mainnet); do
// 	echo $file
// 	go run . $file
// done

var (
	oldMagic = [8]byte{'r', 'd', 'c', 'e', 'c', 'i', 'd', 'x'}
	// compact index sized
	newMagic = [8]byte{'c', 'o', 'm', 'p', 'i', 's', 'z', 'd'}
)

func main() {
	var dry bool
	flag.BoolVar(&dry, "dry", false, "dry run")
	flag.Parse()
	file := flag.Arg(0)
	if file == "" {
		panic("need file arg")
	}
	fmt.Println()
	fmt.Printf("File: %s\n", file)
	// open read write
	f, err := os.OpenFile(file, os.O_RDWR, 0)
	if err != nil {
		panic(err)
	}
	defer f.Close()
	b := make([]byte, 8)
	_, err = io.ReadFull(f, b)
	if err != nil {
		panic(err)
	}

	fmt.Printf("First 8 bytes = %v , as string = %s\n", b, b)
	target := oldMagic
	if !bytes.Equal(b, target[:]) {
		fmt.Printf("Doesn't match old magic; skipping\n")
		return
	}
	if dry {
		fmt.Printf("⚪ Dry run, not replacing and exiting\n")
		return
	}
	fmt.Printf("Found old magic; replacing with new magic\n")
	_, err = f.Seek(0, 0)
	if err != nil {
		panic(err)
	}
	_, err = f.Write(newMagic[:])
	if err != nil {
		panic(err)
	}
	fmt.Printf("✅ Replaced old magic with new magic.\n")
}
