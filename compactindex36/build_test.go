package compactindex36

// This is a fork of the original project at https://github.com/firedancer-io/radiance/tree/main/pkg/compactindex
// The following changes have been made:
// - The package has been renamed to `compactindex36` to avoid conflicts with the original package
// - The values it indexes are 36-bit values instead of 8-bit values. This allows to index CIDs (in particular sha256+CBOR CIDs) directly.

import (
	"bytes"
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/davecgh/go-spew/spew"
	"github.com/ipfs/go-cid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/vbauerster/mpb/v8/decor"
)

var testCidStrings = []string{
	"bafyreiba5kzq6wf6neax6ascsh5khxhuy7zc6vqsu6zac32i7ilv4u62nm",
	"bafyreie42alzugm43fiqv64ss3h5fh3xriaeamul7d7qmrrbxe6fpjo5b4",
	"bafyreidam5koitaftfx7sydge5ta3ig2j5qbabqcql4umpom3yuia4sbm4",
	"bafyreia3pebgypo4oqgdg4pqpjfybmcdbsbavcdscotji4wj2gfc3r4icm",
	"bafyreigudmeashua4432mbq3tawwnsz3qfpmm5tjpwahopn7cxttotqdge",
	"bafyreic3azak2ds4fomkw35pmvsznu46sgonmketlnfaqnoc6owi4t64my",
	"bafyreib6t4ooiajnebkwgk4z57fhcvejc663a6haq6cb6tjjluj4fuulla",
	"bafyreidmohyrgchkgavx7wubebip5agb4ngisnlkqaot4kz2eo635ny5m4",
	"bafyreicpmxvpxwjemofmic6aka72dliueqxtsklrilkofwbqgn6ffuz7ka",
	"bafyreifkjdmj3kmi2hkoqcqweunbktouxo6sy362rysl34ffyjinebylim",
	"bafyreidzql2rmbs3chtq2cmbnncvfyz2tjclwqx4vnowvyph77fomh26qi",
	"bafyreig4kpaq6rf5y46qgqhdzgr5uauubfqyevbmj6pmtaxxhh3tkyzury",
	"bafyreianxqyomvh6dl533cs25z7yfda2z62ity3w7sdqf3kk4tmogu7t24",
	"bafyreicaq6dv5jsq4du2tqiefr3baepnj4ei3bpxvg5g6np7ygacgbw5aq",
	"bafyreia4b2nleifcp54w4scrjy7fgctsoy6zz4mkot3gw6xydqkrc2wdtq",
	"bafyreierpgsryprxfgshtzjarnb662d5akhg7om6utubggjwtlg6qwwj5i",
	"bafyreidufcwvs7fvot2blqnwciaxre35s3ip6xxkncrus4voci3ktots2q",
	"bafyreif23uzartrw62g5pywtrsz3xsl2wdw73o4fvtsf76gqgx37mfpqjm",
	"bafyreianu4oifizvqyop753ao4hrocftlbnn6kzm7xtsm4ryaz6uawkgmu",
	"bafyreidekyir7cti4jch652nnmimrftoqynjxl6vzjimfkqxh42rx27yiy",
	"bafyreia3zuym3akg4gp5ewlmdwxnybrsqrab4m6tpsgxq65az6z7r5jtba",
	"bafyreihherovnppygar6h5hu4we4hkxrvoqtpkawwcmn7hkyeq6cisouyu",
	"bafyreicmqd5dhn2hv2qcskf27vsml36p6srz6zoxjfjkmnu7ltczqtbkbe",
	"bafyreihy2c7lomf3i3nucd5bbwvox3luhtnzujkybchgmyxenmanon7rxe",
	"bafyreicld6buy3mr4ibs2jzakoaptdj7xvpjo4pwhwiuywnrzfzoh5ahqi",
	"bafyreifyapa5a5ii72hfmqiwgsjto6iarshfwwvrrxdw3bhr62ucuutl4e",
	"bafyreigrlvwdaivwthwvihcbyrnl5pl7jfor72xlaivi2f6ajypy4yku3a",
	"bafyreiamvgkbpuahegu3mhxxujzvxk2t5hrykhrfw47yurlxqumkv243iy",
	"bafyreib4qf7qpjmpr2eqi7mqwqxw2fznnkvhzkpj3udiloxqay5fhk5wui",
	"bafyreidbol6tdhj42rdpchpafszgmnmg7tgvi2uwou7s2whiamznzawhk4",
	"bafyreidrpejzimhuwq6j74jzv2odzriuitwmdkp2ibojzcax6jdpqiztti",
	"bafyreidrgb4vmgvsreebrj6apscopszfbgw5e7llh22kk2cdayyeoyggwy",
	"bafyreigpzlopkl2ttxfdf6n5sgxyda4bvlglre7nkjq37uecmvf47f6ttm",
	"bafyreidcq3csrifsyeed42fbky42w7bxhvg6fd42l7qkw3cnxliab4e7nu",
	"bafyreibchdux4qchrrz67kikde273mjth475fedjisvoazf3zhmodlkx7a",
	"bafyreie4rdlgpfcrrdlonofkwlrefh6z5hcwieasatkddozvyknwqahh4q",
	"bafyreibhwuih7ekso6zypyr4uwl37xewyu7foy2clqvz4l7lbgwxpslyyu",
	"bafyreigltijqq3m6h7h6du5o4ynqwmimtslnsmyu3njwlnpuyadyev6awa",
	"bafyreihwtszo3p7ujg2wsuhsqon5tidxxnyin2t42uhj7zq6xta7fo2suy",
	"bafyreie2uggjajncn2lna6ytq2sw2uu4xw724pe6wj4ihhiawnnjm5sgwa",
	"bafyreignb5gdw7fwfycoipjqbkvkve7dkuugr3s5ylkaucn3ks7klxh4te",
	"bafyreib3iwnufpnoxgf7z5w3vtygu2z2kcqxj3quxypupfgmr53tyt6wdq",
	"bafyreic7kxsh7nmfpxmrm727yug2rfnrhfuavmpll3cms4r6cpnbbuwgqm",
	"bafyreig2o4yrzlwo74eom4v65tenr6yjh2v23vbl7sjffrppzceenxs3eq",
	"bafyreidletnh5bxnc6k2p3idnul5qatfcf4qqrgmkjxolgpu7wolye47hm",
	"bafyreigv2nni66nb6be5dchkonpb2t556qplv5xz4vdolwmyz4m32aufdi",
	"bafyreid66pezal5svaidpvxc3zz6w5eksxcjn6omelhsqhj5jmcmxhgjhm",
	"bafyreihjhwpvm2soq5syyovsiqrchsuojsdk4imj2gqk6pikc4rxdqtmny",
	"bafyreidt3oveadwf5jrmxatrwa5bdxvfyxnrucypmtqwiu2pvrrztrj5xe",
	"bafyreid6y6r44wqcwql5yyitmw5mpfmrrlsois2unbqzmtlvyeefqahnnu",
	"bafyreic6evvtf3y3slkbwhzbjuvspqu2jxf7qr267rhigmox6f4a5a36eq",
	"bafyreiekep5a55yvebqzzi6x7xyotse57zfwcpyeh2xermqkvxlkvpxh24",
	"bafyreigwb22sgfg56dc2jnnvxttjyhwfp4itevlukqj2wfz5ebru72elv4",
	"bafyreiebz2fxh64dqvbiwmqnyj5rj63txl5u7abmets2imhn2su6tcuvyu",
	"bafyreigcm7wkxlsyc26acgb7nfjho2twh6au2pbk35w6bsbv2qt7rt7iaq",
	"bafyreieiuq6g74i25huoumvey7oynljndt2d4qvbddqkhpysrexu7ixsuy",
	"bafyreihuhj5slybgbqzdr4mpkyo5dwvqjxfhicardbph6htiyeut2frol4",
	"bafyreiaskg4kwqrpdcatnymvno4xf54uewysdiz3357fdct2tlnx2gpkqq",
	"bafyreicakit2lbmg3wo4uoox4rc2gv3odzrrkrr32zwk7qaolpoc7uyz5u",
	"bafyreih5jcnhw4evhq5j4n75miruqfofo2dv46hdtqyd5ht2eqeu7g5cme",
	"bafyreicwtl6ulct4ckjnq57gmctw3wjo6ctvjbbr7l4bwfbzpj3y3g6unm",
	"bafyreiebgoqj3nawzcwjy4t67uljnmvfh55fiqaxsskld6qpjvd2majesq",
	"bafyreif472dxwhnyjhxmxoto3czfblhssgmhrpsqcmrwzprywk45wqdtmi",
	"bafyreiaz444on546zihfuygqchlw4r4vu2tuw5xnelm6dsodqcno23pvzu",
	"bafyreidgzghcd2lfdcylsccvlj43f5ujj7xtriu6ojp7jog5iainecagka",
	"bafyreiehvi56dn3zm2ltfgecss2ydfmcb2hmf6hk76b6ebpoxhquajawze",
	"bafyreie4wcortvdsirbontddokin6wgm25xg46lu3qxcyyjj6rgkuk5cca",
	"bafyreicurlgiukht7wnxy3za3hz5fzs2a62ggc6i3rqhzhck4p2lgt5754",
	"bafyreihn2zwm7m3tqfwa53me4qxiit66yiny5sxtkvvjewjfkbjrgmeswu",
	"bafyreid7m33qok7d66vsyc5mq257rya5sg24rzv5qwbghwsimclt5ll7pi",
}

