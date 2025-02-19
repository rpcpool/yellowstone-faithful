package iplddecoders

import (
	"encoding/json"
	"testing"

	"github.com/davecgh/go-spew/spew"
	cidlink "github.com/ipld/go-ipld-prime/linking/cid"
	"github.com/rpcpool/yellowstone-faithful/ipld/ipldbindcode"
	"github.com/stretchr/testify/require"
)

func TestEpoch(t *testing.T) {
	t.Run("classic/0", func(t *testing.T) {
		epoch, err := _DecodeEpochClassic(epoch_raw0)
		require.NoError(t, err)

		require.Equal(t, 4, epoch.Kind)
		require.Equal(t, KindEpoch, Kind(epoch.Kind))
		require.Equal(t, uint64(39), uint64(epoch.Epoch))
		require.Len(t, epoch.Subsets, 18)

		require.Equal(t, "bafyreias7lbmf6arupr6eskzm2wmd3xbml6d7ieievb3zde6634sv4fqty", epoch.Subsets[0].String())
		require.Equal(t, "bafyreien5cdsa6iaru2ltb346qotacgvzyrkbyufy75nqlr7p67qd7gbpi", epoch.Subsets[1].String())
		require.Equal(t, "bafyreia424a7ec3dx25r3btpi62cmfpjh2jmfmfrf66253co6epfwznucy", epoch.Subsets[2].String())
		require.Equal(t, "bafyreibioycvipupzfxab26zqf4axb7ghr6rz2q7x4j4ecl26a6ejmwnwe", epoch.Subsets[3].String())
		require.Equal(t, "bafyreif5zhe3ptanpnwfqp6cdie3dy46q3kqrtrpuuprppzrnsoztvmdla", epoch.Subsets[4].String())
		require.Equal(t, "bafyreih636mvxdrcboblum55dl5uhw4tsaj2euyikkwa64oi7aofqw2kuq", epoch.Subsets[5].String())
		require.Equal(t, "bafyreicbm23uvxusj67rsya53j6bc3rovr2ccly3pviljjglp4frypwojm", epoch.Subsets[6].String())
		require.Equal(t, "bafyreifhtknmnxrn6bpvngx3tzcc5hpgm25z6z5ioi3w36rmdrdwyuxhom", epoch.Subsets[7].String())
		require.Equal(t, "bafyreicsi5bepry34caou6fanoh2oqd6z4xerdmgmbnavhlgksaqqyyjha", epoch.Subsets[8].String())
		require.Equal(t, "bafyreiak5ezxvtsyjwpwohebymghgddlkgjbpqkwfhqhsjlcihcn5a33oq", epoch.Subsets[9].String())
		require.Equal(t, "bafyreigcs57a64jrwueug2zinpactvlt5fyq4nldqkhh7shbpixdkmbfha", epoch.Subsets[10].String())
		require.Equal(t, "bafyreigbbnmlyqairftvgpwi7z7pul4mothra7o53b3ythfr2gsdatngra", epoch.Subsets[11].String())
		require.Equal(t, "bafyreifbsq33fzmzyiyy3og73nmtk76vct76d7rcdk24nzfgjuebqtkedi", epoch.Subsets[12].String())
		require.Equal(t, "bafyreiadscov2rht764uwre47mjaltwskpsdjk76bfczkcj7lpmyid4ffi", epoch.Subsets[13].String())
		require.Equal(t, "bafyreid4n3aulssv24u4ffwg6wmyieyjovxhchuj452snuzttibx2vbu4u", epoch.Subsets[14].String())
		require.Equal(t, "bafyreibxeir3ywclsofoo3ar6i2z3kqxljupk3dhwu2gzicdcp27jrewvi", epoch.Subsets[15].String())
		require.Equal(t, "bafyreih6jdnpx6qspzph2ztdnygv44asgq7ec2u3qbczefkom72icb5qxu", epoch.Subsets[16].String())
		require.Equal(t, "bafyreibm4uwn3bsfja6q7fmyhzptj756iuwc5pdeeq62lmzwvsbzld4pzm", epoch.Subsets[17].String())
	})
	t.Run("fast/0", func(t *testing.T) {
		epoch, err := _DecodeEpochFast(epoch_raw0)
		require.NoError(t, err)

		require.Equal(t, 4, epoch.Kind)
		require.Equal(t, KindEpoch, Kind(epoch.Kind))
		require.Equal(t, uint64(39), uint64(epoch.Epoch))
		require.Len(t, epoch.Subsets, 18)

		require.Equal(t, "bafyreias7lbmf6arupr6eskzm2wmd3xbml6d7ieievb3zde6634sv4fqty", epoch.Subsets[0].String())
		require.Equal(t, "bafyreien5cdsa6iaru2ltb346qotacgvzyrkbyufy75nqlr7p67qd7gbpi", epoch.Subsets[1].String())
		require.Equal(t, "bafyreia424a7ec3dx25r3btpi62cmfpjh2jmfmfrf66253co6epfwznucy", epoch.Subsets[2].String())
		require.Equal(t, "bafyreibioycvipupzfxab26zqf4axb7ghr6rz2q7x4j4ecl26a6ejmwnwe", epoch.Subsets[3].String())
		require.Equal(t, "bafyreif5zhe3ptanpnwfqp6cdie3dy46q3kqrtrpuuprppzrnsoztvmdla", epoch.Subsets[4].String())
		require.Equal(t, "bafyreih636mvxdrcboblum55dl5uhw4tsaj2euyikkwa64oi7aofqw2kuq", epoch.Subsets[5].String())
		require.Equal(t, "bafyreicbm23uvxusj67rsya53j6bc3rovr2ccly3pviljjglp4frypwojm", epoch.Subsets[6].String())
		require.Equal(t, "bafyreifhtknmnxrn6bpvngx3tzcc5hpgm25z6z5ioi3w36rmdrdwyuxhom", epoch.Subsets[7].String())
		require.Equal(t, "bafyreicsi5bepry34caou6fanoh2oqd6z4xerdmgmbnavhlgksaqqyyjha", epoch.Subsets[8].String())
		require.Equal(t, "bafyreiak5ezxvtsyjwpwohebymghgddlkgjbpqkwfhqhsjlcihcn5a33oq", epoch.Subsets[9].String())
		require.Equal(t, "bafyreigcs57a64jrwueug2zinpactvlt5fyq4nldqkhh7shbpixdkmbfha", epoch.Subsets[10].String())
		require.Equal(t, "bafyreigbbnmlyqairftvgpwi7z7pul4mothra7o53b3ythfr2gsdatngra", epoch.Subsets[11].String())
		require.Equal(t, "bafyreifbsq33fzmzyiyy3og73nmtk76vct76d7rcdk24nzfgjuebqtkedi", epoch.Subsets[12].String())
		require.Equal(t, "bafyreiadscov2rht764uwre47mjaltwskpsdjk76bfczkcj7lpmyid4ffi", epoch.Subsets[13].String())
		require.Equal(t, "bafyreid4n3aulssv24u4ffwg6wmyieyjovxhchuj452snuzttibx2vbu4u", epoch.Subsets[14].String())
		require.Equal(t, "bafyreibxeir3ywclsofoo3ar6i2z3kqxljupk3dhwu2gzicdcp27jrewvi", epoch.Subsets[15].String())
		require.Equal(t, "bafyreih6jdnpx6qspzph2ztdnygv44asgq7ec2u3qbczefkom72icb5qxu", epoch.Subsets[16].String())
		require.Equal(t, "bafyreibm4uwn3bsfja6q7fmyhzptj756iuwc5pdeeq62lmzwvsbzld4pzm", epoch.Subsets[17].String())

		{
			// test CBOR encoding
			encoded, err := epoch.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, epoch_raw0, encoded)
		}
	})

	t.Run("classic/1", func(t *testing.T) {
		epoch, err := _DecodeEpochClassic(epoch_raw1)
		require.NoError(t, err)

		require.Equal(t, 4, epoch.Kind)
		require.Equal(t, KindEpoch, Kind(epoch.Kind))
		require.Equal(t, uint64(120), uint64(epoch.Epoch))
		require.Len(t, epoch.Subsets, 2)

		require.Equal(t, "bafyreiczoyhs7u7usregcft534drngud55fei4yzko2wppg5jk4kwmpyv4", epoch.Subsets[0].String())
		require.Equal(t, "bafyreidp6mjjdck4bl6hch57ulwgtgwtwgh3jlj5wsnjwphu3wb5lgseiy", epoch.Subsets[1].String())
	})
	t.Run("fast/1", func(t *testing.T) {
		epoch, err := _DecodeEpochFast(epoch_raw1)
		require.NoError(t, err)

		require.Equal(t, 4, epoch.Kind)
		require.Equal(t, KindEpoch, Kind(epoch.Kind))
		require.Equal(t, uint64(120), uint64(epoch.Epoch))
		require.Len(t, epoch.Subsets, 2)

		require.Equal(t, "bafyreiczoyhs7u7usregcft534drngud55fei4yzko2wppg5jk4kwmpyv4", epoch.Subsets[0].String())
		require.Equal(t, "bafyreidp6mjjdck4bl6hch57ulwgtgwtwgh3jlj5wsnjwphu3wb5lgseiy", epoch.Subsets[1].String())

		{
			// test CBOR encoding
			encoded, err := epoch.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, epoch_raw1, encoded)
		}
	})
}

