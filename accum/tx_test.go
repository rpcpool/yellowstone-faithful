package accum

import (
	"testing"

	"github.com/davecgh/go-spew/spew"
	"github.com/stretchr/testify/require"
)

func TestSortPieces(t *testing.T) {
	{
		tx := TransactionWithSlot{
			Offset: 100,
			Length: 200,
			MetadataPieces: MetadataPieceSectionRefs{
				{Offset: 42, Length: 20},
				{Offset: 0, Length: 20},
				{Offset: 21, Length: 20},
			},
		}

		totalOffset, totalLength, numPieces := tx.GetTotalOffsetAndLengthAndCount()
		spew.Dump(totalOffset, totalLength, numPieces)
		require.Equal(t, uint64(0), totalOffset)
		require.Equal(t, uint64(300), totalLength)
		require.Equal(t, 4, numPieces) // 3 pieces + 1 transaction
	}
}