var testCids = func() []cid.Cid {
	var cids []cid.Cid
	for _, s := range testCidStrings {
		c, err := cid.Decode(s)
		if err != nil {
			panic(err)
		}
		cids = append(cids, c)
	}
	return cids
}()

func concatBytes(bs ...[]byte) []byte {
	var out []byte
	for _, b := range bs {
		out = append(out, b...)
	}
	return out
}

func numberToHexBytes(n int) string {
	return (fmt.Sprintf("0x%02x", n))
}

func FormatByteSlice(buf []byte) string {
	elems := make([]string, 0)
	for _, v := range buf {
		elems = append(elems, numberToHexBytes(int(v)))
	}

	return "{" + strings.Join(elems, ", ") + "}" + fmt.Sprintf("(len=%v)", len(elems))
}

func splitBufferWithProvidedSizes(buf []byte, sizes []int) [][]byte {
	var out [][]byte
	var offset int
	for _, size := range sizes {
		out = append(out, buf[offset:offset+size])
		offset += size
	}
	return out
}

func compareBufferArrays(a, b [][]byte) []bool {
	var out []bool

	for i := 0; i < len(a); i++ {
		out = append(out, bytes.Equal(a[i], b[i]))
	}

	return out
}

func TestBuilder(t *testing.T) {
	const numBuckets = 3
	const maxValue = math.MaxUint64

	// Create a table with 3 buckets.
	builder, err := NewBuilder("", numBuckets*targetEntriesPerBucket, maxValue)
	require.NoError(t, err)
	require.NotNil(t, builder)
	assert.Len(t, builder.buckets, 3)
	defer builder.Close()

	// Insert a few entries.
	keys := []string{"hello", "world", "blub", "foo"}
	for i, key := range keys {
		require.NoError(t, builder.Insert([]byte(key), [36]byte(testCids[i].Bytes())))
	}
	{
		// print test values
		for _, tc := range testCids {
			spew.Dump(FormatByteSlice(tc.Bytes()))
		}
	}

	// Create index file.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal index.
	require.NoError(t, builder.Seal(context.TODO(), targetFile))

	// Assert binary content.
	buf, err := os.ReadFile(targetFile.Name())
	require.NoError(t, err)
	expected := concatBytes(
		// --- File header
		// magic
		[]byte{0x72, 0x64, 0x63, 0x65, 0x63, 0x69, 0x64, 0x78}, // 0
		// max file size
		[]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff}, // 1
		// num buckets
		[]byte{0x03, 0x00, 0x00, 0x00}, // 2
		// padding
		[]byte{0x01, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, // 3

		// --- Bucket header 0
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 4
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00}, // 5
		// hash len
		[]byte{0x03}, // 6
		// padding
		[]byte{0x00}, // 7
		// file offset
		[]byte{0x50, 0x00, 0x00, 0x00, 0x00, 0x00}, // 8

		// --- Bucket header 1
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 9
		// num entries
		[]byte{0x01, 0x00, 0x00, 0x00}, // 10
		// hash len
		[]byte{0x03}, // 11
		// padding
		[]byte{0x00}, // 12
		// file offset
		[]byte{0x77, 0x00, 0x00, 0x00, 0x00, 0x00}, // 13

		// --- Bucket header 2
		// hash domain
		[]byte{0x00, 0x00, 0x00, 0x00}, // 14
		// num entries
		[]byte{0x02, 0x00, 0x00, 0x00}, // 15
		// hash len
		[]byte{0x03}, // 16
		// padding
		[]byte{0x00}, // 17
		// file offset
		[]byte{0x9e, 0x00, 0x00, 0x00, 0x00, 0x00}, // 18

		// --- Bucket 0
		// hash
		[]byte{0xe2, 0xdb, 0x55}, // 19
		// value
		[]byte{0x1, 0x71, 0x12, 0x20, 0x20, 0xea, 0xb3, 0xf, 0x58, 0xbe, 0x69, 0x1, 0x7f, 0x2, 0x42, 0x91, 0xfa, 0xa3, 0xdc, 0xf4, 0xc7, 0xf2, 0x2f, 0x56, 0x12, 0xa7, 0xb2, 0x1, 0x6f, 0x48, 0xfa, 0x17, 0x5e, 0x53, 0xda, 0x6b}, // 20

		// --- Bucket 2
		// hash
		[]byte{0x92, 0xcd, 0xbb}, // 21
		// value
		[]byte{0x01, 0x71, 0x12, 0x20, 0x9c, 0xd0, 0x17, 0x9a, 0x19, 0x9c, 0xd9, 0x51, 0x0a, 0xfb, 0x92, 0x96, 0xcf, 0xd2, 0x9f, 0x77, 0x8a, 0x00, 0x40, 0x32, 0x8b, 0xf8, 0xff, 0x06, 0x46, 0x21, 0xb9, 0x3c, 0x57, 0xa5, 0xdd, 0x0f}, // 22
		// hash
		[]byte{0x98, 0x3d, 0xbd}, // 25
		// value
		[]byte{0x01, 0x71, 0x12, 0x20, 0x1b, 0x79, 0x02, 0x6c, 0x3d, 0xdc, 0x74, 0x0c, 0x33, 0x71, 0xf0, 0x7a, 0x4b, 0x80, 0xb0, 0x43, 0x0c, 0x82, 0x0a, 0x88, 0x72, 0x13, 0xa6, 0x94, 0x72, 0xc9, 0xd1, 0x8a, 0x2d, 0xc7, 0x88, 0x13}, // 26
		// hash
		[]byte{0xe3, 0x09, 0x6b}, // 23
		// value
		[]byte{0x1, 0x71, 0x12, 0x20, 0x60, 0x67, 0x54, 0xe4, 0x4c, 0x5, 0x99, 0x6f, 0xf9, 0x60, 0x66, 0x27, 0x66, 0xd, 0xa0, 0xda, 0x4f, 0x60, 0x10, 0x6, 0x2, 0x82, 0xf9, 0x46, 0x3d, 0xcc, 0xde, 0x28, 0x80, 0x72, 0x41, 0x67}, // 24
	)
	assert.Equal(t, expected, buf)

	{
		splitSizes := []int{
			// --- File header
			8, 8, 4, 12,
			// --- Bucket header 0
			4, 4, 1, 1, 6,
			// --- Bucket header 1
			4, 4, 1, 1, 6,
			// --- Bucket header 2
			4, 4, 1, 1, 6,
			// --- Bucket 0
			3, 36,
			// --- Bucket 2
			3, 36, 3, 36, 3, 36,
		}
		splitExpected := splitBufferWithProvidedSizes(expected, splitSizes)
		splitGot := splitBufferWithProvidedSizes(buf, splitSizes)

		comparations := compareBufferArrays(splitExpected, splitGot)

		for i, equal := range comparations {
			if !equal {
				t.Errorf("%d: \nexpected: %v, \n     got: %v", i, FormatByteSlice(splitExpected[i]), FormatByteSlice(splitGot[i]))
			}
		}

	}

	// Reset file offset.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)

	// Open index.
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")
	require.NotNil(t, db)

	// File header assertions.
	assert.Equal(t, Header{
		FileSize:   maxValue,
		NumBuckets: numBuckets,
	}, db.Header)

	// Get bucket handles.
	buckets := make([]*Bucket, numBuckets)
	for i := range buckets {
		buckets[i], err = db.GetBucket(uint(i))
		require.NoError(t, err)
	}

	// Ensure out-of-bounds bucket accesses fail.
	_, wantErr := db.GetBucket(numBuckets)
	assert.EqualError(t, wantErr, "out of bounds bucket index: 3 >= 3")

	// Bucket header assertions.
	assert.Equal(t, BucketDescriptor{
		BucketHeader: BucketHeader{
			HashDomain: 0x00,
			NumEntries: 1,
			HashLen:    3,
			FileOffset: 0x50,
		},
		Stride:      39, // 3 + 36
		OffsetWidth: 36,
	}, buckets[0].BucketDescriptor)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 1,
		HashLen:    3,
		FileOffset: 119,
	}, buckets[1].BucketHeader)
	assert.Equal(t, BucketHeader{
		HashDomain: 0x00,
		NumEntries: 2,
		HashLen:    3,
		FileOffset: 158,
	}, buckets[2].BucketHeader)

	// Test lookups.
	entries, err := buckets[2].Load( /*batchSize*/ 3)
	require.NoError(t, err)
	assert.Equal(t, []Entry{
		{
			Hash:  12402072,
			Value: [36]byte(testCids[3].Bytes()),
		},
		{
			Hash:  7014883,
			Value: [36]byte(testCids[2].Bytes()),
		},
	}, entries)

	{
		for i, keyString := range keys {
			key := []byte(keyString)
			bucket, err := db.LookupBucket(key)
			require.NoError(t, err)

			value, err := bucket.Lookup(key)
			require.NoError(t, err)
			assert.Equal(t, [36]byte(testCids[i].Bytes()), value)
		}
	}
}