func BenchmarkEpoch(b *testing.B) {
	b.Run("classic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEpochClassic(epoch_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEpochFast(epoch_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestSubset(t *testing.T) {
	t.Run("classic/0", func(t *testing.T) {
		subset, err := DecodeSubset(subset_raw0)
		require.NoError(t, err)

		require.Equal(t, 3, subset.Kind)
		require.Equal(t, KindSubset, Kind(subset.Kind))
		require.Equal(t, uint64(16848004), uint64(subset.First))
		require.Equal(t, uint64(16880506), uint64(subset.Last))
		require.Len(t, subset.Blocks, 10)

		require.Equal(t, "bafyreiflfrsugma6wuzsyeepa66d52psbv7ihmookmtqq3jxnjwpmrf4xy", subset.Blocks[0].String())
		require.Equal(t, "bafyreibjm6zf3i4fapc7m65oeawdos6rn53lt5vo2pivm7zeq5hfjeisku", subset.Blocks[1].String())
		require.Equal(t, "bafyreihirhmjfwlpoydajhizsuzpznefizv7zj5yoy3maelv6r2v5xj6ja", subset.Blocks[2].String())
		require.Equal(t, "bafyreifwtriqonlvpu4ibuvl5u5rfs7k7geaapehzvf4tcd4mip7ppspwi", subset.Blocks[3].String())
		require.Equal(t, "bafyreicknnm32pye7ryod6t7rbkwa2lyy72xkcuixl7jy2x7v3ro5s6mq4", subset.Blocks[4].String())
		require.Equal(t, "bafyreigwp7n6plerjyiizs3bczewwquuytdlgf7i7asrvawzpwoyxhvrr4", subset.Blocks[5].String())
		require.Equal(t, "bafyreigak3xfyxwqal5vie4xmqz7vu4thkxumxz4pgl27uxfjnh43tlztq", subset.Blocks[6].String())
		require.Equal(t, "bafyreifya6babw7u5mzt5rpdrluaznoh6i7g66lxw4skh7ghpojlklpu6y", subset.Blocks[7].String())
		require.Equal(t, "bafyreigasvrkts2agnvalobin54lyzzvgof7kjca7jmr4xuxhbhf2yt7ke", subset.Blocks[8].String())
		require.Equal(t, "bafyreiddffhmh3o4jjku2gqljukjyc54g5vqmxfsth5hwlmiosc76rdxeq", subset.Blocks[9].String())
	})
	t.Run("fast/0", func(t *testing.T) {
		subset, err := _DecodeSubsetFast(subset_raw0)
		require.NoError(t, err)

		require.Equal(t, 3, subset.Kind)
		require.Equal(t, KindSubset, Kind(subset.Kind))
		require.Equal(t, uint64(16848004), uint64(subset.First))
		require.Equal(t, uint64(16880506), uint64(subset.Last))
		require.Len(t, subset.Blocks, 10)

		require.Equal(t, "bafyreiflfrsugma6wuzsyeepa66d52psbv7ihmookmtqq3jxnjwpmrf4xy", subset.Blocks[0].String())
		require.Equal(t, "bafyreibjm6zf3i4fapc7m65oeawdos6rn53lt5vo2pivm7zeq5hfjeisku", subset.Blocks[1].String())
		require.Equal(t, "bafyreihirhmjfwlpoydajhizsuzpznefizv7zj5yoy3maelv6r2v5xj6ja", subset.Blocks[2].String())
		require.Equal(t, "bafyreifwtriqonlvpu4ibuvl5u5rfs7k7geaapehzvf4tcd4mip7ppspwi", subset.Blocks[3].String())
		require.Equal(t, "bafyreicknnm32pye7ryod6t7rbkwa2lyy72xkcuixl7jy2x7v3ro5s6mq4", subset.Blocks[4].String())
		require.Equal(t, "bafyreigwp7n6plerjyiizs3bczewwquuytdlgf7i7asrvawzpwoyxhvrr4", subset.Blocks[5].String())
		require.Equal(t, "bafyreigak3xfyxwqal5vie4xmqz7vu4thkxumxz4pgl27uxfjnh43tlztq", subset.Blocks[6].String())
		require.Equal(t, "bafyreifya6babw7u5mzt5rpdrluaznoh6i7g66lxw4skh7ghpojlklpu6y", subset.Blocks[7].String())
		require.Equal(t, "bafyreigasvrkts2agnvalobin54lyzzvgof7kjca7jmr4xuxhbhf2yt7ke", subset.Blocks[8].String())
		require.Equal(t, "bafyreiddffhmh3o4jjku2gqljukjyc54g5vqmxfsth5hwlmiosc76rdxeq", subset.Blocks[9].String())

		{
			// test CBOR encoding
			encoded, err := subset.MarshalCBOR()
			require.NoError(t, err)
			spew.Dump(subset.Blocks[0].(cidlink.Link).Cid.Bytes())
			require.Equal(t, subset_raw0, encoded)
		}
	})

	t.Run("classic/1", func(t *testing.T) {
		subset, err := DecodeSubset(subset_raw1)
		require.NoError(t, err)

		require.Equal(t, 3, subset.Kind)
		require.Equal(t, KindSubset, Kind(subset.Kind))
		require.Equal(t, uint64(16880507), uint64(subset.First))
		require.Equal(t, uint64(16905823), uint64(subset.Last))
		require.Len(t, subset.Blocks, 10)

		require.Equal(t, "bafyreig74ql7fhmwocmmngifkcddvmind77ebrxutxm2ig7ad4ybpzpz6y", subset.Blocks[0].String())
		require.Equal(t, "bafyreifkgwft732pcffte272zifhfrpmu3gniuwuzktsne3z3ifg2mmluu", subset.Blocks[1].String())
		require.Equal(t, "bafyreidivk76k7tgyodnkdq4zllljjxfg6cf7iulgnbubfuzdwdtcpdg2i", subset.Blocks[2].String())
		require.Equal(t, "bafyreiet6ht5eamn6hzyliit24zbmr7ewcij5adbrnoxyezclac2uecspy", subset.Blocks[3].String())
		require.Equal(t, "bafyreigaudyx6xsl6fu3csgy5wh62ufrpmnahi4gg5vnzaqggffwkotvxe", subset.Blocks[4].String())
		require.Equal(t, "bafyreifg5rgepvwpmadazghxqwjefbtkhrxdore6sk3so5z5vhfnyfmkv4", subset.Blocks[5].String())
		require.Equal(t, "bafyreies5ctberh7yziovnwh3zvg5se2av3crckbj7dqx5mugizjfralu4", subset.Blocks[6].String())
		require.Equal(t, "bafyreidpt2pqocplw34auzupk2qnvjjlg3eogig2fqsonphvanu5pudyce", subset.Blocks[7].String())
		require.Equal(t, "bafyreigzbzgt3dsb6bm3r5i3carslnjirzlolwyqcmcdwciyqqrkodqo5u", subset.Blocks[8].String())
		require.Equal(t, "bafyreiam4aka6ymgcyylvhap5vuwinumwbded3ktl7qmti2tmprmjd7qh4", subset.Blocks[9].String())
	})
	t.Run("fast/1", func(t *testing.T) {
		subset, err := _DecodeSubsetFast(subset_raw1)
		require.NoError(t, err)

		require.Equal(t, 3, subset.Kind)
		require.Equal(t, KindSubset, Kind(subset.Kind))
		require.Equal(t, uint64(16880507), uint64(subset.First))
		require.Equal(t, uint64(16905823), uint64(subset.Last))
		require.Len(t, subset.Blocks, 10)

		require.Equal(t, "bafyreig74ql7fhmwocmmngifkcddvmind77ebrxutxm2ig7ad4ybpzpz6y", subset.Blocks[0].String())
		require.Equal(t, "bafyreifkgwft732pcffte272zifhfrpmu3gniuwuzktsne3z3ifg2mmluu", subset.Blocks[1].String())
		require.Equal(t, "bafyreidivk76k7tgyodnkdq4zllljjxfg6cf7iulgnbubfuzdwdtcpdg2i", subset.Blocks[2].String())
		require.Equal(t, "bafyreiet6ht5eamn6hzyliit24zbmr7ewcij5adbrnoxyezclac2uecspy", subset.Blocks[3].String())
		require.Equal(t, "bafyreigaudyx6xsl6fu3csgy5wh62ufrpmnahi4gg5vnzaqggffwkotvxe", subset.Blocks[4].String())
		require.Equal(t, "bafyreifg5rgepvwpmadazghxqwjefbtkhrxdore6sk3so5z5vhfnyfmkv4", subset.Blocks[5].String())
		require.Equal(t, "bafyreies5ctberh7yziovnwh3zvg5se2av3crckbj7dqx5mugizjfralu4", subset.Blocks[6].String())
		require.Equal(t, "bafyreidpt2pqocplw34auzupk2qnvjjlg3eogig2fqsonphvanu5pudyce", subset.Blocks[7].String())
		require.Equal(t, "bafyreigzbzgt3dsb6bm3r5i3carslnjirzlolwyqcmcdwciyqqrkodqo5u", subset.Blocks[8].String())
		require.Equal(t, "bafyreiam4aka6ymgcyylvhap5vuwinumwbded3ktl7qmti2tmprmjd7qh4", subset.Blocks[9].String())

		{
			// test CBOR encoding
			encoded, err := subset.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, subset_raw1, encoded)
		}
	})
}

