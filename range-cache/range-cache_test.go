package rangecache

import (
	"bytes"
	"context"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCache(t *testing.T) {
	t.Run("basic", func(t *testing.T) {
		v := []byte("hello")
		full := append(v, []byte(" world")...)
		rd := bytes.NewReader(full)
		rc := NewRangeCache(
			int64(len(full)),
			"test",
			func(p []byte, off int64) (n int, err error) {
				return rd.ReadAt(p, off)
			})

		{
			{
				err := rc.SetRange(context.Background(), 0, int64(len(v)), v)
				require.NoError(t, err)
				err = rc.SetRange(context.Background(), 1, 1, []byte("e"))
				require.NoError(t, err)
			}
			/////
			{
				got, err := rc.GetRange(context.Background(), 1, 3)
				require.NoError(t, err)
				require.Equal(t, []byte("ell"), got)
			}
			{
				got, err := rc.GetRange(context.Background(), 1, 7)
				require.NoError(t, err)
				require.Equal(t, []byte("ello wo"), got)
			}
		}
	})
}
