package linkedlog

import (
	"encoding/binary"
	"fmt"
	"testing"
)

func TestOffsetAndSizeAndSlot(t *testing.T) {
	{
		ca := OffsetAndSizeAndSlot{
			Offset: 1,
			Size:   2,
			Slot:   3,
			Flags:  NewBitmap(),
		}
		buf := ca.Bytes()

		{
			ca2 := OffsetAndSizeAndSlot{}
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
		ca := OffsetAndSizeAndSlot{
			Offset: 281474976710655,
			Size:   16777215,
			Slot:   1099511627775,
			Flags:  NewBitmap(),
		}
		ca.Flags.Set(0, true)
		ca.Flags.Set(1, true)
		buf := ca.Bytes()

		{
			ca2 := OffsetAndSizeAndSlot{}
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
		many := []OffsetAndSizeAndSlot{
			{
				Offset: 1,
				Size:   2,
				Slot:   3,
				Flags:  NewBitmapFromValues(true, false, true),
			},
			{
				Offset: 4,
				Size:   5,
				Slot:   6,
				Flags:  NewBitmapFromValues(false, true, false, true),
			},
			{
				Offset: 281474976710655,
				Size:   16777215,
				Slot:   1099511627775,
				Flags:  NewBitmapFromValues(true, true, false, true),
			},
		}
		buf := make([]byte, 0, binary.MaxVarintLen64*3*len(many))
		for _, ca := range many {
			buf = append(buf, ca.Bytes()...)
		}

		{
			many2, err := OffsetAndSizeAndSlotSliceFromBytes(buf)
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
