package blocktimeindex

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestWriter(t *testing.T) {
	{
		epoch := uint64(0)
		w := NewForEpoch(epoch)
		if w.start != 0 {
			t.Errorf("expected 0, got %v", w.start)
		}
		if w.end != 431999 {
			t.Errorf("expected 431999, got %v", w.end)
		}
		if w.epoch != 0 {
			t.Errorf("expected 0, got %v", w.epoch)
		}
		if w.capacity != 432000 {
			t.Errorf("expected 432000, got %v", w.capacity)
		}
		if len(w.values) != 432000 {
			t.Errorf("expected 432000, got %v", len(w.values))
		}
		err := w.Set(0, 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if got, err := w.Get(0); got != 1 || err != nil {
			t.Errorf("expected 1, got %v, %v", got, err)
		}
		err = w.Set(431999, 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if got, err := w.Get(431999); got != 1 || err != nil {
			t.Errorf("expected 1, got %v, %v", got, err)
		}
		err = w.Set(432000, 1)
		if !errors.Is(err, &ErrSlotOutOfRange{}) {
			t.Errorf("expected ErrSlotOutOfRange, got %v", err)
		}
		// expect error when getting out of range slot
		if _, err := w.Get(432000); !errors.Is(err, &ErrSlotOutOfRange{}) {
			t.Errorf("expected ErrSlotOutOfRange, got %v", err)
		}
	}
	{
		epoch := uint64(1)
		w := NewForEpoch(epoch)
		if w.start != 432000 {
			t.Errorf("expected 432000, got %v", w.start)
		}
		if w.end != 863999 {
			t.Errorf("expected 863999, got %v", w.end)
		}
		if w.epoch != 1 {
			t.Errorf("expected 1, got %v", w.epoch)
		}
		if w.capacity != 432000 {
			t.Errorf("expected 432000, got %v", w.capacity)
		}
		if len(w.values) != 432000 {
			t.Errorf("expected 432000, got %v", len(w.values))
		}
		if len(w.values) != 432000 {
			t.Errorf("expected 432000, got %v", len(w.values))
		}
		err := w.Set(432000, 123)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if got, err := w.Get(432000); got != 123 || err != nil {
			t.Errorf("expected 1, got %v, %v", got, err)
		}
		err = w.Set(863999, 1)
		if err != nil {
			t.Errorf("expected no error, got %v", err)
		}
		if got, err := w.Get(863999); got != 1 || err != nil {
			t.Errorf("expected 1, got %v, %v", got, err)
		}
		err = w.Set(864000, 1)
		if !errors.Is(err, &ErrSlotOutOfRange{}) {
			t.Errorf("expected ErrSlotOutOfRange, got %v", err)
		}
		// expect error when getting out of range slot
		if _, err := w.Get(864000); !errors.Is(err, &ErrSlotOutOfRange{}) {
			t.Errorf("expected ErrSlotOutOfRange, got %v", err)
		}
		{
			// test writing
			buf, err := w.MarshalBinary()
			if err != nil {
				t.Errorf("expected no error, got %v", err)
			}
			// the expected length is:
			// - len(magic) + len(start) + len(end) + len(capacity) + len(epoch) + (len(values) * 4)
			expectedLen := DefaultIndexByteSize
			if len(buf) != expectedLen {
				t.Errorf("expected %v, got %v", expectedLen, len(buf))
			}
			{
				// test reading
				got, err := FromBytes(buf)
				if err != nil {
					t.Errorf("expected no error, got %v", err)
				}
				assert.Equal(t, w.start, got.start)
				assert.Equal(t, w.end, got.end)
				assert.Equal(t, w.epoch, got.epoch)
				assert.Equal(t, w.capacity, got.capacity)
				assert.Equal(t, w.values, got.values)
			}
		}
	}
}