func TestBuilder_Random(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping long test")
	}

	numKeys := uint(len(testCids))
	const keySize = uint(16)
	const maxOffset = uint64(1000000)
	const queries = int(10000)

	// Create new builder session.
	builder, err := NewBuilder("", numKeys, maxOffset)
	require.NoError(t, err)
	require.NotNil(t, builder)
	require.NotEmpty(t, builder.buckets)

	// Ensure we cleaned up after ourselves.
	defer func() {
		_, statErr := os.Stat(builder.dir)
		assert.Truef(t, errors.Is(statErr, fs.ErrNotExist), "Delete failed: %v", statErr)
	}()
	defer builder.Close()

	// Insert items to temp buckets.
	preInsert := time.Now()
	key := make([]byte, keySize)
	for i := uint(0); i < numKeys; i++ {
		binary.LittleEndian.PutUint64(key, uint64(i))
		err := builder.Insert(key, [36]byte(testCids[i].Bytes()))
		require.NoError(t, err)
	}
	t.Logf("Inserted %d keys in %s", numKeys, time.Since(preInsert))

	// Create file for final index.
	targetFile, err := os.CreateTemp("", "compactindex-final-")
	require.NoError(t, err)
	defer os.Remove(targetFile.Name())
	defer targetFile.Close()

	// Seal to final index.
	preSeal := time.Now()
	sealErr := builder.Seal(context.TODO(), targetFile)
	require.NoError(t, sealErr, "Seal failed")
	t.Logf("Sealed in %s", time.Since(preSeal))

	// Print some stats.
	targetStat, err := targetFile.Stat()
	require.NoError(t, err)
	t.Logf("Index size: %d (% .2f)", targetStat.Size(), decor.SizeB1000(targetStat.Size()))
	t.Logf("Bytes per entry: %f", float64(targetStat.Size())/float64(numKeys))
	t.Logf("Indexing speed: %f/s", float64(numKeys)/time.Since(preInsert).Seconds())

	// Open index.
	_, seekErr := targetFile.Seek(0, io.SeekStart)
	require.NoError(t, seekErr)
	db, err := Open(targetFile)
	require.NoError(t, err, "Failed to open generated index")

	// Run query benchmark.
	preQuery := time.Now()
	for i := queries; i != 0; i-- {
		keyN := uint64(rand.Int63n(int64(numKeys)))
		binary.LittleEndian.PutUint64(key, keyN)

		bucket, err := db.LookupBucket(key)
		require.NoError(t, err)

		value, err := bucket.Lookup(key)
		require.NoError(t, err)
		require.Equal(t, [36]byte(testCids[keyN].Bytes()), value)
	}
	t.Logf("Queried %d items", queries)
	t.Logf("Query speed: %f/s", float64(queries)/time.Since(preQuery).Seconds())
}
