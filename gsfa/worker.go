package gsfa

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/dustin/go-humanize"
	"github.com/gagliardetto/solana-go"
)

func workerRead(indexRoot string, pubkey string, limit int) error {
	gsfa, err := NewGsfaReader(indexRoot)
	if err != nil {
		return err
	}
	defer gsfa.Close()
	gsfa.Get(context.Background(), solana.NewWallet().PublicKey(), limit) // warmup

	pk, err := solana.PublicKeyFromBase58(pubkey)
	if err != nil {
		return err
	}

	startedAt := time.Now()
	sigs, err := gsfa.Get(context.Background(), pk, limit)
	if err != nil {
		return err
	}
	fmt.Printf("Got %d signatures in %s\n", len(sigs), time.Since(startedAt))
	spew.Dump(sigs)
	{
		took("Get", func() {
			gsfa.Get(context.Background(), pk, limit)
		})
		pk2 := solana.MPK("CiDwVBFgWV9VwPVuU3sPsph2xgPzRYhuhVVA7EcZoKd")
		took("Get", func() {
			gsfa.Get(context.Background(), pk2, limit)
		})
		took("Get", func() {
			gsfa.Get(context.Background(), pk, limit)
		})
		took("Get", func() {
			gsfa.Get(context.Background(), pk2, limit)
		})
		took("Get", func() {
			gsfa.Get(context.Background(), solana.MPK("c8fpTXm3XTSVpAViQ9cBdU56t3ByGe7j6UTUKhqhPxs"), limit)
		})
	}
	return nil
}

func took(name string, cb func()) {
	startedAt := time.Now()
	cb()
	fmt.Printf("%s took %s\n", name, time.Since(startedAt))
}

func workerDemoLoad(root string, numGlobalAccounts uint64, numSigs int) error {
	defer func() {
		{
			// print the size of the index.
			size, err := getDirSize(root)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Index folder size: %s\n", humanize.Bytes(size))
		}
	}()
	///----------------------------------------

	accu, err := NewGsfaWriter(
		root,
		500_000,
	)
	if err != nil {
		return fmt.Errorf("error while opening accumulator: %w", err)
	}
	defer func() {
		accu.Close()
	}()

	for i := 0; i < numSigs; i++ {
		sig := generateRandomSignature()
		howManyKeys := genRandomNumberBetween(1, 10)
		keys := genRandomKeys(numGlobalAccounts, howManyKeys)
		err = accu.Push(sig, keys)
		if err != nil {
			panic(err)
		}
		if i%(numSigs/10) == 0 {
			fmt.Println(keys[0], i/(numSigs/10))
		}
	}
	// force flush:
	if err := accu.Flush(); err != nil {
		panic(err)
	}
	fmt.Println("Flushed.")
	return nil
}

func worker(root string) error {
	indexFolder := filepath.Join(root, "offsets-index")
	os.MkdirAll(indexFolder, os.ModePerm)

	defer func() {
		{
			// print the size of the index.
			size, err := getDirSize(root)
			if err != nil {
				panic(err)
			}
			fmt.Printf("Index folder size: %s\n", humanize.Bytes(size))
		}
	}()
	///----------------------------------------

	accu, err := NewGsfaWriter(
		root,
		1000000,
	)
	if err != nil {
		return fmt.Errorf("error while opening accumulator: %w", err)
	}
	defer func() {
		accu.Close()
	}()
	keyStrings := []string{
		"CeTwZkgj9bLSkBJ8WPJZFe9zeQ2Z9HJwK8CMfUTEgWYQ",
		"BMRAPhUR3NirnaSAUCWRUwC62jyqgEUAyGbzR8wz3c64",
		"DUN6D2M598AHrhHggySMDWzvzoZwgJuPXAUKVeRQZ6DZ",
		"38aNxeW6VhSu9sAwvfqSEoqEeXZ5B6KrHX4QwPG9NdTZ",
		"EhgYh1aR5jXKzpVJnZgQqYYyhfXCbgA39W1td1EGHbe9",
		"HN7uRdKmGJKaXjK9HayxaXG5TSDCbZTDR5tR4zT9NeUy",
		"5T8kksicHbDXMETjRHFN5LGWXeTjn3n9dNkqfc6CceFA",
		"BfjScBGJ2KUXKRNPdpi2XCBEhYGyzz8BLY7CLw1SJany",
		"AAzR77VN31T68J3LbLu64XBkrtjr1T9aNycvsR1nitzJ",
		"71rkqrWSqqEccxFhSwHZFmaEwZBvWFxLX66C4VJBBioV",
	}
	keys := make([]solana.PublicKey, len(keyStrings))
	for i, keyString := range keyStrings {
		keys[i] = solana.MustPublicKeyFromBase58(keyString)
	}

	{
		sig := solana.MustSignatureFromBase58("5YXMTLhABRRs5NtE66kjS6re17pN7xoC8UYvChYMuHySBviyTjxKcsw7riibTtmbRBGxqXX7C3FHgbbsfNk6z2Ga")
		err = accu.Push(sig, keys)
		if err != nil {
			panic(err)
		}
	}
	{
		sig := solana.MustSignatureFromBase58("5kyKrGTGHJhPgWohMW7kn3YS7rq3rdKAHf7J7SnZNZjwPta7jT5tKV9oNejvKQX6k2DcCbk358zKSdQFawMnr8ci")
		err = accu.Push(sig, keys[0:1])
		if err != nil {
			panic(err)
		}

	}
	{
		sig := solana.MustSignatureFromBase58("5tPrsxsjifTuXJKvw4KvpGAG73s3JWwhgGbQckYYQC4gexHqiV9tBRnSan5YjMv7vwvJjZfC6rvC8AX8HaRpjA13")
		err = accu.Push(sig, keys[0:3])
		if err != nil {
			panic(err)
		}
	}
	// force flush:
	if err := accu.Flush(); err != nil {
		panic(err)
	}
	fmt.Println("Flushed.")
	{
		sig := solana.MustSignatureFromBase58("3ighZ7KKy1SQsayDX4sfsoC5HAKqZG8XT8NfQh9GmSjq3DCHBnNY9Vc6JNinAxnUdCHxTKoNVALeUA24yd98ZEWt")
		err = accu.Push(sig, keys)
		if err != nil {
			panic(err)
		}
	}
	{
		sig := solana.MustSignatureFromBase58("4LoJumTFxec2viccvKZZL2gieYDbUu7EsuDzNkr73aKxrF4Kb5FNqgQvUpthYoGbjU46iDVsfEYTpMtZEFZy5RCG")
		err = accu.Push(sig, keys[0:1])
		if err != nil {
			panic(err)
		}
	}
	{
		sig := solana.MustSignatureFromBase58("3MGAb27HPFka3JhoLYwoR268EVHe7NMa8mLuTV7Z9sPXayDhFEmNLGvjDR1aBoPzoVKD4i6ws38vRZ7X45NkneeS")
		err = accu.Push(sig, keys[0:3])
		if err != nil {
			panic(err)
		}
	}
	// force flush:
	if err := accu.Flush(); err != nil {
		panic(err)
	}
	fmt.Println("Flushed.")
	return nil
}