func BenchmarkSubset(b *testing.B) {
	b.Run("classic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeSubsetClassic(subset_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeSubsetFast(subset_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestBlock(t *testing.T) {
	t.Run("classic-fast/0", func(t *testing.T) {
		block, err := _DecodeBlockClassic(block_raw0)
		require.NoError(t, err)

		require.Equal(t, 2, block.Kind)
		require.Equal(t, KindBlock, Kind(block.Kind))
		require.Equal(t, uint64(9), uint64(block.Slot))
		require.Len(t, block.Shredding, 67)
		require.Len(t, block.Entries, 67)

		expectAsJson := `{"kind":2,"slot":9,"shredding":[{"entry_end_idx":0,"shred_end_idx":0},{"entry_end_idx":1,"shred_end_idx":1},{"entry_end_idx":2,"shred_end_idx":2},{"entry_end_idx":3,"shred_end_idx":3},{"entry_end_idx":4,"shred_end_idx":4},{"entry_end_idx":5,"shred_end_idx":5},{"entry_end_idx":6,"shred_end_idx":6},{"entry_end_idx":7,"shred_end_idx":7},{"entry_end_idx":8,"shred_end_idx":8},{"entry_end_idx":9,"shred_end_idx":9},{"entry_end_idx":10,"shred_end_idx":10},{"entry_end_idx":11,"shred_end_idx":11},{"entry_end_idx":12,"shred_end_idx":12},{"entry_end_idx":13,"shred_end_idx":13},{"entry_end_idx":14,"shred_end_idx":14},{"entry_end_idx":15,"shred_end_idx":15},{"entry_end_idx":16,"shred_end_idx":16},{"entry_end_idx":17,"shred_end_idx":17},{"entry_end_idx":18,"shred_end_idx":18},{"entry_end_idx":19,"shred_end_idx":19},{"entry_end_idx":20,"shred_end_idx":20},{"entry_end_idx":21,"shred_end_idx":21},{"entry_end_idx":22,"shred_end_idx":22},{"entry_end_idx":23,"shred_end_idx":23},{"entry_end_idx":24,"shred_end_idx":24},{"entry_end_idx":25,"shred_end_idx":25},{"entry_end_idx":26,"shred_end_idx":26},{"entry_end_idx":27,"shred_end_idx":27},{"entry_end_idx":28,"shred_end_idx":28},{"entry_end_idx":29,"shred_end_idx":29},{"entry_end_idx":30,"shred_end_idx":30},{"entry_end_idx":31,"shred_end_idx":31},{"entry_end_idx":32,"shred_end_idx":32},{"entry_end_idx":33,"shred_end_idx":33},{"entry_end_idx":34,"shred_end_idx":34},{"entry_end_idx":35,"shred_end_idx":35},{"entry_end_idx":36,"shred_end_idx":36},{"entry_end_idx":37,"shred_end_idx":37},{"entry_end_idx":38,"shred_end_idx":38},{"entry_end_idx":39,"shred_end_idx":39},{"entry_end_idx":40,"shred_end_idx":40},{"entry_end_idx":41,"shred_end_idx":41},{"entry_end_idx":42,"shred_end_idx":42},{"entry_end_idx":43,"shred_end_idx":43},{"entry_end_idx":44,"shred_end_idx":44},{"entry_end_idx":45,"shred_end_idx":45},{"entry_end_idx":46,"shred_end_idx":46},{"entry_end_idx":47,"shred_end_idx":47},{"entry_end_idx":48,"shred_end_idx":48},{"entry_end_idx":49,"shred_end_idx":49},{"entry_end_idx":50,"shred_end_idx":50},{"entry_end_idx":51,"shred_end_idx":51},{"entry_end_idx":52,"shred_end_idx":52},{"entry_end_idx":53,"shred_end_idx":53},{"entry_end_idx":54,"shred_end_idx":54},{"entry_end_idx":55,"shred_end_idx":55},{"entry_end_idx":56,"shred_end_idx":56},{"entry_end_idx":57,"shred_end_idx":57},{"entry_end_idx":58,"shred_end_idx":58},{"entry_end_idx":59,"shred_end_idx":59},{"entry_end_idx":60,"shred_end_idx":60},{"entry_end_idx":61,"shred_end_idx":61},{"entry_end_idx":62,"shred_end_idx":62},{"entry_end_idx":63,"shred_end_idx":63},{"entry_end_idx":64,"shred_end_idx":64},{"entry_end_idx":65,"shred_end_idx":65},{"entry_end_idx":66,"shred_end_idx":66}],"entries":[{"\/":"bafyreieoodp5usfhjplhph653vpvkpys24meiyz3kvcoacj653yqoqganu"},{"\/":"bafyreig6fq5m25b736egoexujhstvhejjj5m5im6i6vlfvwr4ogcukfmny"},{"\/":"bafyreif7ll4bmg3nvk2n3gws7lmecdwmuw2esyktbbmtoijuu2f2fzqndy"},{"\/":"bafyreihg5wf2azztumc63bbputua2pgkvd2jus7catewuil6jjc7gprrai"},{"\/":"bafyreibg6p4ugixkbzrpleipuales2gvkg4ghimdkkvck66r6odzcm2bwi"},{"\/":"bafyreibjtw6uxt6xkcupkv5svmwr6dyi24hxq7nq6epj3gwiyufarhubz4"},{"\/":"bafyreif32w32hqgb2ohg2lryfg2zcdlh2s5b4e4v3b4irugrpwzfj7cqxy"},{"\/":"bafyreid6shkgpadven4etav2dygfoddqilui2u5no5lckkbmrcr5t3sjae"},{"\/":"bafyreih5po4uj464jx4mbcnbgclop4jtb5nuwbowe5cqdpq7rfbiuee5be"},{"\/":"bafyreig6flggmcc2swze4a4iorrb3qoddzvo2u6jbv6qpfqqs4cnod4bhq"},{"\/":"bafyreiebcnzlxpjfgbhuawbsiekadoeenals2l6hkugksvilhxr2mcfmgy"},{"\/":"bafyreideleztelgalb22jdn7ic424a7chsjgq7lls4vaqlvwe3kao3vj3q"},{"\/":"bafyreifhxlur26gdsevcbjad7hzgx7beog3qy5iluzqqc3slkibcy73hxi"},{"\/":"bafyreibzc6wgdzu3acufuo7ig44d77joikci7b4bwhb4w63gi4jmtukxvi"},{"\/":"bafyreibughffmlegv4j2cqcxrlk6z3brivcm2ug6fiyol44hkakv455y54"},{"\/":"bafyreiaf64pp5x72gdviltpouljhwo35ctkvhfvvajb5p7ejialg54khdy"},{"\/":"bafyreiev7wcees2rnkxsivemomzkrssa2etkrhuyao56apbkw5yy7rj6im"},{"\/":"bafyreifu4qvhjd44ky2zjeao5gamanrnfmybjmqxfrmchfgllbysmxvzjm"},{"\/":"bafyreiewjsfepwak4ft7pe4mi5tdefc7jdl5ssen6ef76ryuwql6n7e52m"},{"\/":"bafyreihamgkr23zldmeiaygil7n56rrxpabsufs6hkqelq5zjdgrfggnwa"},{"\/":"bafyreibzobcao62dmft6higwkhhp6644xoysczn7qy74qkji2f3myn4qim"},{"\/":"bafyreihoixhqp2lvz4cod6gxul7c2cedcmkjrg47h2hnbjhw3umryylb2e"},{"\/":"bafyreidpibbb7vud5a2ue4abom5msklm2x6p334aipxa4fltflegm3qhem"},{"\/":"bafyreif4vy3jofm64ldrw3idedua2wedetzvwx64po5huqdpr3vyxzaejy"},{"\/":"bafyreif3tbcx2hkk5d3wdl5ewx6ufugyliud3smhben2krmo3lrdg646me"},{"\/":"bafyreihkmldv7wzoqjsexnhesgbo2tv72eeqhql4jz37aaf5r5on657maa"},{"\/":"bafyreigiovd74gb6slvc23jctmt5pc7kj23ytmrapuzkdktx3he5zq6sma"},{"\/":"bafyreicj5m2w5ayimybh7zujgsevlkiixs44dcnbw6dhnrxi2vcwisdo6e"},{"\/":"bafyreicgzp25vmi3qdyyuvfpxarrebetkq7mch4v74f3lqdoqluztejq34"},{"\/":"bafyreib25nv4gcthnl7zzrai3tny3g5kubcbhjyt4hntwfm4zmanelgjqa"},{"\/":"bafyreibk7yeqimsuxxg6xx5oovf75po74xa3lssuoobwexfzcugiaw7xfm"},{"\/":"bafyreifof6beaz5jp25bq3xtfouzmv3nji4khuiolh2k4zvelviaxit7bu"},{"\/":"bafyreiaenycmszslvxu335jrdmuwcfl2l554dpxzgsy3ntsjziru7uedzq"},{"\/":"bafyreih3sz33gnwxckgs2n75fyudybvitqufcypac5vrdmdr3hday3w24u"},{"\/":"bafyreia3hamb6qfhmwiswy6gwbu2vfftbv2hbimbpwamrqbux4gx4zdqsq"},{"\/":"bafyreiarbifnpbwe3btdrtkfkkbkh64avgwqhuzbapxlk6xopbywcrwhmm"},{"\/":"bafyreihnppgpmixhcbn5gd6qmg2awnf56uu7amqcbr6xqnn25fntmejgsu"},{"\/":"bafyreichr2s2yyykdki7zm4vzisqblcwprx6ovdv4p3aognbl3izcozylq"},{"\/":"bafyreifg25gxnp4advt4zk2zvsnl4cz55ra25l42w5rpqgdjdqhgmne2ga"},{"\/":"bafyreib5ogvpbdigppclhr2vzl2kj4cj23xxrxf6vl3o2fnvuxztjejsd4"},{"\/":"bafyreigsayj7yhxe6dmjohxon3uivyvbbvgba4c44fgxbe63kco3hjrafy"},{"\/":"bafyreibuxnha4m432txryctn774jokwcpyl5xqkkko522t2bjx3kmcdtvy"},{"\/":"bafyreifstthml25blp7yppydg63llqzdeokk2wqc2ym4o5dsuzdmgfymuy"},{"\/":"bafyreid3ro3luiymembibkrrfhhmaqjitmibvzwxfmx2z22ky34bji4ehq"},{"\/":"bafyreifhwdqeflexo6sqwydszew45masur6wzjqztvny7qweuokiiemele"},{"\/":"bafyreiglp6anezlnhfjao4qhlms74hofsp3av4gid5uqequr3ch6c7ukru"},{"\/":"bafyreiasbud42ceabhtt3o3w36nexpontqwjj7tg4pg3rh32dwhazxyizy"},{"\/":"bafyreiafvkbyyqrud6mrdc72sszjct6v7hnxrgxqkekjzts7nyviznhnpu"},{"\/":"bafyreigwtxpj4eplp7toqphanp5hncsmvvhdwni3hte52gapbgynkv5k7y"},{"\/":"bafyreie4rged3jfbs6cqeqnvdt5osvkhm7qyr5okkhgsvoms34g46jezle"},{"\/":"bafyreiacpbjrhwrixbwpjoxk4ekq6aiywpxmnbn3dhvnlsre5elsvhs76q"},{"\/":"bafyreih2cksz6jixbynwtlsmhyozfpcxxy6yxdnn77urpodupkxodiv43q"},{"\/":"bafyreicmvzwovcptox7ouux3muwoswa77fua3vnf45lrrqfqrp3qdawm7u"},{"\/":"bafyreidpmpgka22nsaridfhbjma6sim6p4idlfopogmz3lvuuajckfxfbq"},{"\/":"bafyreidtwlxjgbvkiahfwjmmn3j3xhn5uk74nrwbprdzujd2et2l26c5sq"},{"\/":"bafyreibbqmxznuft3ipsqtmni2dfgoygn4scx7db3jgzkgzhpjn35fb4ci"},{"\/":"bafyreihdkmlk6djhf5jqp6bnhssykb5jenxih34f2vpmhs5bvwqkeeskqy"},{"\/":"bafyreidy7y3fcoborpylvodseyscmf5mixp65lz3ulq7eqggv6byjxe4oy"},{"\/":"bafyreiaxrj3ewromgsyui7nkhl3d6sfyzb23cmziumsnmr3gicwke5ixg4"},{"\/":"bafyreiecvcq3na6immh4k7pj3hemx5rhzfz23zmap7qi6sjr56iuonqybe"},{"\/":"bafyreidmzimbgattikq6jyen564kun5prw4f3lgc37zaxorvtrmye7l7ei"},{"\/":"bafyreifqpkxq64gkdafnz3f4ho4qdg5ovb4vdn3k5t6b47gcaqmik4yj54"},{"\/":"bafyreibm6i3b4fhlmfxc2z7nd5x5ymyca4o7wdhhrewagnnhx3kpqfjacq"},{"\/":"bafyreie3coibkka6uf25cngpc2ovtdexdiaanlicgxnzterkfxpewah6oq"},{"\/":"bafyreifht6ibzxoiavte6n7jawykptmyiyzcauxi4asfb2wlzxk2s3dru4"},{"\/":"bafyreihift7rpktl5nlkaw6xlc6w6oprsu5eu2epifocr5yw2zrcrlhjma"},{"\/":"bafyreibyqwttnacsgvt4xn3y4e7nrraqnadoy5c5qwvoilccib4jr4f5ji"}],"meta":{"parent_slot":8,"blocktime":0,"block_height":null},"rewards":{"\/":"bafkqaaa"}}`
		gotAsJson, err := json.Marshal(block)
		require.NoError(t, err)
		require.JSONEq(t, expectAsJson, string(gotAsJson))

		{
			fastBlock, err := _DecodeBlockFast(block_raw0)
			require.NoError(t, err)
			// compare with slow, field by field
			require.Equal(t, block.Kind, fastBlock.Kind)
			require.Equal(t, block.Slot, fastBlock.Slot)
			require.Equal(t, block.Shredding, fastBlock.Shredding)
			require.Equal(t, block.Entries, fastBlock.Entries)
			{
				require.Equal(t, block.Meta.Parent_slot, fastBlock.Meta.Parent_slot)
				require.Equal(t, block.Meta.Blocktime, fastBlock.Meta.Blocktime)
				{
					classicMetaBlockHeight, okClassic := block.Meta.GetBlockHeight()
					fastMetaBlockHeight, okFast := fastBlock.Meta.GetBlockHeight()
					require.Equal(t, okClassic, okFast)
					if okClassic {
						require.Equal(t, classicMetaBlockHeight, fastMetaBlockHeight)
					}
				}
			}
			require.Equal(t, block.Rewards, fastBlock.Rewards)
		}

		{
			// test CBOR encoding
			encoded, err := block.MarshalCBOR()
			require.NoError(t, err)
			// require.Equal(t, block_raw0, encoded)
			{
				// let's test as value equivalence instead of comparing bytes
				decoded, err := DecodeBlock(encoded)
				require.NoError(t, err)
				// compare field by field so it's easier to debug
				require.EqualValues(t, block.Kind, decoded.Kind)
				require.EqualValues(t, block.Slot, decoded.Slot)
				require.EqualValues(t, block.Shredding, decoded.Shredding)
				require.EqualValues(t, block.Entries, decoded.Entries)
				{
					blockheight1, ok1 := block.Meta.GetBlockHeight()
					blockheight2, ok2 := decoded.Meta.GetBlockHeight()
					require.EqualValues(t, blockheight1, blockheight2)
					require.EqualValues(t, ok1, ok2)

					require.EqualValues(t, block.Meta.Blocktime, decoded.Meta.Blocktime)
					require.EqualValues(t, block.Meta.Parent_slot, decoded.Meta.Parent_slot)
					require.True(t, block.Meta.Equivalent(decoded.Meta))
				}
				require.EqualValues(t, block.Rewards, decoded.Rewards)
				// require.EqualValues(t, block, decoded)
				{
					// now compare as json
					blockJson, err := json.Marshal(block)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(blockJson), string(decodedJson))
				}
			}
		}
	})
	t.Run("classic-fast/1", func(t *testing.T) {
		block, err := _DecodeBlockClassic(block_raw1)
		require.NoError(t, err)

		require.Equal(t, 2, block.Kind)
		require.Equal(t, KindBlock, Kind(block.Kind))
		require.Equal(t, uint64(16848018), uint64(block.Slot))
		require.Len(t, block.Shredding, 85)
		require.Len(t, block.Entries, 85)

		expectAsJson := `{"kind":2,"slot":16848018,"shredding":[{"entry_end_idx":0,"shred_end_idx":0},{"entry_end_idx":1,"shred_end_idx":1},{"entry_end_idx":2,"shred_end_idx":2},{"entry_end_idx":3,"shred_end_idx":3},{"entry_end_idx":4,"shred_end_idx":4},{"entry_end_idx":5,"shred_end_idx":5},{"entry_end_idx":6,"shred_end_idx":6},{"entry_end_idx":7,"shred_end_idx":7},{"entry_end_idx":8,"shred_end_idx":8},{"entry_end_idx":9,"shred_end_idx":9},{"entry_end_idx":10,"shred_end_idx":10},{"entry_end_idx":11,"shred_end_idx":11},{"entry_end_idx":12,"shred_end_idx":12},{"entry_end_idx":13,"shred_end_idx":13},{"entry_end_idx":14,"shred_end_idx":14},{"entry_end_idx":15,"shred_end_idx":-1},{"entry_end_idx":16,"shred_end_idx":-1},{"entry_end_idx":17,"shred_end_idx":-1},{"entry_end_idx":18,"shred_end_idx":-1},{"entry_end_idx":19,"shred_end_idx":15},{"entry_end_idx":20,"shred_end_idx":16},{"entry_end_idx":21,"shred_end_idx":17},{"entry_end_idx":22,"shred_end_idx":18},{"entry_end_idx":23,"shred_end_idx":19},{"entry_end_idx":24,"shred_end_idx":20},{"entry_end_idx":25,"shred_end_idx":21},{"entry_end_idx":26,"shred_end_idx":22},{"entry_end_idx":27,"shred_end_idx":23},{"entry_end_idx":28,"shred_end_idx":24},{"entry_end_idx":29,"shred_end_idx":25},{"entry_end_idx":30,"shred_end_idx":26},{"entry_end_idx":31,"shred_end_idx":27},{"entry_end_idx":32,"shred_end_idx":28},{"entry_end_idx":33,"shred_end_idx":29},{"entry_end_idx":34,"shred_end_idx":30},{"entry_end_idx":35,"shred_end_idx":-1},{"entry_end_idx":36,"shred_end_idx":31},{"entry_end_idx":37,"shred_end_idx":-1},{"entry_end_idx":38,"shred_end_idx":-1},{"entry_end_idx":39,"shred_end_idx":-1},{"entry_end_idx":40,"shred_end_idx":-1},{"entry_end_idx":41,"shred_end_idx":-1},{"entry_end_idx":42,"shred_end_idx":-1},{"entry_end_idx":43,"shred_end_idx":-1},{"entry_end_idx":44,"shred_end_idx":34},{"entry_end_idx":45,"shred_end_idx":-1},{"entry_end_idx":46,"shred_end_idx":-1},{"entry_end_idx":47,"shred_end_idx":-1},{"entry_end_idx":48,"shred_end_idx":-1},{"entry_end_idx":49,"shred_end_idx":-1},{"entry_end_idx":50,"shred_end_idx":-1},{"entry_end_idx":51,"shred_end_idx":-1},{"entry_end_idx":52,"shred_end_idx":37},{"entry_end_idx":53,"shred_end_idx":-1},{"entry_end_idx":54,"shred_end_idx":-1},{"entry_end_idx":55,"shred_end_idx":-1},{"entry_end_idx":56,"shred_end_idx":39},{"entry_end_idx":57,"shred_end_idx":40},{"entry_end_idx":58,"shred_end_idx":41},{"entry_end_idx":59,"shred_end_idx":42},{"entry_end_idx":60,"shred_end_idx":43},{"entry_end_idx":61,"shred_end_idx":44},{"entry_end_idx":62,"shred_end_idx":45},{"entry_end_idx":63,"shred_end_idx":-1},{"entry_end_idx":64,"shred_end_idx":46},{"entry_end_idx":65,"shred_end_idx":47},{"entry_end_idx":66,"shred_end_idx":48},{"entry_end_idx":67,"shred_end_idx":49},{"entry_end_idx":68,"shred_end_idx":50},{"entry_end_idx":69,"shred_end_idx":51},{"entry_end_idx":70,"shred_end_idx":52},{"entry_end_idx":71,"shred_end_idx":53},{"entry_end_idx":72,"shred_end_idx":54},{"entry_end_idx":73,"shred_end_idx":55},{"entry_end_idx":74,"shred_end_idx":56},{"entry_end_idx":75,"shred_end_idx":57},{"entry_end_idx":76,"shred_end_idx":58},{"entry_end_idx":77,"shred_end_idx":59},{"entry_end_idx":78,"shred_end_idx":60},{"entry_end_idx":79,"shred_end_idx":61},{"entry_end_idx":80,"shred_end_idx":62},{"entry_end_idx":81,"shred_end_idx":63},{"entry_end_idx":82,"shred_end_idx":64},{"entry_end_idx":83,"shred_end_idx":65},{"entry_end_idx":84,"shred_end_idx":66}],"entries":[{"/":"bafyreid2pnfheom5ewa7mazfg3dueqg7f3aayter4jp7f7rc4cvzyeqc5e"},{"/":"bafyreig7y6dwlmqz4jukrulh4wguhwgq75rs63kn376gjon255inyx4sjm"},{"/":"bafyreiew7mg2npzkuy6mpnkzaap5zmgzpmp2cokhnbmhog2omln4hxbvqi"},{"/":"bafyreic4pbfo2zeizxh4duzvarpp5fqfu6hkipdeqzjugc3pjqsbp6hauy"},{"/":"bafyreiaphvmkua6oh65tszmqvirrzzqaaednzqgtj7t225h4aq42j2lv3q"},{"/":"bafyreiciqm6czupeogkyleki42qy6pizphm7qdcdbb5m7osd3yn2xu46om"},{"/":"bafyreiagi2mmtzwtogu7akfdx3dobckmmudrwwtdva4v7tp7d2gwbszmtq"},{"/":"bafyreibtewsoti2exf2t2j32tprl5ayolrejc3ig6ecb5ikysm3fy2rllu"},{"/":"bafyreib3mlzeouportbpblkcbclpguhex77p2s754logx6tm2ia7djglue"},{"/":"bafyreiann4uwj6lcndwt5a43wpnmxmsz6ijvqdrqx57djlje5ydx3fbz5i"},{"/":"bafyreidcz2einavxee7o34ik3kf66btel5ugjimkutaa2rszjcro2dqt3i"},{"/":"bafyreigvw2gsd5mne7l3wjbscenfkhusyfj3knsjol7pwfufh5kmvvcu2a"},{"/":"bafyreia2qbtzdsjhlnk6anxtshhpittir6xoxxn22mv7f3r5nhx5r56bry"},{"/":"bafyreicsb5j6fmxhhryhgshfo3z3l2mcgi6fb6eaph7anfvdhakqywkdwm"},{"/":"bafyreidykikh4pavbjxlxqsv5fupdorb7nrqv4xeeyhhajcz7nm3b7r4l4"},{"/":"bafyreibsqxrex6y55t5nbwbgswpk2t3xigo7tns4jvw5gbfpcqlck3h4ky"},{"/":"bafyreib2elshkyv54ljn5552slpkm3cv5r5nrvgdm5kjw7jdahm2z4o54e"},{"/":"bafyreidfz5uamlso2b3cr6rmkbvtrvnovesm5fia7uqnphhjgaduvwm4ne"},{"/":"bafyreiaat3nsoscfbdvfrkqnwii2c6sglcy6sfmooxhuwihjkrzzzclofe"},{"/":"bafyreiczg4tsctu5h6tzcrbmuuzedibwxrnhnnvu2t6zqiv32agd5ihmuu"},{"/":"bafyreia5vj25jkczh2hpce4uuvwvmyfuvjecgrmpvov5xqivyfoiwx4lfm"},{"/":"bafyreihdtes3sjuuv33cocgtgemy7n65z6uqqtbu2tujzpcfqtws6qle7u"},{"/":"bafyreieqc72qh3cvoghvloqlgisyelntrd4os64skj7saxbe55p7fxblym"},{"/":"bafyreidanjusj2qb7rogbdnn647lvqz7acljkyzpja76hvctnex7vfkyvy"},{"/":"bafyreiflvro2pygpwqt5onvg7j4bb5eedqetwb6yibpjw2z2n6upt7bx4y"},{"/":"bafyreia7rzquteo3zoy4xmydgzhggo4xi6wuccsqpe4cb6jgtr425itnoq"},{"/":"bafyreibt665vuyili5c7nia3dlwjupstrmohxqxjvcgloz7uuau4xntjsa"},{"/":"bafyreifhnjejjihvbnjhkshyle6rsvib4q7ss6jis37ozieivtolyw6qo4"},{"/":"bafyreiegcjkyvmkdokt7mjp6ukes7mr5byz3sxbcy5kvk4kzq5wgjhmuwu"},{"/":"bafyreigjumeksxuv4u3stmv3uis2zjcgt7dwsltge5vspdd52sitj6l35e"},{"/":"bafyreifn5x2qbrzbckslrdtzrj5ds4v23fa7gb4ehq7jhgfqckqkvvyfb4"},{"/":"bafyreievizxgpsppfbsbamcpoifcuhbjgxn3k5bzdatvfpcowdpenlwxf4"},{"/":"bafyreid7mmrvrvkia72td4fyldl4uu3jd74vnavracds3pcfvtrd7x2c64"},{"/":"bafyreibgldedgli2s3stggcqoefhltedouovhghmfji3mll7ud7qjhiywq"},{"/":"bafyreibvs3mvadlu73ux2n7vt2wwiwkyjmuphrdt643naxn545kebpx6zq"},{"/":"bafyreiehz75y66w6x4kgtbjivlgcbnyldscum7x3j3bx5x6aazojgdvmxy"},{"/":"bafyreifci7vratpgbegljwdulrbyz2quxv54tc27rsjinel5xnplfgbugi"},{"/":"bafyreif37mblyrz2lixaxjlh6btsmzeumun2pkryp4dj44tixso73wctxi"},{"/":"bafyreictaj5yyfskmm3c4uvjunr7ubodjwuaheqqhmjrwjsvrie4g6sszy"},{"/":"bafyreigwlamsurb37gojtvbhyc3hpvxes4rlqzhssxob57n3txe36xurw4"},{"/":"bafyreihcfokx26kybzc4lykrl6ghodt57qjebtzroky345nicgqdffbzlu"},{"/":"bafyreib4eadify34mg2h7ngj3hnxuwd5rpz6gnfdoek2lvfxfgdnpwebki"},{"/":"bafyreicwsogpjtmz6vo5hb7igwxxboekcb6sqxffagqryq5qfpzkg6g2nu"},{"/":"bafyreie4j4ehiyzuyjdfe3xlhnwsb4yrr72tohbxwbg4zryro6q73lwvx4"},{"/":"bafyreibcjb42puzvoy5e7xqn6yncyk7cb2zch67p2fflimvrzclpb6ayb4"},{"/":"bafyreig2tnsvrdb6v6lhtej7eby7rgsxjepdl3a7sphwhoejaesj3om5je"},{"/":"bafyreihxnmudf353tkgwjbgd3dsbvofmkfi5y6mzso7r6dsojmjpmsfrxy"},{"/":"bafyreie3lqjtnydo7vhuaozifkmbedkky4ndjvoxzmn2cfrrs74qmv2w2i"},{"/":"bafyreiad2b6qarttppzumqpa5jukz7d5jydjjozgjwudxzxkpoovpthqty"},{"/":"bafyreibucarddrqxfwdsyty7bspd5ao6fklayfqdvnxbcr3acekzwlxpae"},{"/":"bafyreigo2v3eb6eagg4sif4x4ybf5dejsnwcbbxj35wlocapideupatxn4"},{"/":"bafyreiap7aneqnbjlpcrnkgpssje7ils3t5pxllhk2blhxzk7w5zja364u"},{"/":"bafyreihnfrxdjjvhugjrfmswusas4atdof2tn3toiiniederizfiuqvkve"},{"/":"bafyreiaivbjsaegwjasysaxgslby3ck47qqfo43kseda4yrk7uoxpu2w7a"},{"/":"bafyreieghxibplszxzqi3xa735a3z7nisn767yrn2eo2dg3hxiebshwbwa"},{"/":"bafyreidbktchhrmytkmp4dmabrsqbzyrqttklzhvohac55rx6pqmoqjnvq"},{"/":"bafyreiaabogocc6h6abi4otsblke24l7v5kqxieb7k2wjsuht74jlwygyi"},{"/":"bafyreifm3cg2ukuxmpabbgnpw62zgurzmuo64plqxk6dnbdgsyxka4cpda"},{"/":"bafyreidmmqpdfn46cravwep2p4xusv2a2p6nahp27ztsaxofw3pxc3ehzu"},{"/":"bafyreiatuacmtj4fyvommecyiydidlkj4tcftxpofaehrzekuic2c5g4fa"},{"/":"bafyreidgvapadmwg2lvvyb2acad65iiav2vpvzrbxvqyasaf3fndfzqwxe"},{"/":"bafyreiazoctpsemmhlosv4wpwuvclboli7zpnkgmaszabzh2vtvyn6dbbe"},{"/":"bafyreianzo76ejvoyq7xqgqkyu75brplhmp75jltdnuubyewribzcflbbi"},{"/":"bafyreihk6vnuqdr5bld2qb7xvzfi4sjbhysiiyupzct3kp7upbusmiuvda"},{"/":"bafyreiaixtqyox5ymhk4gkkxghrrmbpy7ffimrmlngazwikto6jdsypm6u"},{"/":"bafyreiayhwy2yn2g7rgc65pnn3qqku4bsutrgnqmvngei4owcuq5ckfp54"},{"/":"bafyreidpczdtysbbucuv2ku6japqrcnkmpqxxlvpkxeecunp3q63hna74m"},{"/":"bafyreido7i7n5jkhjfx5grdaadmmevpp2ifxxx6y4avvjncoxd4kevsshy"},{"/":"bafyreidvv7mckn4cohucqda6ckrm667uuc4xwq5jlzxjghaulp33sgsxzu"},{"/":"bafyreidfguqbpqdnnstpkmxoqji7wgvzxdf4uxigxuspdhcgb5vqqdswdm"},{"/":"bafyreidzveienjrmf4hgmfyp6wzreazct5jns35iehsfjacfmehsq32da4"},{"/":"bafyreib7pcqbtffuz3v5smdh7uuo2ojt4tckwxf2geg3h7c5ml4tchi644"},{"/":"bafyreigc4qema3s24z2axrixraxmtwkfzlr54sqsuwqnzijpjx3ska2edm"},{"/":"bafyreie56qvcygvbyj4h6g6t3yvoar2d6of2t5lssxcvospr23mteo4tce"},{"/":"bafyreihv3az3lzekpadegmafcgn6jhpcw4awgg7ypkreuii6jts2df22y4"},{"/":"bafyreiain2cfmrvbczxbp5v4maoq5prizjvzpv57n7mcu3a36tnnzrinvm"},{"/":"bafyreievbheznnf3r6d4naswfywdgu52i6qgemofmvf2tmi3k6ohhxuxaq"},{"/":"bafyreihva4zfqiyjk7uh62w3ma66ngedut534ntjbl5y5q4qopl6c3dn3a"},{"/":"bafyreicdqu73vveyet5vz2fmn3tnb3q3sd75r6jtovx7spg5spbybbs7le"},{"/":"bafyreibxlrsjknuaxngadlngvjf5zgawpkppib4akspo3oz2xbsm5wgwwi"},{"/":"bafyreihtiusstwldx2hoek7fzq7olwdghxkaymvndai52nrtyvqh7ba6wy"},{"/":"bafyreicnvrum2uoe2dr3igr4fxgpztlqpga2bhrsyjpd67gp273smbt2fa"},{"/":"bafyreidzstejdvm4dytzjbbipvn4ojgyvw52aobzyvu5uwbmey2gi2tqua"},{"/":"bafyreifffo5boyjwyx4pr64ium2gp7fyvxavob4fjr4n2pzb46232tcyme"},{"/":"bafyreigmpuij5pyqpialgwy7umnoma2rtjzsamqhj6xzm2drxsapayzrj4"}],"meta":{"parent_slot":16848017,"blocktime":0,"block_height":null},"rewards":{"/":"bafkqaaa"}}`
		gotAsJson, err := json.Marshal(block)
		require.NoError(t, err)
		require.JSONEq(t, expectAsJson, string(gotAsJson))

		{
			fastBlock, err := _DecodeBlockFast(block_raw1)
			require.NoError(t, err)
			// compare with slow, field by field
			require.Equal(t, block.Kind, fastBlock.Kind)
			require.Equal(t, block.Slot, fastBlock.Slot)
			require.Equal(t, block.Shredding, fastBlock.Shredding)
			require.Equal(t, block.Entries, fastBlock.Entries)
			{
				require.Equal(t, block.Meta.Parent_slot, fastBlock.Meta.Parent_slot)
				require.Equal(t, block.Meta.Blocktime, fastBlock.Meta.Blocktime)
				{
					classicMetaBlockHeight, okClassic := block.Meta.GetBlockHeight()
					fastMetaBlockHeight, okFast := fastBlock.Meta.GetBlockHeight()
					require.Equal(t, okClassic, okFast)
					if okClassic {
						require.Equal(t, classicMetaBlockHeight, fastMetaBlockHeight)
					}
				}
			}
			require.Equal(t, block.Rewards, fastBlock.Rewards)
		}

		{
			// test CBOR encoding
			encoded, err := block.MarshalCBOR()
			require.NoError(t, err)
			// require.Equal(t, block_raw1, encoded)
			{
				// let's test as value equivalence instead of comparing bytes
				decoded, err := DecodeBlock(encoded)
				require.NoError(t, err)
				// compare field by field so it's easier to debug
				require.EqualValues(t, block.Kind, decoded.Kind)
				require.EqualValues(t, block.Slot, decoded.Slot)
				require.EqualValues(t, block.Shredding, decoded.Shredding)
				require.EqualValues(t, block.Entries, decoded.Entries)
				{
					blockheight1, ok1 := block.Meta.GetBlockHeight()
					blockheight2, ok2 := decoded.Meta.GetBlockHeight()
					require.EqualValues(t, blockheight1, blockheight2)
					require.EqualValues(t, ok1, ok2)

					require.EqualValues(t, block.Meta.Blocktime, decoded.Meta.Blocktime)
					require.EqualValues(t, block.Meta.Parent_slot, decoded.Meta.Parent_slot)
					require.True(t, block.Meta.Equivalent(decoded.Meta))
				}
				require.EqualValues(t, block.Rewards, decoded.Rewards)
				// require.EqualValues(t, block, decoded)
				{
					// now compare as json
					blockJson, err := json.Marshal(block)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(blockJson), string(decodedJson))
				}
			}
		}
	})
}

