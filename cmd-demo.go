package main

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/davecgh/go-spew/spew"
	"github.com/filecoin-project/lassie/pkg/types"
	"github.com/ipfs/go-cid"
	"github.com/ipld/go-ipld-prime/storage/memstore"
	"github.com/rpcpool/yellowstone-faithful/iplddecoders"
	"github.com/urfave/cli/v2"
	"gopkg.in/yaml.v2"
)

type EpochListFile struct {
	Epochs map[uint64]Epoch `yaml:"epochs"`
}

type Epoch struct {
	EpochNumber uint64   `yaml:"-"`
	Cid         cid.Cid  `yaml:"cid"`
	Indexes     *Indexes `yaml:"indexes"`
}

type Indexes struct {
	SlotToCid string `yaml:"slot_to_cid"`
	SigToCid  string `yaml:"sig_to_cid"`
}

func ParseEpochListFile(path string) (*EpochListFile, []string, error) {
	// parse yaml
	// validate
	// return
	file, err := os.Open(path)
	if err != nil {
		return nil, nil, err
	}
	defer file.Close()

	decoder := yaml.NewDecoder(file)
	decoder.SetStrict(true)

	var epochList EpochListFile
	err = decoder.Decode(&epochList)
	if err != nil {
		return nil, nil, err
	}

	if len(epochList.Epochs) == 0 {
		return nil, nil, errors.New("no epochs found")
	}

	warnings := []string{}
	for epochNumber, epoch := range epochList.Epochs {
		epoch.EpochNumber = epochNumber
		if !epoch.Cid.Defined() {
			return nil, warnings, fmt.Errorf("epoch %d: cid not defined", epochNumber)
		}
		if epoch.Indexes == nil {
			warnings = append(warnings, fmt.Sprintf("epoch %d: indexes not defined -- getTransaction won't be available", epochNumber))
		}
		if epoch.Indexes.SlotToCid == "" {
			warnings = append(warnings, fmt.Sprintf("epoch %d: slot_to_cid index not specified -- block location will be inferred", epochNumber))
		}
		if epoch.Indexes.SigToCid == "" {
			warnings = append(warnings, fmt.Sprintf("epoch %d: sig_to_cid index not specified -- getTransaction won't be available", epochNumber))
		}
	}

	return &epochList, warnings, err
}

func newCmd_Demo() *cli.Command {
	return &cli.Command{
		Name:        "demo",
		Description: "demo",
		Flags:       []cli.Flag{},
		Action: func(cctx *cli.Context) error {
			{
				epochList, warnings, err := ParseEpochListFile("epochs.yaml")
				if err != nil {
					panic(err)
				}
				if len(warnings) > 0 {
					fmt.Println("WARNINGS:")
					for _, warning := range warnings {
						fmt.Println(warning)
					}
				}
				spew.Dump(epochList)
				return nil
			}

			// TODO:
			// - load epoch mapping from file (epoch number -> cid)
			// - fetch and validate all epoch objects
			// - fetch enough data to be able to respond to getBlock requests
			// - fetch enough data to be able to respond to getTransaction requests

			ls, err := newLassieWrapper(cctx)
			if err != nil {
				return fmt.Errorf("newLassieWrapper: %w", err)
			}

			store := NewWrappedMemStore()
			{
				stats, err := ls.Fetch(
					cctx.Context,
					cid.MustParse("bafyreiagdtlc3xwhbeywzpwmxvwkogcujhlsm6f4cfdgpjpyu77gkubro4"),
					"",
					types.DagScopeBlock,
					store,
				)
				if err != nil {
					return err
				}
				spew.Dump(stats)
			}
			{
				stats, err := ls.Fetch(
					cctx.Context,
					cid.MustParse("bafyreibobxzvpg7a424hlvspzaqvrzehudgqqgddbn5czxeowdhjqq4gta"),
					"",
					types.DagScopeBlock,
					store,
				)
				if err != nil {
					return err
				}
				spew.Dump(stats)
			}

			spew.Dump(store)

			{
				for key, node := range store.Bag {
					fmt.Println(cid.MustParse([]byte(key)))
					decoded, err := iplddecoders.DecodeAny(node)
					if err != nil {
						panic(err)
					}
					spew.Dump(decoded)
				}

				store.Each(context.Background(), func(cid cid.Cid, node []byte) error {
					fmt.Println(cid)
					decoded, err := iplddecoders.DecodeAny(node)
					if err != nil {
						panic(err)
					}
					spew.Dump(decoded)
					return nil
				})
			}
			return nil
		},
	}
}

type WrappedMemStore struct {
	*memstore.Store
}

func NewWrappedMemStore() *WrappedMemStore {
	return &WrappedMemStore{Store: &memstore.Store{}}
}

func (w *WrappedMemStore) Each(ctx context.Context, cb func(cid.Cid, []byte) error) error {
	for key, value := range w.Store.Bag {
		if err := cb(cid.MustParse([]byte(key)), value); err != nil {
			if errors.Is(err, ErrStopIteration) {
				return nil
			}
			return err
		}
	}
	return nil
}
