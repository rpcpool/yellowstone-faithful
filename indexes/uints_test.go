package indexes

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUints(t *testing.T) {
	{
		require.Equal(t, int(16_777_215), maxUint24)
		require.Equal(t, int(1_099_511_627_775), maxUint40)
		require.Equal(t, int(281_474_976_710_655), maxUint48)
		require.Equal(t, uint(math.MaxUint64), uint(maxUint64))
	}
	{
		v := uint24tob(0)
		require.Equal(t, []byte{0, 0, 0}, v)
		require.Equal(t, uint32(0), btoUint24(v))

		v = uint24tob(1)
		require.Equal(t, []byte{1, 0, 0}, v)
		require.Equal(t, uint32(1), btoUint24(v))

		v = uint24tob(maxUint24)
		require.Equal(t, []byte{255, 255, 255}, v)
		require.Equal(t, uint32(maxUint24), btoUint24(v))

		v = uint24tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0}, v)
		require.Equal(t, uint32(123), btoUint24(v))

		require.Panics(t, func() {
			v = uint24tob(maxUint24 + 1)
			require.Equal(t, []byte{0, 0, 0}, v)
			require.Equal(t, uint32(0), btoUint24(v))
		})
	}
	{
		v := uint40tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), btoUint40(v))

		v = uint40tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), btoUint40(v))

		v = uint40tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), btoUint40(v))

		v = uint40tob(maxUint40)
		require.Equal(t, []byte{255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(maxUint40), btoUint40(v))

		require.Panics(t, func() {
			v = uint40tob(maxUint40 + 1)
			require.Equal(t, []byte{0, 0, 0, 0, 0}, v)
			require.Equal(t, uint64(0), btoUint40(v))
		})
	}
	{
		v := uint48tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), btoUint48(v))

		v = uint48tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), btoUint48(v))

		v = uint48tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), btoUint48(v))

		v = uint48tob(maxUint48)
		require.Equal(t, []byte{255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(maxUint48), btoUint48(v))

		require.Panics(t, func() {
			v = uint48tob(maxUint48 + 1)
			require.Equal(t, []byte{0, 0, 0, 0, 0, 0}, v)
			require.Equal(t, uint64(0), btoUint48(v))
		})
	}
	{
		v := uint64tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), btoUint64(v))

		v = uint64tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), btoUint64(v))

		v = uint64tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), btoUint64(v))

		v = uint64tob(math.MaxUint64)
		require.Equal(t, []byte{255, 255, 255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(math.MaxUint64), btoUint64(v))

		v = uint64tob(math.MaxUint64 - 1)
		require.Equal(t, []byte{254, 255, 255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(math.MaxUint64-1), btoUint64(v))
	}
	{
		buf := make([]byte, 9)
		copy(buf[:6], uint48tob(123))
		copy(buf[6:], uint24tob(uint32(456)))
		{
			require.Equal(t, buf[:6], uint48tob(123))
			require.Equal(t, buf[6:], uint24tob(uint32(456)))
		}
		{
			v := btoUint48(buf[:6])
			require.Equal(t, uint64(123), v)
			require.Equal(t, uint32(123), uint32(v))
		}
		{
			v := btoUint24(buf[6:])
			require.Equal(t, uint32(456), v)
			require.Equal(t, uint64(uint32(456)), uint64(v))
		}

		{
			v := OffsetAndSize{
				Offset: 123,
				Size:   456,
			}
			encoded := v.Bytes()
			require.Equal(t, []byte{0x7b, 0x00, 0x00, 0x00, 0x00, 0x00, 0xc8, 0x01, 0x00}, encoded)
			require.Equal(t, buf, encoded)
		}
		require.Equal(t, uint48tob(123), buf[:6])
		require.Equal(t, uint24tob(uint32(456)), buf[6:])
		require.Equal(t, uint64(123), btoUint48(buf[:6]))
		require.Equal(t, uint32(456), btoUint24(uint24tob(uint32(456))))
		require.Equal(t, uint32(456), btoUint24(buf[6:]))
		require.Equal(t, uint64(uint32(456)), uint64(btoUint24(buf[6:])))
	}
}
