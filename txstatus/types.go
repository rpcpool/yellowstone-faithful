package txstatus

import (
	"encoding/binary"
	"fmt"

	bin "github.com/gagliardetto/binary"
	"github.com/gagliardetto/solana-go"
)

type Parameters struct {
	ProgramID   solana.PublicKey
	Instruction CompiledInstruction
	AccountKeys AccountKeys
	StackHeight *uint32
}

func (inst Parameters) MarshalWithEncoder(encoder *bin.Encoder) error {
	_, err := encoder.Write(inst.ProgramID[:])
	if err != nil {
		return fmt.Errorf("failed to write ProgramID: %w", err)
	}
	err = inst.Instruction.MarshalWithEncoder(encoder)
	if err != nil {
		return fmt.Errorf("failed to write Instruction: %w", err)
	}
	err = inst.AccountKeys.MarshalWithEncoder(encoder)
	if err != nil {
		return fmt.Errorf("failed to write AccountKeys: %w", err)
	}
	if inst.StackHeight != nil {
		err = encoder.WriteOption(true)
		if err != nil {
			return fmt.Errorf("failed to write Option(StackHeight): %w", err)
		}
		err = encoder.WriteUint32(*inst.StackHeight, binary.LittleEndian)
		if err != nil {
			return fmt.Errorf("failed to write StackHeight: %w", err)
		}
	} else {
		err = encoder.WriteOption(false)
		if err != nil {
			return fmt.Errorf("failed to write Option(StackHeight): %w", err)
		}
	}
	return nil
}

type CompiledInstruction struct {
	ProgramIDIndex uint8
	Accounts       solana.Uint8SliceAsNum
	Data           []byte
}

func (inst CompiledInstruction) MarshalWithEncoder(encoder *bin.Encoder) error {
	{
		// .compiled_instruction.program_id_index as uint8
		err := encoder.WriteUint8(inst.ProgramIDIndex)
		if err != nil {
			return fmt.Errorf("failed to write ProgramIDIndex: %w", err)
		}
		// .compiled_instruction.accounts:
		{
			// len uint16
			err := encoder.WriteUint16(uint16(len(inst.Accounts)), binary.LittleEndian)
			if err != nil {
				return fmt.Errorf("failed to write len(Accounts): %w", err)
			}
			// values:
			_, err = encoder.Write(inst.Accounts)
			if err != nil {
				return fmt.Errorf("failed to write Accounts: %w", err)
			}
		}
		// .compiled_instruction.data:
		{
			// len uint16
			dataLen := uint16(len(inst.Data))
			if int(dataLen) != len(inst.Data) {
				return fmt.Errorf("encoded len(Data) is larger than 16 bits")
			}
			err := encoder.WriteUint16(dataLen, binary.LittleEndian)
			if err != nil {
				return fmt.Errorf("failed to write len(Data): %w", err)
			}
			// value:
			_, err = encoder.Write(inst.Data)
			if err != nil {
				return fmt.Errorf("failed to write Data: %w", err)
			}
		}
	}
	return nil
}

type AccountKeys struct {
	StaticKeys  []solana.PublicKey
	DynamicKeys *LoadedAddresses
}

func (inst AccountKeys) MarshalWithEncoder(encoder *bin.Encoder) error {
	{
		// account_keys.static_keys:
		{
			// len uint16
			err := encoder.WriteUint16(uint16(len(inst.StaticKeys)), binary.LittleEndian)
			if err != nil {
				return fmt.Errorf("failed to write len(StaticKeys): %w", err)
			}
			// keys:
			for keyIndex, key := range inst.StaticKeys {
				// key
				_, err := encoder.Write(key[:])
				if err != nil {
					return fmt.Errorf("failed to write StaticKeys[%d]: %w", keyIndex, err)
				}
			}
		}
		// account_keys.dynamic_keys:
		if inst.DynamicKeys != nil {
			err := encoder.WriteOption(true)
			if err != nil {
				return fmt.Errorf("failed to write Option(DynamicKeys): %w", err)
			}
			err = inst.DynamicKeys.MarshalWithEncoder(encoder)
			if err != nil {
				return fmt.Errorf("failed to write DynamicKeys: %w", err)
			}
		} else {
			err := encoder.WriteOption(false)
			if err != nil {
				return fmt.Errorf("failed to write Option(DynamicKeys): %w", err)
			}
		}
	}
	return nil
}

type LoadedAddresses struct {
	Writable []solana.PublicKey
	Readonly []solana.PublicKey
}

func (inst LoadedAddresses) MarshalWithEncoder(encoder *bin.Encoder) error {
	{
		// account_keys.dynamic_keys.writable:
		{
			// len uint16
			err := encoder.WriteUint16(uint16(len(inst.Writable)), binary.LittleEndian)
			if err != nil {
				return fmt.Errorf("failed to write len(Writable): %w", err)
			}
			// keys:
			for keyIndex, key := range inst.Writable {
				_, err := encoder.Write(key[:])
				if err != nil {
					return fmt.Errorf("failed to write Writable[%d]: %w", keyIndex, err)
				}
			}
		}
		// account_keys.dynamic_keys.readonly:
		{
			// len uint16
			err := encoder.WriteUint16(uint16(len(inst.Readonly)), binary.LittleEndian)
			if err != nil {
				return fmt.Errorf("failed to write len(Readonly): %w", err)
			}
			// keys:
			for keyIndex, key := range inst.Readonly {
				_, err := encoder.Write(key[:])
				if err != nil {
					return fmt.Errorf("failed to write Readonly[%d]: %w", keyIndex, err)
				}
			}
		}
	}
	return nil
}

var DebugMode bool

func debugf(format string, args ...interface{}) {
	if DebugMode {
		fmt.Printf(format, args...)
	}
}

func debugln(args ...interface{}) {
	if DebugMode {
		fmt.Println(args...)
	}
}