func BenchmarkBlock(b *testing.B) {
	b.Run("classic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeBlockClassic(block_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeBlockFast(block_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func compareRewards(t *testing.T, raw []byte, expectAsJson string) {
	rewards, err := _DecodeRewardsClassic(raw)
	require.NoError(t, err)

	gotAsJson, err := json.Marshal(rewards)
	require.NoError(t, err)
	require.JSONEq(t, expectAsJson, string(gotAsJson))

	{
		fastRewards, err := _DecodeRewardsFast(raw)
		require.NoError(t, err)
		// compare with slow, field by field
		require.Equal(t, rewards.Kind, fastRewards.Kind)
		require.Equal(t, rewards.Slot, fastRewards.Slot)
		compareDataFrame_simple(t, rewards.Data, fastRewards.Data)
	}

	{
		// test CBOR encoding
		encoded, err := rewards.MarshalCBOR()
		require.NoError(t, err)
		require.Equal(t, raw, encoded)
		{
			// let's test as value equivalence instead of comparing bytes
			decoded, err := DecodeRewards(encoded)
			require.NoError(t, err)
			// compare field by field so it's easier to debug
			require.EqualValues(t, rewards.Kind, decoded.Kind)
			require.EqualValues(t, rewards.Slot, decoded.Slot)
			compareDataFrame_simple(t, rewards.Data, decoded.Data)

			// require.EqualValues(t, rewards, decoded)
			{
				// now compare as json
				rewardsJson, err := json.Marshal(rewards)
				require.NoError(t, err)
				decodedJson, err := json.Marshal(decoded)
				require.NoError(t, err)
				require.JSONEq(t, string(rewardsJson), string(decodedJson))
			}
		}
	}
}

func TestRewards(t *testing.T) {
	t.Run("classic-fast/0", func(t *testing.T) {
		expectAsJson := `{"kind":5,"slot":16848004,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAQQAAAAAAAAAAAAC7G9vK","next":null}}`
		compareRewards(t, rewards_raw0, expectAsJson)
	})
	t.Run("classic-fast/1", func(t *testing.T) {
		expectAsJson := `{"kind":5,"slot":16848004,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAQQAAAAAAAAAAAAC7G9vK","next":null}}`
		compareRewards(t, rewards_raw1, expectAsJson)
	})
}

func BenchmarkRewards(b *testing.B) {
	b.Run("classic", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeRewardsClassic(rewards_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeRewardsFast(rewards_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func comprareEntry(t *testing.T, raw []byte, expectAsJson string) {
	entry, err := _DecodeEntryClassic(raw)
	require.NoError(t, err)

	gotAsJson, err := json.Marshal(entry)
	require.NoError(t, err)
	require.JSONEq(t, expectAsJson, string(gotAsJson))

	{
		fastEntry, err := _DecodeEntryFast(raw)
		require.NoError(t, err)
		// compare with slow, field by field
		require.Equal(t, entry.Hash, fastEntry.Hash)
		require.Equal(t, entry.Kind, fastEntry.Kind)
		require.Equal(t, entry.NumHashes, fastEntry.NumHashes)
		require.Equal(t, entry.Transactions, fastEntry.Transactions)
	}

	{
		// test CBOR encoding
		encoded, err := entry.MarshalCBOR()
		require.NoError(t, err)
		require.Equal(t, raw, encoded)
		{
			// let's test as value equivalence instead of comparing bytes
			decoded, err := DecodeEntry(encoded)
			require.NoError(t, err)
			// compare field by field so it's easier to debug
			require.EqualValues(t, entry.Hash, decoded.Hash)
			require.EqualValues(t, entry.Kind, decoded.Kind)
			require.EqualValues(t, entry.NumHashes, decoded.NumHashes)
			require.EqualValues(t, entry.Transactions, decoded.Transactions)

			// require.EqualValues(t, entry, decoded)
			{
				// now compare as json
				entryJson, err := json.Marshal(entry)
				require.NoError(t, err)
				decodedJson, err := json.Marshal(decoded)
				require.NoError(t, err)
				require.JSONEq(t, string(entryJson), string(decodedJson))
			}
		}
	}
}

func TestEntry(t *testing.T) {
	t.Run("classic-fast/0", func(t *testing.T) {
		expectAsJson := `{"kind":1,"num_hashes":12500,"hash":"3a43cd82e140873740fde924da4125ac30e2fec5eb92344dbb2bb4776973feec","transactions":null}`
		comprareEntry(t, entry_raw0, expectAsJson)
	})
	t.Run("classic-fast/1", func(t *testing.T) {
		expectAsJson := `{"kind":1,"num_hashes":12500,"hash":"b12c324e55fb861ce6ef0d315ed3115bea52f6bec83cf09c9872c70de69fdfea","transactions":null}`
		comprareEntry(t, entry_raw1, expectAsJson)
	})
	t.Run("classic-fast/2", func(t *testing.T) {
		expectAsJson := `{"kind":1,"num_hashes":12500,"hash":"475c39d0431d1479a35fa3499e0a8dd6e472254f5f734408a896a9fda5219995","transactions":null}`
		comprareEntry(t, entry_raw2, expectAsJson)
	})
	t.Run("classic-fast/3", func(t *testing.T) {
		expectAsJson := `{"kind":1,"num_hashes":12179,"hash":"87b3f95ad785a5e8c7b5ffae44b37c200c27d5464870545489560c217a48d798","transactions":[{"/":"bafyreibysst7x3lvzdrllbspoob5z2epcrb6bmzqqlcxxysvku4cmvdk4e"}]}`
		comprareEntry(t, entry_raw3, expectAsJson)
	})
}

func BenchmarkEntry(b *testing.B) {
	b.Run("classic/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryClassic(entry_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryFast(entry_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryClassic(entry_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryFast(entry_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryClassic(entry_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryFast(entry_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryClassic(entry_raw3)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeEntryFast(entry_raw3)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func compareDataFrame_simple(t *testing.T, a ipldbindcode.DataFrame, b ipldbindcode.DataFrame) {
	require.Equal(t, a.Kind, b.Kind)
	{
		classicDataHash, okClassic := a.GetHash()
		fastDataHash, okFast := b.GetHash()
		require.Equal(t, okClassic, okFast)
		if okClassic {
			require.Equal(t, classicDataHash, fastDataHash)
		}
	}
	{
		classicDataIndex, okClassic := a.GetIndex()
		fastDataIndex, okFast := b.GetIndex()
		require.Equal(t, okClassic, okFast)
		if okClassic {
			require.Equal(t, classicDataIndex, fastDataIndex)
		}
	}
	{
		classicDataTotal, okClassic := a.GetTotal()
		fastDataTotal, okFast := b.GetTotal()
		require.Equal(t, okClassic, okFast)
		if okClassic {
			require.Equal(t, classicDataTotal, fastDataTotal)
		}
	}
	require.Equal(t, a.Data, b.Data)
	{
		classicDataNext, okClassic := a.GetNext()
		fastDataNext, okFast := b.GetNext()
		require.Equal(t, okClassic, okFast)
		if okClassic {
			require.Equal(t, classicDataNext, fastDataNext)
		}
	}
}

func compareTransaction(t *testing.T, raw []byte, expectAsJson string) {
	transaction, err := _DecodeTransactionClassic(raw)
	require.NoError(t, err)

	gotAsJson, err := json.Marshal(transaction)
	require.NoError(t, err)
	require.JSONEq(t, expectAsJson, string(gotAsJson))

	{
		fastTransaction, err := _DecodeTransactionFast(raw)
		require.NoError(t, err)
		// compare with slow, field by field
		require.Equal(t, transaction.Kind, fastTransaction.Kind)
		compareDataFrame_simple(t, transaction.Data, fastTransaction.Data)
		compareDataFrame_simple(t, transaction.Metadata, fastTransaction.Metadata)

		require.Equal(t, transaction.Slot, fastTransaction.Slot)
		classicIndex, okClassic := transaction.GetPositionIndex()
		fastIndex, okFast := fastTransaction.GetPositionIndex()
		require.Equal(t, okClassic, okFast)
		if okClassic {
			require.Equal(t, classicIndex, fastIndex)
		}
	}

	{
		// test CBOR encoding
		encoded, err := transaction.MarshalCBOR()
		require.NoError(t, err)
		require.Equal(t, raw, encoded)
		{
			// let's test as value equivalence instead of comparing bytes
			decoded, err := DecodeTransaction(encoded)
			require.NoError(t, err)
			// compare field by field so it's easier to debug
			require.EqualValues(t, transaction.Kind, decoded.Kind)
			compareDataFrame_simple(t, transaction.Data, decoded.Data)
			compareDataFrame_simple(t, transaction.Metadata, decoded.Metadata)
			require.EqualValues(t, transaction.Slot, decoded.Slot)
			{
				classicIndex, okClassic := transaction.GetPositionIndex()
				fastIndex, okFast := decoded.GetPositionIndex()
				require.EqualValues(t, okClassic, okFast)
				if okClassic {
					require.EqualValues(t, classicIndex, fastIndex)
				}

				// require.EqualValues(t, transaction, decoded)
				{
					// now compare as json
					transactionJson, err := json.Marshal(transaction)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(transactionJson), string(decodedJson))
				}
			}
		}
	}
}

func TestTransaction(t *testing.T) {
	t.Run("classic-fast/0", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AYbTMUdKwOfLPFey+AwyctaBtizbmzA4GiKpHwj+4ZrfKJu+xyl67fjZA6Nn1P8bg57V3OnuZVmUWyx8eSIdEwgBAAMFBRm4eNZlQLMYzIafIkHEG3bCnw0fIZY+Zqt/itnGLqcFGbhso5XTeMn5AgdGOiWLQlHMPlUD7ru2OG1kkuQjSgan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAALY8zyGeltaQlaJeQ5wMCwZM8BOX2PZ5LVgiytnw6PELAQQEAQIDAD0CAAAAAgAAAAAAAAB9FAEBAAAAAH4UAQEAAAAA8qsHs5MMwvaTJoc++kGCUvyGn9od2r8SeheTKCk1uFgA","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAdQEAIkIHENBHAT890iif/RN6KSuP8n1gnL2lhV4OEer9wXwFAKd6CDJeQdbOHGooX+3txOI=","next":null},"slot":16848004,"index":0}`
		compareTransaction(t, transaction_raw0, expectAsJson)
	})
	t.Run("classic-fast/1", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AZefWbthGY4DrlWddGbFstb2SuKNH2kQJUNp4Y3+XOBlXUP77qnjOShtgsRvJqS+j4rJ7ZsBT1Eex7QuV+D0KAoBAAMFrBYKcNpllQ32WLoMCd2PaL1ByibWi05UEFONRtCO9tN/rqFhq+q8I5Y2Z+0JFrZ3yFicOGyV+ehkL4SjrHfiJQan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAAATJHdRQdragJfvZNTXpGfb842WXhg6U+rPInif8dK4lAQQEAQIDADUCAAAAAQAAAAAAAAB/FAEBAAAAACKM6438Q48QuYhCbMm6BPD6WjPg3sRS8h5u6TFuxDFtAA==","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAfQEAIoIHEeBJAYD6rVKuC6pKHZFBMb4PCp29nWQ+Alni/aB8BQCneggyXkHWzhxqKF8Qe9xm","next":null},"slot":16848004,"index":6}`
		compareTransaction(t, transaction_raw1, expectAsJson)
	})
	t.Run("classic-fast/2", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AU04JgfCwBzeM10luGumC6Mnx8IWiO5qhvHS5m9ShDo5tvVnFNR/iM9WTpEsX8KWNLQ6FjwmdzOtwZVlJDJQNQ4BAAMFvkZkGP0en25Qmgvlhgth8IBmoux3dFHeyEEWaMD4BCTuT+i3rh8B6b/JqzN6SbgKY9oBR02IwOL0BAUppaUrsQan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAALY8zyGeltaQlaJeQ5wMCwZM8BOX2PZ5LVgiytnw6PELAQQEAQIDAD0CAAAAAgAAAAAAAAB9FAEBAAAAAH4UAQEAAAAA8qsHs5MMwvaTJoc++kGCUvyGn9od2r8SeheTKCk1uFgA","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAfQEAIoIHEdBHAQ9+oavXvoj/Ho3UI3wfaJy9HWVOgkjr/aB8BQCneggyXkHWzhxqKF8WNguo","next":null},"slot":16848004,"index":8}`
		compareTransaction(t, transaction_raw2, expectAsJson)
	})
	t.Run("classic-fast/3", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AbjhOmVSb6dBNf7FcdWRwXvLDOmVeCvDdH4srQhbKRwj1YSeyxuhpyggTZlw70yqXQX84VM4EBC62/BDV3KqNQABAAMFrBYKcNpllQ32WLoMCd2PaL1ByibWi05UEFONRtCO9tN/rqFhq+q8I5Y2Z+0JFrZ3yFicOGyV+ehkL4SjrHfiJQan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAADlz4zDCm4MfP8sOSTdO2NA4j0EKI+Tr8jMoUFA2770DAQQEAQIDAD0CAAAAAQAAAAAAAAAAAAAAAAAAAKsDQFxUzcQvpR7Wgr04E4nWAkP1c5fE3h52rVPT1WJKAZuNb14AAAAA","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"","next":null},"slot":1,"index":1}`
		compareTransaction(t, transaction_raw3, expectAsJson)
	})
	t.Run("classic-fast/4", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AdHaUM234iw6vFCpgRRaF4Lvrb2sAmKokR/BgzaQLIVbxctNhzSE5Ix7Qr6KwWjKiMY8n+SIoWFPt7XK27gQ0gIBAAMFCK6Qs/2APoEj6JkBOD1M9U0vjKxIY8kKr6NMUEW4acMIrpCz3Qi9S1iHrT5Ko9CID7ZaeVz/bOYvjz35TFxFdAan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAALY8zyGeltaQlaJeQ5wMCwZM8BOX2PZ5LVgiytnw6PELAQQEAQIDAD0CAAAAAgAAAAAAAAB9FAEBAAAAAH4UAQEAAAAA8qsHs5MMwvaTJoc++kGCUvyGn9od2r8SeheTKCk1uFgA","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAdQEAIkIHEOBJAQCoCRidUlq12kNRvw8CWH2ddCfIjGt/KB8FAKd6CDJeQdbOHGooX48KQ74=","next":null},"slot":16848004,"index":1}`
		compareTransaction(t, transaction_raw4, expectAsJson)
	})
	t.Run("classic-fast/5", func(t *testing.T) {
		expectAsJson := `{"kind":0,"data":{"kind":6,"hash":null,"index":null,"total":null,"data":"AQeB17Q3DGsAv2R6BmbM7ukaXyYynXVmr+coaZ/TKfyK3fjJsEQu1fJg7wEN98c7D+N/KpBEiieUumyfRcojpgIBAAMFGbp8+B5VJlJMidUT8RS7fDdlLddAEj5D8sMi7g2Dm6ay3bgQbbpn1DKxtxmGFCf6JW/b2WjXiaLebsTElKgjLQan1RcZLwqvxvJl4/t3zHragsUp0L47E24tAFUgAAAABqfVFxjHdMkoVmOYaR1etoteuKObS21cc1VbIQAAAAAHYUgdNXR0u3xNdiTr072z2DVec9EQQ/wNo1OAAAAAAATJHdRQdragJfvZNTXpGfb842WXhg6U+rPInif8dK4lAQQEAQIDAEUCAAAAAwAAAAAAAAB9FAEBAAAAAH4UAQEAAAAAfxQBAQAAAAAijOuN/EOPELmIQmzJugTw+loz4N7EUvIebukxbsQxbQA=","next":null},"metadata":{"kind":6,"hash":null,"index":null,"total":null,"data":"KLUv/QQAdQEAIkIHEOBJATj3aGqd6z4G1hgK+X2BWH2ZUz4OidR+cB8FAKd6CDJeQdbOHGooXxSr/Ag=","next":null},"slot":16848004,"index":4}`
		compareTransaction(t, transaction_raw5, expectAsJson)
	})
}

