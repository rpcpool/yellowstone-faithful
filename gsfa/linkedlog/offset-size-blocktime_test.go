package linkedlog

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestOffsetAndSizeAndBlocktime(t *testing.T) {
	{
		ca := OffsetAndSizeAndBlocktime{
			Offset:    1,
			Size:      2,
			Blocktime: 3,
		}
		buf := ca.Bytes()

		{
			ca2 := OffsetAndSizeAndBlocktime{}
			err := ca2.FromBytes(buf)
			if err != nil {
				panic(err)
			}
			if ca != ca2 {
				panic(fmt.Sprintf("expected %v, got %v", ca, ca2))
			}
		}
	}
	{
		// now with very high values
		ca := OffsetAndSizeAndBlocktime{
			Offset:    281474976710655,
			Size:      16777215,
			Blocktime: 1099511627775,
		}
		buf := ca.Bytes()

		{
			ca2 := OffsetAndSizeAndBlocktime{}
			err := ca2.FromBytes(buf)
			if err != nil {
				panic(err)
			}
			if ca != ca2 {
				panic(fmt.Sprintf("expected %v, got %v", ca, ca2))
			}
		}
	}
	{
		many := []OffsetAndSizeAndBlocktime{
			{
				Offset:    1,
				Size:      2,
				Blocktime: 3,
			},
			{
				Offset:    4,
				Size:      5,
				Blocktime: 6,
			},
			{
				Offset:    281474976710655,
				Size:      16777215,
				Blocktime: 1099511627775,
			},
		}
		buf := make([]byte, 0, binary.MaxVarintLen64*3*len(many))
		for _, ca := range many {
			buf = append(buf, ca.Bytes()...)
		}

		{
			many2, err := OffsetAndSizeAndBlocktimeSliceFromBytes(buf)
			if err != nil {
				panic(err)
			}
			if len(many) != len(many2) {
				panic(fmt.Sprintf("expected %v, got %v", many, many2))
			}
			for i := range many {
				if many[i] != many2[i] {
					panic(fmt.Sprintf("expected %v, got %v", many, many2))
				}
			}
		}
	}
}
