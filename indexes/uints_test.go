package indexes

import (
	"math"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestUints(t *testing.T) {
	{
		require.Equal(t, int(16_777_215), MaxUint24)
		require.Equal(t, int(1_099_511_627_775), MaxUint40)
		require.Equal(t, int(281_474_976_710_655), MaxUint48)
		require.Equal(t, uint(math.MaxUint64), uint(MaxUint64))
	}
	{
		v := Uint24tob(0)
		require.Equal(t, []byte{0, 0, 0}, v)
		require.Equal(t, uint32(0), BtoUint24(v))

		v = Uint24tob(1)
		require.Equal(t, []byte{1, 0, 0}, v)
		require.Equal(t, uint32(1), BtoUint24(v))

		v = Uint24tob(MaxUint24)
		require.Equal(t, []byte{255, 255, 255}, v)
		require.Equal(t, uint32(MaxUint24), BtoUint24(v))

		v = Uint24tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0}, v)
		require.Equal(t, uint32(123), BtoUint24(v))

		require.Panics(t, func() {
			v = Uint24tob(MaxUint24 + 1)
			require.Equal(t, []byte{0, 0, 0}, v)
			require.Equal(t, uint32(0), BtoUint24(v))
		})
	}
	{
		v := Uint40tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), BtoUint40(v))

		v = Uint40tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), BtoUint40(v))

		v = Uint40tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), BtoUint40(v))

		v = Uint40tob(MaxUint40)
		require.Equal(t, []byte{255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(MaxUint40), BtoUint40(v))

		require.Panics(t, func() {
			v = Uint40tob(MaxUint40 + 1)
			require.Equal(t, []byte{0, 0, 0, 0, 0}, v)
			require.Equal(t, uint64(0), BtoUint40(v))
		})
	}
	{
		v := Uint48tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), BtoUint48(v))

		v = Uint48tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), BtoUint48(v))

		v = Uint48tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), BtoUint48(v))

		v = Uint48tob(MaxUint48)
		require.Equal(t, []byte{255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(MaxUint48), BtoUint48(v))

		require.Panics(t, func() {
			v = Uint48tob(MaxUint48 + 1)
			require.Equal(t, []byte{0, 0, 0, 0, 0, 0}, v)
			require.Equal(t, uint64(0), BtoUint48(v))
		})
	}
	{
		v := Uint64tob(0)
		require.Equal(t, []byte{0, 0, 0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(0), BtoUint64(v))

		v = Uint64tob(1)
		require.Equal(t, []byte{1, 0, 0, 0, 0, 0, 0, 0}, v)
		require.Equal(t, uint64(1), BtoUint64(v))

		v = Uint64tob(123)
		require.Equal(t, []byte{0x7b, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0, 0x0}, v)
		require.Equal(t, uint64(123), BtoUint64(v))

		v = Uint64tob(math.MaxUint64)
		require.Equal(t, []byte{255, 255, 255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(math.MaxUint64), BtoUint64(v))

		v = Uint64tob(math.MaxUint64 - 1)
		require.Equal(t, []byte{254, 255, 255, 255, 255, 255, 255, 255}, v)
		require.Equal(t, uint64(math.MaxUint64-1), BtoUint64(v))
	}
	{
		buf := make([]byte, 9)
		copy(buf[:6], Uint48tob(123))
		copy(buf[6:], Uint24tob(uint32(456)))
		{
			require.Equal(t, buf[:6], Uint48tob(123))
			require.Equal(t, buf[6:], Uint24tob(uint32(456)))
		}
		{
			v := BtoUint48(buf[:6])
			require.Equal(t, uint64(123), v)
			require.Equal(t, uint32(123), uint32(v))
		}
		{
			v := BtoUint24(buf[6:])
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
		require.Equal(t, Uint48tob(123), buf[:6])
		require.Equal(t, Uint24tob(uint32(456)), buf[6:])
		require.Equal(t, uint64(123), BtoUint48(buf[:6]))
		require.Equal(t, uint32(456), BtoUint24(Uint24tob(uint32(456))))
		require.Equal(t, uint32(456), BtoUint24(buf[6:]))
		require.Equal(t, uint64(uint32(456)), uint64(BtoUint24(buf[6:])))
	}
}