func BenchmarkTransaction(b *testing.B) {
	b.Run("classic/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw3)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/3", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw3)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw4)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/4", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw4)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionClassic(transaction_raw5)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/5", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeTransactionFast(transaction_raw5)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}

func TestDataFrame(t *testing.T) {
	t.Run("classic-fast/0", func(t *testing.T) {
		frameClassic, err := _DecodeDataFrameClassic(dataFrame_raw0)
		require.NoError(t, err)
		require.NotNil(t, frameClassic)

		frameFast, err := _DecodeDataFrameFast(dataFrame_raw0)
		require.NoError(t, err)
		require.NotNil(t, frameFast)

		compareDataFrame_simple(t, *frameClassic, *frameFast)

		expectAsJson := `{"kind":6,"hash":"13388989860809387070","index":1,"total":2,"data":"IHdvcmxk","next":null}`
		gotAsJson, err := json.Marshal(frameClassic)
		require.NoError(t, err)
		require.JSONEq(t, expectAsJson, string(gotAsJson))

		{ // test CBOR encoding
			encoded, err := frameClassic.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, dataFrame_raw0, encoded)
			{
				// let's test as value equivalence instead of comparing bytes
				decoded, err := DecodeDataFrame(encoded)
				require.NoError(t, err)
				// compare field by field so it's easier to debug
				require.EqualValues(t, frameClassic.Kind, decoded.Kind)
				require.EqualValues(t, frameClassic.Hash, decoded.Hash)
				require.EqualValues(t, frameClassic.Index, decoded.Index)
				require.EqualValues(t, frameClassic.Total, decoded.Total)
				require.EqualValues(t, frameClassic.Data, decoded.Data)
				require.EqualValues(t, frameClassic.Next, decoded.Next)

				// require.EqualValues(t, frameClassic, decoded)
				{
					// now compare as json
					frameJson, err := json.Marshal(frameClassic)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(frameJson), string(decodedJson))
				}
			}
		}
	})
	t.Run("classic-fast/1", func(t *testing.T) {
		frameClassic, err := _DecodeDataFrameClassic(dataFrame_raw1)
		require.NoError(t, err)
		require.NotNil(t, frameClassic)

		frameFast, err := _DecodeDataFrameFast(dataFrame_raw1)
		require.NoError(t, err)
		require.NotNil(t, frameFast)

		compareDataFrame_simple(t, *frameClassic, *frameFast)

		expectAsJson := `{"kind":6,"hash":"5236830283428082936","index":26,"total":28,"data":"sk/pZfAGyREJDg==","next":null}`
		gotAsJson, err := json.Marshal(frameClassic)
		require.NoError(t, err)
		require.JSONEq(t, expectAsJson, string(gotAsJson))

		{ // test CBOR encoding
			encoded, err := frameClassic.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, dataFrame_raw1, encoded)
			{
				// let's test as value equivalence instead of comparing bytes
				decoded, err := DecodeDataFrame(encoded)
				require.NoError(t, err)
				// compare field by field so it's easier to debug
				require.EqualValues(t, frameClassic.Kind, decoded.Kind)
				require.EqualValues(t, frameClassic.Hash, decoded.Hash)
				require.EqualValues(t, frameClassic.Index, decoded.Index)
				require.EqualValues(t, frameClassic.Total, decoded.Total)
				require.EqualValues(t, frameClassic.Data, decoded.Data)
				require.EqualValues(t, frameClassic.Next, decoded.Next)

				// require.EqualValues(t, frameClassic, decoded)
				{
					// now compare as json
					frameJson, err := json.Marshal(frameClassic)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(frameJson), string(decodedJson))
				}
			}
		}
	})
	t.Run("classic-fast/2", func(t *testing.T) {
		frameClassic, err := _DecodeDataFrameClassic(dataFrame_raw2)
		require.NoError(t, err)
		require.NotNil(t, frameClassic)

		frameFast, err := _DecodeDataFrameFast(dataFrame_raw2)
		require.NoError(t, err)
		require.NotNil(t, frameFast)

		compareDataFrame_simple(t, *frameClassic, *frameFast)

		expectAsJson := `{"kind":6,"hash":"5236830283428082936","index":22,"total":28,"data":"b+2zraUnY6tx6Q==","next":[{"/":"bafyreid2i4binymehw5kf75yduyadcsa5db3wfacnnqil7ld2sp5n2y7wa"},{"/":"bafyreia4rs42uo2srir5pvj2r3rveh4septkpept225yrya7zlqzf5pfyy"},{"/":"bafyreidly4pxe4x3ie4n43htg23d7qvshxcukbeai47hjxrlnh5a5nvphq"},{"/":"bafyreicxgl7qbfjqwzigin5altahbcc7xjg2nh7ubpjqy37lxn6b2nesmy"},{"/":"bafyreicr3bznoht2g3rixrbwdscszac3y4ic6kmjx3lgdftmihznsmzrj4"}]}`
		gotAsJson, err := json.Marshal(frameClassic)
		require.NoError(t, err)
		require.JSONEq(t, expectAsJson, string(gotAsJson))

		{ // test CBOR encoding
			encoded, err := frameClassic.MarshalCBOR()
			require.NoError(t, err)
			require.Equal(t, dataFrame_raw2, encoded)
			{
				// let's test as value equivalence instead of comparing bytes
				decoded, err := DecodeDataFrame(encoded)
				require.NoError(t, err)
				// compare field by field so it's easier to debug
				require.EqualValues(t, frameClassic.Kind, decoded.Kind)
				require.EqualValues(t, frameClassic.Hash, decoded.Hash)
				require.EqualValues(t, frameClassic.Index, decoded.Index)
				require.EqualValues(t, frameClassic.Total, decoded.Total)
				require.EqualValues(t, frameClassic.Data, decoded.Data)
				require.EqualValues(t, frameClassic.Next, decoded.Next)

				// require.EqualValues(t, frameClassic, decoded)
				{
					// now compare as json
					frameJson, err := json.Marshal(frameClassic)
					require.NoError(t, err)
					decodedJson, err := json.Marshal(decoded)
					require.NoError(t, err)
					require.JSONEq(t, string(frameJson), string(decodedJson))
				}
			}
		}
	})
}

func BenchmarkDataFrame(b *testing.B) {
	b.Run("classic/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameClassic(dataFrame_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/0", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameFast(dataFrame_raw0)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameClassic(dataFrame_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/1", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameFast(dataFrame_raw1)
			if err != nil {
				b.Fatal(err)
			}
		}
	})

	b.Run("classic/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameClassic(dataFrame_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
	b.Run("fast/2", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			_, err := _DecodeDataFrameFast(dataFrame_raw2)
			if err != nil {
				b.Fatal(err)
			}
		}
	})
}
