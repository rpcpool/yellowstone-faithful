package blocktimeindex

import "fmt"

var _ error = &ErrSlotOutOfRange{}

type ErrSlotOutOfRange struct {
	start, end uint64
	slot       uint64
}

func NewErrSlotOutOfRange(start, end, slot uint64) error {
	return &ErrSlotOutOfRange{start: start, end: end, slot: slot}
}

func (e *ErrSlotOutOfRange) Error() string {
	if e == nil {
		return "nil"
	}
	return fmt.Sprintf("slot %d is out of range [%d, %d]", e.slot, e.start, e.end)
}

func (e *ErrSlotOutOfRange) Is(target error) bool {
	_, ok := target.(*ErrSlotOutOfRange)
	return ok
}
