package ipldbindcode

import (
	"encoding/json"
	"testing"

	"github.com/ipfs/go-cid"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/stretchr/testify/require"
)

func TestDataFrame(t *testing.T) {
	{
		df := DataFrame{
			Kind: 6,
		}
		{
			h := 456
			hp := &h
			df.Hash = &hp
		}
		{
			i := 123
			ip := &i
			df.Index = &ip
		}
		{
			t := 789
			tp := &t
			df.Total = &tp
		}
		df.Data = []uint8{1, 2, 3}
		{
			got, err := df.MarshalJSON()
			require.NoError(t, err)
			want := `{"kind":6,"hash":"456","index":123,"total":789,"data":"AQID","next":null}`
			if string(got) != want {
				t.Fatalf("got %s, want %s", got, want)
			}
			{
				// try unmarshal
				var df2 DataFrame
				err := json.Unmarshal(got, &df2)
				require.NoError(t, err)

				require.Equal(t, df, df2)
			}
		}
		// now add some next values
		parsedCid, err := cid.Parse("bafyreigggzehcmuibshwtq35acyie6cyuahqjklwe5stxnqoqosuevz6w4")
		require.NoError(t, err)
		next := &List__Link{
			cidlink.Link{Cid: parsedCid},
		}
		df.Next = &next
		{
			got, err := df.MarshalJSON()
			require.NoError(t, err)
			want := `{"kind":6,"hash":"456","index":123,"total":789,"data":"AQID","next":[{"/":"bafyreigggzehcmuibshwtq35acyie6cyuahqjklwe5stxnqoqosuevz6w4"}]}`
			if string(got) != want {
				t.Fatalf("got %s, want %s", got, want)
			}
			{
				// try unmarshal
				var df2 DataFrame
				err := json.Unmarshal(got, &df2)
				require.NoError(t, err)

				require.Equal(t, df, df2)
			}
		}
	}
}
