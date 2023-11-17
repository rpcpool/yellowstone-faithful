package genesis

import (
	"io"
	"time"

	bin "github.com/gagliardetto/binary"
	"github.com/rpcpool/yellowstone-faithful/radiance/runtime"
)

// Dumping ground for handwritten serialization boilerplate.
// To be removed when switching over to serde-generate.

func (g *Genesis) UnmarshalWithDecoder(decoder *bin.Decoder) (err error) {
	var raw struct {
		CreationTime   int64
		NumAccounts    uint64 `bin:"sizeof=Accounts"`
		Accounts       []AccountEntry
		NumBuiltins    uint64 `bin:"sizeof=Builtins"`
		Builtins       []BuiltinProgram
		NumRewardPools uint64 `bin:"sizeof=RewardPools"`
		RewardPools    []AccountEntry
		TicksPerSlot   uint64
		Padding00      uint64
		PohParams      runtime.PohParams
		Padding01      uint64
		Fees           runtime.FeeParams
		Rent           runtime.RentParams
		Inflation      runtime.InflationParams
		EpochSchedule  runtime.EpochSchedule
		ClusterID      uint32
	}
	if err = decoder.Decode(&raw); err != nil {
		return err
	}
	*g = Genesis{
		CreationTime:  time.Unix(raw.CreationTime, 0).UTC(),
		Accounts:      raw.Accounts,
		Builtins:      raw.Builtins,
		RewardPools:   raw.RewardPools,
		TicksPerSlot:  raw.TicksPerSlot,
		PohParams:     raw.PohParams,
		Fees:          raw.Fees,
		Rent:          raw.Rent,
		Inflation:     raw.Inflation,
		EpochSchedule: raw.EpochSchedule,
		ClusterID:     raw.ClusterID,
	}
	return nil
}

func (g *Genesis) MarshalWithEncoder(_ *bin.Encoder) (err error) {
	// TODO not implemented
	panic("not implemented")
}

func (a *AccountEntry) UnmarshalWithDecoder(decoder *bin.Decoder) (err error) {
	if err = decoder.Decode(&a.Pubkey); err != nil {
		return err
	}
	return a.Account.UnmarshalWithDecoder(decoder)
}

func (a *AccountEntry) MarshalWihEncoder(encoder *bin.Encoder) (err error) {
	if err = encoder.WriteBytes(a.Pubkey[:], false); err != nil {
		return err
	}
	return a.Account.MarshalWihEncoder(encoder)
}

func (b *BuiltinProgram) UnmarshalWithDecoder(decoder *bin.Decoder) (err error) {
	var strLen uint64
	if strLen, err = decoder.ReadUint64(bin.LE); err != nil {
		return err
	}
	if strLen > uint64(decoder.Remaining()) {
		return io.ErrUnexpectedEOF
	}
	var strBytes []byte
	if strBytes, err = decoder.ReadNBytes(int(strLen)); err != nil {
		return err
	}
	b.Key = string(strBytes)
	return decoder.Decode(&b.Pubkey)
}

func (*BuiltinProgram) MarshalWihEncoder(_ *bin.Encoder) (err error) {
	// TODO not implemented
	panic("not implemented")
}
