package slotindex

type Data struct {
	Slot   uint64
	Object struct {
		Offset int64
		Size   int64
	}
	Block struct {
		Offset int64
		Size   int64
	}
	Parent struct {
		Offset int64
		Size   int64
	}
	Cid             []byte
	ParentBlocktime int64
	ParentSlot      uint64
	ParentCid       []byte
	ParentBlockHash []byte
}
