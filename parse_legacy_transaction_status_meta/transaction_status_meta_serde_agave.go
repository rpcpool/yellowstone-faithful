package transaction_status_meta_serde_agave

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/bincode"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/serde"
)

var ErrSomeBytesNotRead = errors.New("Some input bytes were not read")

type CompiledInstruction struct {
	ProgramIdIndex uint8
	Accounts       []uint8
	Data           []uint8
}

func (obj *CompiledInstruction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.ProgramIdIndex); err != nil {
		return err
	}
	if err := serialize_vector_u8(obj.Accounts, serializer); err != nil {
		return err
	}
	if err := serialize_vector_u8(obj.Data, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *CompiledInstruction) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeCompiledInstruction(deserializer serde.Deserializer) (CompiledInstruction, error) {
	var obj CompiledInstruction
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("Failed to increase container depth: %w", err)
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.ProgramIdIndex = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize ProgramIdIndex (as u8): %w", err)
	}
	if val, err := deserialize_accounts_vector_u8(deserializer); err == nil {
		obj.Accounts = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Accounts (as []u8): %w", err)
	}
	if val, err := deserialize_accounts_vector_u8(deserializer); err == nil {
		obj.Data = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Data (as []u8): %w", err)
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeCompiledInstruction(input []byte) (CompiledInstruction, error) {
	if input == nil {
		var obj CompiledInstruction
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeCompiledInstruction(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type InnerInstruction struct {
	Instruction CompiledInstruction
	StackHeight *uint32
}

func (obj *InnerInstruction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := obj.Instruction.Serialize(serializer); err != nil {
		return err
	}
	if err := serialize_option_u32(obj.StackHeight, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InnerInstruction) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeInnerInstruction(deserializer serde.Deserializer) (InnerInstruction, error) {
	var obj InnerInstruction
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("Failed to increase container depth: %w", err)
	}
	if val, err := DeserializeCompiledInstruction(deserializer); err == nil {
		obj.Instruction = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Instruction (as CompiledInstruction): %w", err)
	}
	// if val, err := deserialize_option_u32(deserializer); err == nil {
	// 	obj.StackHeight = val
	// } else {
	// 	// TODO: remove StackHeight because it doesn't exist in the legacy format.
	// 	if strings.Contains(err.Error(), "invalid bool byte") {
	// 		obj.StackHeight = nil
	// 		return obj, nil
	// 	}
	// 	return obj, fmt.Errorf("Failed to deserialize StackHeight (as *uint32): %w", err)
	// }
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeInnerInstruction(input []byte) (InnerInstruction, error) {
	if input == nil {
		var obj InnerInstruction
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeInnerInstruction(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type InnerInstructions struct {
	Index        uint8
	Instructions []InnerInstruction
}

func (obj *InnerInstructions) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.Index); err != nil {
		return err
	}
	if err := serialize_vector_InnerInstruction(obj.Instructions, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InnerInstructions) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeInnerInstructions(deserializer serde.Deserializer) (InnerInstructions, error) {
	var obj InnerInstructions
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("Failed to increase container depth: %w", err)
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Index = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Index (as u8): %w", err)
	}
	if val, err := deserialize_vector_InnerInstruction(deserializer); err == nil {
		obj.Instructions = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Instructions (as []InnerInstruction): %w", err)
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeInnerInstructions(input []byte) (InnerInstructions, error) {
	if input == nil {
		var obj InnerInstructions
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeInnerInstructions(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type InstructionError interface {
	isInstructionError()
	String() string
	MarshalJSON() ([]byte, error)
	Serialize(serializer serde.Serializer) error
	BincodeSerialize() ([]byte, error)
}

func DeserializeInstructionError(deserializer serde.Deserializer) (InstructionError, error) {
	index, err := deserializer.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}

	switch index {
	case 0:
		if val, err := load_InstructionError__GenericError(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 1:
		if val, err := load_InstructionError__InvalidArgument(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 2:
		if val, err := load_InstructionError__InvalidInstructionData(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 3:
		if val, err := load_InstructionError__InvalidAccountData(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 4:
		if val, err := load_InstructionError__AccountDataTooSmall(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 5:
		if val, err := load_InstructionError__InsufficientFunds(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 6:
		if val, err := load_InstructionError__IncorrectProgramId(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 7:
		if val, err := load_InstructionError__MissingRequiredSignature(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 8:
		if val, err := load_InstructionError__AccountAlreadyInitialized(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 9:
		if val, err := load_InstructionError__UninitializedAccount(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 10:
		if val, err := load_InstructionError__UnbalancedInstruction(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 11:
		if val, err := load_InstructionError__ModifiedProgramId(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 12:
		if val, err := load_InstructionError__ExternalAccountLamportSpend(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 13:
		if val, err := load_InstructionError__ExternalAccountDataModified(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 14:
		if val, err := load_InstructionError__ReadonlyLamportChange(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 15:
		if val, err := load_InstructionError__ReadonlyDataModified(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 16:
		if val, err := load_InstructionError__DuplicateAccountIndex(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 17:
		if val, err := load_InstructionError__ExecutableModified(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 18:
		if val, err := load_InstructionError__RentEpochModified(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 19:
		if val, err := load_InstructionError__NotEnoughAccountKeys(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 20:
		if val, err := load_InstructionError__AccountDataSizeChanged(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 21:
		if val, err := load_InstructionError__AccountNotExecutable(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 22:
		if val, err := load_InstructionError__AccountBorrowFailed(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 23:
		if val, err := load_InstructionError__AccountBorrowOutstanding(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 24:
		if val, err := load_InstructionError__DuplicateAccountOutOfSync(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 25:
		if val, err := load_InstructionError__Custom(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 26:
		if val, err := load_InstructionError__InvalidError(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 27:
		if val, err := load_InstructionError__ExecutableDataModified(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 28:
		if val, err := load_InstructionError__ExecutableLamportChange(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 29:
		if val, err := load_InstructionError__ExecutableAccountNotRentExempt(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 30:
		if val, err := load_InstructionError__UnsupportedProgramId(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 31:
		if val, err := load_InstructionError__CallDepth(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 32:
		if val, err := load_InstructionError__MissingAccount(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 33:
		if val, err := load_InstructionError__ReentrancyNotAllowed(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 34:
		if val, err := load_InstructionError__MaxSeedLengthExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 35:
		if val, err := load_InstructionError__InvalidSeeds(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 36:
		if val, err := load_InstructionError__InvalidRealloc(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 37:
		if val, err := load_InstructionError__ComputationalBudgetExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 38:
		if val, err := load_InstructionError__PrivilegeEscalation(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 39:
		if val, err := load_InstructionError__ProgramEnvironmentSetupFailure(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 40:
		if val, err := load_InstructionError__ProgramFailedToComplete(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 41:
		if val, err := load_InstructionError__ProgramFailedToCompile(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 42:
		if val, err := load_InstructionError__Immutable(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 43:
		if val, err := load_InstructionError__IncorrectAuthority(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 44:
		if val, err := load_InstructionError__BorshIoError(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 45:
		if val, err := load_InstructionError__AccountNotRentExempt(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 46:
		if val, err := load_InstructionError__InvalidAccountOwner(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 47:
		if val, err := load_InstructionError__ArithmeticOverflow(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 48:
		if val, err := load_InstructionError__UnsupportedSysvar(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 49:
		if val, err := load_InstructionError__IllegalOwner(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 50:
		if val, err := load_InstructionError__MaxAccountsDataAllocationsExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 51:
		if val, err := load_InstructionError__MaxAccountsExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 52:
		if val, err := load_InstructionError__MaxInstructionTraceLengthExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 53:
		if val, err := load_InstructionError__BuiltinProgramsMustConsumeComputeUnits(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("Unknown variant index for InstructionError: %d", index)
	}
}

func BincodeDeserializeInstructionError(input []byte) (InstructionError, error) {
	if input == nil {
		var obj InstructionError
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeInstructionError(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type InstructionError__GenericError struct{}

func (*InstructionError__GenericError) isInstructionError() {}

func (obj *InstructionError__GenericError) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(0)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__GenericError) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__GenericError(deserializer serde.Deserializer) (InstructionError__GenericError, error) {
	var obj InstructionError__GenericError
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidArgument struct{}

func (*InstructionError__InvalidArgument) isInstructionError() {}

func (obj *InstructionError__InvalidArgument) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(1)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidArgument) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidArgument(deserializer serde.Deserializer) (InstructionError__InvalidArgument, error) {
	var obj InstructionError__InvalidArgument
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidInstructionData struct{}

func (*InstructionError__InvalidInstructionData) isInstructionError() {}

func (obj *InstructionError__InvalidInstructionData) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(2)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidInstructionData) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidInstructionData(deserializer serde.Deserializer) (InstructionError__InvalidInstructionData, error) {
	var obj InstructionError__InvalidInstructionData
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidAccountData struct{}

func (*InstructionError__InvalidAccountData) isInstructionError() {}

func (obj *InstructionError__InvalidAccountData) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(3)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidAccountData) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidAccountData(deserializer serde.Deserializer) (InstructionError__InvalidAccountData, error) {
	var obj InstructionError__InvalidAccountData
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountDataTooSmall struct{}

func (*InstructionError__AccountDataTooSmall) isInstructionError() {}

func (obj *InstructionError__AccountDataTooSmall) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(4)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountDataTooSmall) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountDataTooSmall(deserializer serde.Deserializer) (InstructionError__AccountDataTooSmall, error) {
	var obj InstructionError__AccountDataTooSmall
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InsufficientFunds struct{}

func (*InstructionError__InsufficientFunds) isInstructionError() {}

func (obj *InstructionError__InsufficientFunds) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(5)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InsufficientFunds) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InsufficientFunds(deserializer serde.Deserializer) (InstructionError__InsufficientFunds, error) {
	var obj InstructionError__InsufficientFunds
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__IncorrectProgramId struct{}

func (*InstructionError__IncorrectProgramId) isInstructionError() {}

func (obj *InstructionError__IncorrectProgramId) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(6)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__IncorrectProgramId) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__IncorrectProgramId(deserializer serde.Deserializer) (InstructionError__IncorrectProgramId, error) {
	var obj InstructionError__IncorrectProgramId
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MissingRequiredSignature struct{}

func (*InstructionError__MissingRequiredSignature) isInstructionError() {}

func (obj *InstructionError__MissingRequiredSignature) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(7)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MissingRequiredSignature) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MissingRequiredSignature(deserializer serde.Deserializer) (InstructionError__MissingRequiredSignature, error) {
	var obj InstructionError__MissingRequiredSignature
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountAlreadyInitialized struct{}

func (*InstructionError__AccountAlreadyInitialized) isInstructionError() {}

func (obj *InstructionError__AccountAlreadyInitialized) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(8)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountAlreadyInitialized) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountAlreadyInitialized(deserializer serde.Deserializer) (InstructionError__AccountAlreadyInitialized, error) {
	var obj InstructionError__AccountAlreadyInitialized
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__UninitializedAccount struct{}

func (*InstructionError__UninitializedAccount) isInstructionError() {}

func (obj *InstructionError__UninitializedAccount) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(9)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__UninitializedAccount) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__UninitializedAccount(deserializer serde.Deserializer) (InstructionError__UninitializedAccount, error) {
	var obj InstructionError__UninitializedAccount
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__UnbalancedInstruction struct{}

func (*InstructionError__UnbalancedInstruction) isInstructionError() {}

func (obj *InstructionError__UnbalancedInstruction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(10)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__UnbalancedInstruction) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__UnbalancedInstruction(deserializer serde.Deserializer) (InstructionError__UnbalancedInstruction, error) {
	var obj InstructionError__UnbalancedInstruction
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ModifiedProgramId struct{}

func (*InstructionError__ModifiedProgramId) isInstructionError() {}

func (obj *InstructionError__ModifiedProgramId) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(11)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ModifiedProgramId) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ModifiedProgramId(deserializer serde.Deserializer) (InstructionError__ModifiedProgramId, error) {
	var obj InstructionError__ModifiedProgramId
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExternalAccountLamportSpend struct{}

func (*InstructionError__ExternalAccountLamportSpend) isInstructionError() {}

func (obj *InstructionError__ExternalAccountLamportSpend) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(12)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExternalAccountLamportSpend) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExternalAccountLamportSpend(deserializer serde.Deserializer) (InstructionError__ExternalAccountLamportSpend, error) {
	var obj InstructionError__ExternalAccountLamportSpend
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExternalAccountDataModified struct{}

func (*InstructionError__ExternalAccountDataModified) isInstructionError() {}

func (obj *InstructionError__ExternalAccountDataModified) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(13)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExternalAccountDataModified) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExternalAccountDataModified(deserializer serde.Deserializer) (InstructionError__ExternalAccountDataModified, error) {
	var obj InstructionError__ExternalAccountDataModified
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ReadonlyLamportChange struct{}

func (*InstructionError__ReadonlyLamportChange) isInstructionError() {}

func (obj *InstructionError__ReadonlyLamportChange) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(14)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ReadonlyLamportChange) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ReadonlyLamportChange(deserializer serde.Deserializer) (InstructionError__ReadonlyLamportChange, error) {
	var obj InstructionError__ReadonlyLamportChange
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ReadonlyDataModified struct{}

func (*InstructionError__ReadonlyDataModified) isInstructionError() {}

func (obj *InstructionError__ReadonlyDataModified) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(15)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ReadonlyDataModified) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ReadonlyDataModified(deserializer serde.Deserializer) (InstructionError__ReadonlyDataModified, error) {
	var obj InstructionError__ReadonlyDataModified
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__DuplicateAccountIndex struct{}

func (*InstructionError__DuplicateAccountIndex) isInstructionError() {}

func (obj *InstructionError__DuplicateAccountIndex) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(16)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__DuplicateAccountIndex) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__DuplicateAccountIndex(deserializer serde.Deserializer) (InstructionError__DuplicateAccountIndex, error) {
	var obj InstructionError__DuplicateAccountIndex
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExecutableModified struct{}

func (*InstructionError__ExecutableModified) isInstructionError() {}

func (obj *InstructionError__ExecutableModified) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(17)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExecutableModified) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExecutableModified(deserializer serde.Deserializer) (InstructionError__ExecutableModified, error) {
	var obj InstructionError__ExecutableModified
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__RentEpochModified struct{}

func (*InstructionError__RentEpochModified) isInstructionError() {}

func (obj *InstructionError__RentEpochModified) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(18)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__RentEpochModified) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__RentEpochModified(deserializer serde.Deserializer) (InstructionError__RentEpochModified, error) {
	var obj InstructionError__RentEpochModified
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__NotEnoughAccountKeys struct{}

func (*InstructionError__NotEnoughAccountKeys) isInstructionError() {}

func (obj *InstructionError__NotEnoughAccountKeys) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(19)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__NotEnoughAccountKeys) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__NotEnoughAccountKeys(deserializer serde.Deserializer) (InstructionError__NotEnoughAccountKeys, error) {
	var obj InstructionError__NotEnoughAccountKeys
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountDataSizeChanged struct{}

func (*InstructionError__AccountDataSizeChanged) isInstructionError() {}

func (obj *InstructionError__AccountDataSizeChanged) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(20)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountDataSizeChanged) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountDataSizeChanged(deserializer serde.Deserializer) (InstructionError__AccountDataSizeChanged, error) {
	var obj InstructionError__AccountDataSizeChanged
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountNotExecutable struct{}

func (*InstructionError__AccountNotExecutable) isInstructionError() {}

func (obj *InstructionError__AccountNotExecutable) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(21)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountNotExecutable) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountNotExecutable(deserializer serde.Deserializer) (InstructionError__AccountNotExecutable, error) {
	var obj InstructionError__AccountNotExecutable
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountBorrowFailed struct{}

func (*InstructionError__AccountBorrowFailed) isInstructionError() {}

func (obj *InstructionError__AccountBorrowFailed) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(22)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountBorrowFailed) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountBorrowFailed(deserializer serde.Deserializer) (InstructionError__AccountBorrowFailed, error) {
	var obj InstructionError__AccountBorrowFailed
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__AccountBorrowOutstanding struct{}

func (*InstructionError__AccountBorrowOutstanding) isInstructionError() {}

func (obj *InstructionError__AccountBorrowOutstanding) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(23)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountBorrowOutstanding) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountBorrowOutstanding(deserializer serde.Deserializer) (InstructionError__AccountBorrowOutstanding, error) {
	var obj InstructionError__AccountBorrowOutstanding
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__DuplicateAccountOutOfSync struct{}

func (*InstructionError__DuplicateAccountOutOfSync) isInstructionError() {}

func (obj *InstructionError__DuplicateAccountOutOfSync) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(24)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__DuplicateAccountOutOfSync) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__DuplicateAccountOutOfSync(deserializer serde.Deserializer) (InstructionError__DuplicateAccountOutOfSync, error) {
	var obj InstructionError__DuplicateAccountOutOfSync
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__Custom uint32

func (*InstructionError__Custom) isInstructionError() {}

func (obj *InstructionError__Custom) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(25)
	if err := serializer.SerializeU32(((uint32)(*obj))); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__Custom) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__Custom(deserializer serde.Deserializer) (InstructionError__Custom, error) {
	var obj uint32
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return (InstructionError__Custom)(obj), err
	}
	if val, err := deserializer.DeserializeU32(); err == nil {
		obj = val
	} else {
		return ((InstructionError__Custom)(obj)), err
	}
	deserializer.DecreaseContainerDepth()
	return (InstructionError__Custom)(obj), nil
}

type InstructionError__InvalidError struct{}

func (*InstructionError__InvalidError) isInstructionError() {}

func (obj *InstructionError__InvalidError) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(26)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidError) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidError(deserializer serde.Deserializer) (InstructionError__InvalidError, error) {
	var obj InstructionError__InvalidError
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExecutableDataModified struct{}

func (*InstructionError__ExecutableDataModified) isInstructionError() {}

func (obj *InstructionError__ExecutableDataModified) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(27)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExecutableDataModified) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExecutableDataModified(deserializer serde.Deserializer) (InstructionError__ExecutableDataModified, error) {
	var obj InstructionError__ExecutableDataModified
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExecutableLamportChange struct{}

func (*InstructionError__ExecutableLamportChange) isInstructionError() {}

func (obj *InstructionError__ExecutableLamportChange) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(28)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExecutableLamportChange) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExecutableLamportChange(deserializer serde.Deserializer) (InstructionError__ExecutableLamportChange, error) {
	var obj InstructionError__ExecutableLamportChange
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ExecutableAccountNotRentExempt struct{}

func (*InstructionError__ExecutableAccountNotRentExempt) isInstructionError() {}

func (obj *InstructionError__ExecutableAccountNotRentExempt) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(29)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ExecutableAccountNotRentExempt) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ExecutableAccountNotRentExempt(deserializer serde.Deserializer) (InstructionError__ExecutableAccountNotRentExempt, error) {
	var obj InstructionError__ExecutableAccountNotRentExempt
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__UnsupportedProgramId struct{}

func (*InstructionError__UnsupportedProgramId) isInstructionError() {}

func (obj *InstructionError__UnsupportedProgramId) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(30)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__UnsupportedProgramId) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__UnsupportedProgramId(deserializer serde.Deserializer) (InstructionError__UnsupportedProgramId, error) {
	var obj InstructionError__UnsupportedProgramId
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__CallDepth struct{}

func (*InstructionError__CallDepth) isInstructionError() {}

func (obj *InstructionError__CallDepth) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(31)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__CallDepth) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__CallDepth(deserializer serde.Deserializer) (InstructionError__CallDepth, error) {
	var obj InstructionError__CallDepth
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MissingAccount struct{}

func (*InstructionError__MissingAccount) isInstructionError() {}

func (obj *InstructionError__MissingAccount) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(32)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MissingAccount) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MissingAccount(deserializer serde.Deserializer) (InstructionError__MissingAccount, error) {
	var obj InstructionError__MissingAccount
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ReentrancyNotAllowed struct{}

func (*InstructionError__ReentrancyNotAllowed) isInstructionError() {}

func (obj *InstructionError__ReentrancyNotAllowed) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(33)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ReentrancyNotAllowed) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ReentrancyNotAllowed(deserializer serde.Deserializer) (InstructionError__ReentrancyNotAllowed, error) {
	var obj InstructionError__ReentrancyNotAllowed
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MaxSeedLengthExceeded struct{}

func (*InstructionError__MaxSeedLengthExceeded) isInstructionError() {}

func (obj *InstructionError__MaxSeedLengthExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(34)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MaxSeedLengthExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MaxSeedLengthExceeded(deserializer serde.Deserializer) (InstructionError__MaxSeedLengthExceeded, error) {
	var obj InstructionError__MaxSeedLengthExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidSeeds struct{}

func (*InstructionError__InvalidSeeds) isInstructionError() {}

func (obj *InstructionError__InvalidSeeds) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(35)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidSeeds) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidSeeds(deserializer serde.Deserializer) (InstructionError__InvalidSeeds, error) {
	var obj InstructionError__InvalidSeeds
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidRealloc struct{}

func (*InstructionError__InvalidRealloc) isInstructionError() {}

func (obj *InstructionError__InvalidRealloc) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(36)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidRealloc) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidRealloc(deserializer serde.Deserializer) (InstructionError__InvalidRealloc, error) {
	var obj InstructionError__InvalidRealloc
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ComputationalBudgetExceeded struct{}

func (*InstructionError__ComputationalBudgetExceeded) isInstructionError() {}

func (obj *InstructionError__ComputationalBudgetExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(37)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ComputationalBudgetExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ComputationalBudgetExceeded(deserializer serde.Deserializer) (InstructionError__ComputationalBudgetExceeded, error) {
	var obj InstructionError__ComputationalBudgetExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__PrivilegeEscalation struct{}

func (*InstructionError__PrivilegeEscalation) isInstructionError() {}

func (obj *InstructionError__PrivilegeEscalation) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(38)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__PrivilegeEscalation) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__PrivilegeEscalation(deserializer serde.Deserializer) (InstructionError__PrivilegeEscalation, error) {
	var obj InstructionError__PrivilegeEscalation
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ProgramEnvironmentSetupFailure struct{}

func (*InstructionError__ProgramEnvironmentSetupFailure) isInstructionError() {}

func (obj *InstructionError__ProgramEnvironmentSetupFailure) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(39)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ProgramEnvironmentSetupFailure) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ProgramEnvironmentSetupFailure(deserializer serde.Deserializer) (InstructionError__ProgramEnvironmentSetupFailure, error) {
	var obj InstructionError__ProgramEnvironmentSetupFailure
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ProgramFailedToComplete struct{}

func (*InstructionError__ProgramFailedToComplete) isInstructionError() {}

func (obj *InstructionError__ProgramFailedToComplete) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(40)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ProgramFailedToComplete) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ProgramFailedToComplete(deserializer serde.Deserializer) (InstructionError__ProgramFailedToComplete, error) {
	var obj InstructionError__ProgramFailedToComplete
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ProgramFailedToCompile struct{}

func (*InstructionError__ProgramFailedToCompile) isInstructionError() {}

func (obj *InstructionError__ProgramFailedToCompile) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(41)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ProgramFailedToCompile) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ProgramFailedToCompile(deserializer serde.Deserializer) (InstructionError__ProgramFailedToCompile, error) {
	var obj InstructionError__ProgramFailedToCompile
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__Immutable struct{}

func (*InstructionError__Immutable) isInstructionError() {}

func (obj *InstructionError__Immutable) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(42)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__Immutable) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__Immutable(deserializer serde.Deserializer) (InstructionError__Immutable, error) {
	var obj InstructionError__Immutable
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__IncorrectAuthority struct{}

func (*InstructionError__IncorrectAuthority) isInstructionError() {}

func (obj *InstructionError__IncorrectAuthority) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(43)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__IncorrectAuthority) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__IncorrectAuthority(deserializer serde.Deserializer) (InstructionError__IncorrectAuthority, error) {
	var obj InstructionError__IncorrectAuthority
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__BorshIoError string

func (*InstructionError__BorshIoError) isInstructionError() {}

func (obj *InstructionError__BorshIoError) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(44)
	if err := serializer.SerializeStr(((string)(*obj))); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__BorshIoError) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__BorshIoError(deserializer serde.Deserializer) (InstructionError__BorshIoError, error) {
	var obj string
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return (InstructionError__BorshIoError)(obj), err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj = val
	} else {
		return ((InstructionError__BorshIoError)(obj)), err
	}
	deserializer.DecreaseContainerDepth()
	return (InstructionError__BorshIoError)(obj), nil
}

type InstructionError__AccountNotRentExempt struct{}

func (*InstructionError__AccountNotRentExempt) isInstructionError() {}

func (obj *InstructionError__AccountNotRentExempt) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(45)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__AccountNotRentExempt) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__AccountNotRentExempt(deserializer serde.Deserializer) (InstructionError__AccountNotRentExempt, error) {
	var obj InstructionError__AccountNotRentExempt
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__InvalidAccountOwner struct{}

func (*InstructionError__InvalidAccountOwner) isInstructionError() {}

func (obj *InstructionError__InvalidAccountOwner) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(46)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__InvalidAccountOwner) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__InvalidAccountOwner(deserializer serde.Deserializer) (InstructionError__InvalidAccountOwner, error) {
	var obj InstructionError__InvalidAccountOwner
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__ArithmeticOverflow struct{}

func (*InstructionError__ArithmeticOverflow) isInstructionError() {}

func (obj *InstructionError__ArithmeticOverflow) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(47)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__ArithmeticOverflow) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__ArithmeticOverflow(deserializer serde.Deserializer) (InstructionError__ArithmeticOverflow, error) {
	var obj InstructionError__ArithmeticOverflow
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__UnsupportedSysvar struct{}

func (*InstructionError__UnsupportedSysvar) isInstructionError() {}

func (obj *InstructionError__UnsupportedSysvar) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(48)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__UnsupportedSysvar) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__UnsupportedSysvar(deserializer serde.Deserializer) (InstructionError__UnsupportedSysvar, error) {
	var obj InstructionError__UnsupportedSysvar
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__IllegalOwner struct{}

func (*InstructionError__IllegalOwner) isInstructionError() {}

func (obj *InstructionError__IllegalOwner) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(49)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__IllegalOwner) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__IllegalOwner(deserializer serde.Deserializer) (InstructionError__IllegalOwner, error) {
	var obj InstructionError__IllegalOwner
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MaxAccountsDataAllocationsExceeded struct{}

func (*InstructionError__MaxAccountsDataAllocationsExceeded) isInstructionError() {}

func (obj *InstructionError__MaxAccountsDataAllocationsExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(50)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MaxAccountsDataAllocationsExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MaxAccountsDataAllocationsExceeded(deserializer serde.Deserializer) (InstructionError__MaxAccountsDataAllocationsExceeded, error) {
	var obj InstructionError__MaxAccountsDataAllocationsExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MaxAccountsExceeded struct{}

func (*InstructionError__MaxAccountsExceeded) isInstructionError() {}

func (obj *InstructionError__MaxAccountsExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(51)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MaxAccountsExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MaxAccountsExceeded(deserializer serde.Deserializer) (InstructionError__MaxAccountsExceeded, error) {
	var obj InstructionError__MaxAccountsExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__MaxInstructionTraceLengthExceeded struct{}

func (*InstructionError__MaxInstructionTraceLengthExceeded) isInstructionError() {}

func (obj *InstructionError__MaxInstructionTraceLengthExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(52)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__MaxInstructionTraceLengthExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__MaxInstructionTraceLengthExceeded(deserializer serde.Deserializer) (InstructionError__MaxInstructionTraceLengthExceeded, error) {
	var obj InstructionError__MaxInstructionTraceLengthExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type InstructionError__BuiltinProgramsMustConsumeComputeUnits struct{}

func (*InstructionError__BuiltinProgramsMustConsumeComputeUnits) isInstructionError() {}

func (obj *InstructionError__BuiltinProgramsMustConsumeComputeUnits) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(53)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *InstructionError__BuiltinProgramsMustConsumeComputeUnits) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_InstructionError__BuiltinProgramsMustConsumeComputeUnits(deserializer serde.Deserializer) (InstructionError__BuiltinProgramsMustConsumeComputeUnits, error) {
	var obj InstructionError__BuiltinProgramsMustConsumeComputeUnits
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type LoadedAddresses struct {
	Writable []Pubkey
	Readonly []Pubkey
}

func (obj *LoadedAddresses) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serialize_vector_Pubkey(obj.Writable, serializer); err != nil {
		return err
	}
	if err := serialize_vector_Pubkey(obj.Readonly, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *LoadedAddresses) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeLoadedAddresses(deserializer serde.Deserializer) (LoadedAddresses, error) {
	var obj LoadedAddresses
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserialize_vector_Pubkey(deserializer); err == nil {
		obj.Writable = val
	} else {
		return obj, err
	}
	if val, err := deserialize_vector_Pubkey(deserializer); err == nil {
		obj.Readonly = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeLoadedAddresses(input []byte) (LoadedAddresses, error) {
	if input == nil {
		var obj LoadedAddresses
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeLoadedAddresses(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type Pubkey [32]uint8

func (obj *Pubkey) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serialize_array32_u8_array((([32]uint8)(*obj)), serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *Pubkey) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializePubkey(deserializer serde.Deserializer) (Pubkey, error) {
	var obj [32]uint8
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return (Pubkey)(obj), err
	}
	if val, err := deserialize_array32_u8_array(deserializer); err == nil {
		obj = val
	} else {
		return ((Pubkey)(obj)), err
	}
	deserializer.DecreaseContainerDepth()
	return (Pubkey)(obj), nil
}

func BincodeDeserializePubkey(input []byte) (Pubkey, error) {
	if input == nil {
		var obj Pubkey
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializePubkey(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type Result interface {
	isResult()
	Serialize(serializer serde.Serializer) error
	BincodeSerialize() ([]byte, error)
}

func DeserializeResult(deserializer serde.Deserializer) (Result, error) {
	index, err := deserializer.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}

	switch index {
	case 0:
		if val, err := load_Result__Ok(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 1:
		if val, err := load_Result__Err(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("Unknown variant index for Result: %d", index)
	}
}

func BincodeDeserializeResult(input []byte) (Result, error) {
	if input == nil {
		var obj Result
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeResult(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type Result__Ok struct{}

func (*Result__Ok) isResult() {}

func (obj *Result__Ok) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(0)
	if err := serializer.SerializeUnit(((struct{})(*obj))); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *Result__Ok) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_Result__Ok(deserializer serde.Deserializer) (Result__Ok, error) {
	var obj struct{}
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return (Result__Ok)(obj), err
	}
	if val, err := deserializer.DeserializeUnit(); err == nil {
		obj = val
	} else {
		return ((Result__Ok)(obj)), err
	}
	deserializer.DecreaseContainerDepth()
	return (Result__Ok)(obj), nil
}

type Result__Err struct {
	Value TransactionError
}

func (*Result__Err) isResult() {}

func (obj *Result__Err) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(1)
	if err := obj.Value.Serialize(serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *Result__Err) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_Result__Err(deserializer serde.Deserializer) (Result__Err, error) {
	var obj Result__Err
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := DeserializeTransactionError(deserializer); err == nil {
		obj.Value = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type Reward struct {
	Pubkey      string
	Lamports    int64
	PostBalance uint64
	RewardType  *RewardType
	Commission  *uint8
}

func (obj *Reward) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.Pubkey); err != nil {
		return err
	}
	if err := serializer.SerializeI64(obj.Lamports); err != nil {
		return err
	}
	if err := serializer.SerializeU64(obj.PostBalance); err != nil {
		return err
	}
	if err := serialize_option_RewardType(obj.RewardType, serializer); err != nil {
		return err
	}
	if err := serialize_option_u8(obj.Commission, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *Reward) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeReward(deserializer serde.Deserializer) (Reward, error) {
	var obj Reward
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.Pubkey = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeI64(); err == nil {
		obj.Lamports = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU64(); err == nil {
		obj.PostBalance = val
	} else {
		return obj, err
	}
	if val, err := deserialize_option_RewardType(deserializer); err == nil {
		obj.RewardType = val
	} else {
		return obj, err
	}
	if val, err := deserialize_option_u8(deserializer); err == nil {
		obj.Commission = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeReward(input []byte) (Reward, error) {
	if input == nil {
		var obj Reward
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeReward(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type RewardType interface {
	isRewardType()
	Serialize(serializer serde.Serializer) error
	BincodeSerialize() ([]byte, error)
}

func DeserializeRewardType(deserializer serde.Deserializer) (RewardType, error) {
	index, err := deserializer.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}

	switch index {
	case 0:
		if val, err := load_RewardType__Fee(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 1:
		if val, err := load_RewardType__Rent(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 2:
		if val, err := load_RewardType__Staking(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 3:
		if val, err := load_RewardType__Voting(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("Unknown variant index for RewardType: %d", index)
	}
}

func BincodeDeserializeRewardType(input []byte) (RewardType, error) {
	if input == nil {
		var obj RewardType
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeRewardType(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type RewardType__Fee struct{}

func (*RewardType__Fee) isRewardType() {}

func (obj *RewardType__Fee) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(0)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *RewardType__Fee) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_RewardType__Fee(deserializer serde.Deserializer) (RewardType__Fee, error) {
	var obj RewardType__Fee
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type RewardType__Rent struct{}

func (*RewardType__Rent) isRewardType() {}

func (obj *RewardType__Rent) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(1)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *RewardType__Rent) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_RewardType__Rent(deserializer serde.Deserializer) (RewardType__Rent, error) {
	var obj RewardType__Rent
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type RewardType__Staking struct{}

func (*RewardType__Staking) isRewardType() {}

func (obj *RewardType__Staking) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(2)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *RewardType__Staking) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_RewardType__Staking(deserializer serde.Deserializer) (RewardType__Staking, error) {
	var obj RewardType__Staking
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type RewardType__Voting struct{}

func (*RewardType__Voting) isRewardType() {}

func (obj *RewardType__Voting) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(3)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *RewardType__Voting) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_RewardType__Voting(deserializer serde.Deserializer) (RewardType__Voting, error) {
	var obj RewardType__Voting
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError interface {
	isTransactionError()
	String() string
	MarshalJSON() ([]byte, error)
	Serialize(serializer serde.Serializer) error
	BincodeSerialize() ([]byte, error)
}

func DeserializeTransactionError(deserializer serde.Deserializer) (TransactionError, error) {
	index, err := deserializer.DeserializeVariantIndex()
	if err != nil {
		return nil, err
	}

	switch index {
	case 0:
		if val, err := load_TransactionError__AccountInUse(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 1:
		if val, err := load_TransactionError__AccountLoadedTwice(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 2:
		if val, err := load_TransactionError__AccountNotFound(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 3:
		if val, err := load_TransactionError__ProgramAccountNotFound(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 4:
		if val, err := load_TransactionError__InsufficientFundsForFee(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 5:
		if val, err := load_TransactionError__InvalidAccountForFee(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 6:
		if val, err := load_TransactionError__AlreadyProcessed(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 7:
		if val, err := load_TransactionError__BlockhashNotFound(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 8:
		if val, err := load_TransactionError__InstructionError(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 9:
		if val, err := load_TransactionError__CallChainTooDeep(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 10:
		if val, err := load_TransactionError__MissingSignatureForFee(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 11:
		if val, err := load_TransactionError__InvalidAccountIndex(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 12:
		if val, err := load_TransactionError__SignatureFailure(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 13:
		if val, err := load_TransactionError__InvalidProgramForExecution(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 14:
		if val, err := load_TransactionError__SanitizeFailure(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 15:
		if val, err := load_TransactionError__ClusterMaintenance(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 16:
		if val, err := load_TransactionError__AccountBorrowOutstanding(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 17:
		if val, err := load_TransactionError__WouldExceedMaxBlockCostLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 18:
		if val, err := load_TransactionError__UnsupportedVersion(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 19:
		if val, err := load_TransactionError__InvalidWritableAccount(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 20:
		if val, err := load_TransactionError__WouldExceedMaxAccountCostLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 21:
		if val, err := load_TransactionError__WouldExceedAccountDataBlockLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 22:
		if val, err := load_TransactionError__TooManyAccountLocks(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 23:
		if val, err := load_TransactionError__AddressLookupTableNotFound(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 24:
		if val, err := load_TransactionError__InvalidAddressLookupTableOwner(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 25:
		if val, err := load_TransactionError__InvalidAddressLookupTableData(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 26:
		if val, err := load_TransactionError__InvalidAddressLookupTableIndex(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 27:
		if val, err := load_TransactionError__InvalidRentPayingAccount(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 28:
		if val, err := load_TransactionError__WouldExceedMaxVoteCostLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 29:
		if val, err := load_TransactionError__WouldExceedAccountDataTotalLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 30:
		if val, err := load_TransactionError__DuplicateInstruction(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 31:
		if val, err := load_TransactionError__InsufficientFundsForRent(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 32:
		if val, err := load_TransactionError__MaxLoadedAccountsDataSizeExceeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 33:
		if val, err := load_TransactionError__InvalidLoadedAccountsDataSizeLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 34:
		if val, err := load_TransactionError__ResanitizationNeeded(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 35:
		if val, err := load_TransactionError__ProgramExecutionTemporarilyRestricted(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 36:
		if val, err := load_TransactionError__UnbalancedTransaction(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 37:
		if val, err := load_TransactionError__ProgramCacheHitMaxLimit(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	case 38:
		if val, err := load_TransactionError__CommitCancelled(deserializer); err == nil {
			return &val, nil
		} else {
			return nil, err
		}

	default:
		return nil, fmt.Errorf("Unknown variant index for TransactionError: %d", index)
	}
}

func BincodeDeserializeTransactionError(input []byte) (TransactionError, error) {
	if input == nil {
		var obj TransactionError
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeTransactionError(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type TransactionError__AccountInUse struct{}

func (*TransactionError__AccountInUse) isTransactionError() {}

func (obj *TransactionError__AccountInUse) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(0)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AccountInUse) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AccountInUse(deserializer serde.Deserializer) (TransactionError__AccountInUse, error) {
	var obj TransactionError__AccountInUse
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__AccountLoadedTwice struct{}

func (*TransactionError__AccountLoadedTwice) isTransactionError() {}

func (obj *TransactionError__AccountLoadedTwice) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(1)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AccountLoadedTwice) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AccountLoadedTwice(deserializer serde.Deserializer) (TransactionError__AccountLoadedTwice, error) {
	var obj TransactionError__AccountLoadedTwice
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__AccountNotFound struct{}

func (*TransactionError__AccountNotFound) isTransactionError() {}

func (obj *TransactionError__AccountNotFound) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(2)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AccountNotFound) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AccountNotFound(deserializer serde.Deserializer) (TransactionError__AccountNotFound, error) {
	var obj TransactionError__AccountNotFound
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__ProgramAccountNotFound struct{}

func (*TransactionError__ProgramAccountNotFound) isTransactionError() {}

func (obj *TransactionError__ProgramAccountNotFound) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(3)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__ProgramAccountNotFound) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__ProgramAccountNotFound(deserializer serde.Deserializer) (TransactionError__ProgramAccountNotFound, error) {
	var obj TransactionError__ProgramAccountNotFound
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InsufficientFundsForFee struct{}

func (*TransactionError__InsufficientFundsForFee) isTransactionError() {}

func (obj *TransactionError__InsufficientFundsForFee) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(4)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InsufficientFundsForFee) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InsufficientFundsForFee(deserializer serde.Deserializer) (TransactionError__InsufficientFundsForFee, error) {
	var obj TransactionError__InsufficientFundsForFee
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidAccountForFee struct{}

func (*TransactionError__InvalidAccountForFee) isTransactionError() {}

func (obj *TransactionError__InvalidAccountForFee) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(5)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidAccountForFee) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidAccountForFee(deserializer serde.Deserializer) (TransactionError__InvalidAccountForFee, error) {
	var obj TransactionError__InvalidAccountForFee
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__AlreadyProcessed struct{}

func (*TransactionError__AlreadyProcessed) isTransactionError() {}

func (obj *TransactionError__AlreadyProcessed) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(6)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AlreadyProcessed) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AlreadyProcessed(deserializer serde.Deserializer) (TransactionError__AlreadyProcessed, error) {
	var obj TransactionError__AlreadyProcessed
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__BlockhashNotFound struct{}

func (*TransactionError__BlockhashNotFound) isTransactionError() {}

func (obj *TransactionError__BlockhashNotFound) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(7)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__BlockhashNotFound) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__BlockhashNotFound(deserializer serde.Deserializer) (TransactionError__BlockhashNotFound, error) {
	var obj TransactionError__BlockhashNotFound
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InstructionError struct {
	ErrorCode uint8
	Error     InstructionError
}

func (*TransactionError__InstructionError) isTransactionError() {}

func (obj *TransactionError__InstructionError) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(8)
	if err := serializer.SerializeU8(obj.ErrorCode); err != nil {
		return err
	}
	if err := obj.Error.Serialize(serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InstructionError) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InstructionError(deserializer serde.Deserializer) (TransactionError__InstructionError, error) {
	var obj TransactionError__InstructionError
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.ErrorCode = val
	} else {
		return obj, err
	}
	if val, err := DeserializeInstructionError(deserializer); err == nil {
		obj.Error = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__CallChainTooDeep struct{}

func (*TransactionError__CallChainTooDeep) isTransactionError() {}

func (obj *TransactionError__CallChainTooDeep) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(9)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__CallChainTooDeep) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__CallChainTooDeep(deserializer serde.Deserializer) (TransactionError__CallChainTooDeep, error) {
	var obj TransactionError__CallChainTooDeep
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__MissingSignatureForFee struct{}

func (*TransactionError__MissingSignatureForFee) isTransactionError() {}

func (obj *TransactionError__MissingSignatureForFee) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(10)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__MissingSignatureForFee) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__MissingSignatureForFee(deserializer serde.Deserializer) (TransactionError__MissingSignatureForFee, error) {
	var obj TransactionError__MissingSignatureForFee
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidAccountIndex struct{}

func (*TransactionError__InvalidAccountIndex) isTransactionError() {}

func (obj *TransactionError__InvalidAccountIndex) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(11)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidAccountIndex) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidAccountIndex(deserializer serde.Deserializer) (TransactionError__InvalidAccountIndex, error) {
	var obj TransactionError__InvalidAccountIndex
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__SignatureFailure struct{}

func (*TransactionError__SignatureFailure) isTransactionError() {}

func (obj *TransactionError__SignatureFailure) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(12)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__SignatureFailure) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__SignatureFailure(deserializer serde.Deserializer) (TransactionError__SignatureFailure, error) {
	var obj TransactionError__SignatureFailure
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidProgramForExecution struct{}

func (*TransactionError__InvalidProgramForExecution) isTransactionError() {}

func (obj *TransactionError__InvalidProgramForExecution) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(13)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidProgramForExecution) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidProgramForExecution(deserializer serde.Deserializer) (TransactionError__InvalidProgramForExecution, error) {
	var obj TransactionError__InvalidProgramForExecution
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__SanitizeFailure struct{}

func (*TransactionError__SanitizeFailure) isTransactionError() {}

func (obj *TransactionError__SanitizeFailure) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(14)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__SanitizeFailure) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__SanitizeFailure(deserializer serde.Deserializer) (TransactionError__SanitizeFailure, error) {
	var obj TransactionError__SanitizeFailure
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__ClusterMaintenance struct{}

func (*TransactionError__ClusterMaintenance) isTransactionError() {}

func (obj *TransactionError__ClusterMaintenance) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(15)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__ClusterMaintenance) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__ClusterMaintenance(deserializer serde.Deserializer) (TransactionError__ClusterMaintenance, error) {
	var obj TransactionError__ClusterMaintenance
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__AccountBorrowOutstanding struct{}

func (*TransactionError__AccountBorrowOutstanding) isTransactionError() {}

func (obj *TransactionError__AccountBorrowOutstanding) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(16)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AccountBorrowOutstanding) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AccountBorrowOutstanding(deserializer serde.Deserializer) (TransactionError__AccountBorrowOutstanding, error) {
	var obj TransactionError__AccountBorrowOutstanding
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__WouldExceedMaxBlockCostLimit struct{}

func (*TransactionError__WouldExceedMaxBlockCostLimit) isTransactionError() {}

func (obj *TransactionError__WouldExceedMaxBlockCostLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(17)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__WouldExceedMaxBlockCostLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__WouldExceedMaxBlockCostLimit(deserializer serde.Deserializer) (TransactionError__WouldExceedMaxBlockCostLimit, error) {
	var obj TransactionError__WouldExceedMaxBlockCostLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__UnsupportedVersion struct{}

func (*TransactionError__UnsupportedVersion) isTransactionError() {}

func (obj *TransactionError__UnsupportedVersion) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(18)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__UnsupportedVersion) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__UnsupportedVersion(deserializer serde.Deserializer) (TransactionError__UnsupportedVersion, error) {
	var obj TransactionError__UnsupportedVersion
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidWritableAccount struct{}

func (*TransactionError__InvalidWritableAccount) isTransactionError() {}

func (obj *TransactionError__InvalidWritableAccount) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(19)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidWritableAccount) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidWritableAccount(deserializer serde.Deserializer) (TransactionError__InvalidWritableAccount, error) {
	var obj TransactionError__InvalidWritableAccount
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__WouldExceedMaxAccountCostLimit struct{}

func (*TransactionError__WouldExceedMaxAccountCostLimit) isTransactionError() {}

func (obj *TransactionError__WouldExceedMaxAccountCostLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(20)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__WouldExceedMaxAccountCostLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__WouldExceedMaxAccountCostLimit(deserializer serde.Deserializer) (TransactionError__WouldExceedMaxAccountCostLimit, error) {
	var obj TransactionError__WouldExceedMaxAccountCostLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__WouldExceedAccountDataBlockLimit struct{}

func (*TransactionError__WouldExceedAccountDataBlockLimit) isTransactionError() {}

func (obj *TransactionError__WouldExceedAccountDataBlockLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(21)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__WouldExceedAccountDataBlockLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__WouldExceedAccountDataBlockLimit(deserializer serde.Deserializer) (TransactionError__WouldExceedAccountDataBlockLimit, error) {
	var obj TransactionError__WouldExceedAccountDataBlockLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__TooManyAccountLocks struct{}

func (*TransactionError__TooManyAccountLocks) isTransactionError() {}

func (obj *TransactionError__TooManyAccountLocks) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(22)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__TooManyAccountLocks) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__TooManyAccountLocks(deserializer serde.Deserializer) (TransactionError__TooManyAccountLocks, error) {
	var obj TransactionError__TooManyAccountLocks
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__AddressLookupTableNotFound struct{}

func (*TransactionError__AddressLookupTableNotFound) isTransactionError() {}

func (obj *TransactionError__AddressLookupTableNotFound) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(23)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__AddressLookupTableNotFound) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__AddressLookupTableNotFound(deserializer serde.Deserializer) (TransactionError__AddressLookupTableNotFound, error) {
	var obj TransactionError__AddressLookupTableNotFound
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidAddressLookupTableOwner struct{}

func (*TransactionError__InvalidAddressLookupTableOwner) isTransactionError() {}

func (obj *TransactionError__InvalidAddressLookupTableOwner) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(24)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidAddressLookupTableOwner) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidAddressLookupTableOwner(deserializer serde.Deserializer) (TransactionError__InvalidAddressLookupTableOwner, error) {
	var obj TransactionError__InvalidAddressLookupTableOwner
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidAddressLookupTableData struct{}

func (*TransactionError__InvalidAddressLookupTableData) isTransactionError() {}

func (obj *TransactionError__InvalidAddressLookupTableData) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(25)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidAddressLookupTableData) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidAddressLookupTableData(deserializer serde.Deserializer) (TransactionError__InvalidAddressLookupTableData, error) {
	var obj TransactionError__InvalidAddressLookupTableData
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidAddressLookupTableIndex struct{}

func (*TransactionError__InvalidAddressLookupTableIndex) isTransactionError() {}

func (obj *TransactionError__InvalidAddressLookupTableIndex) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(26)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidAddressLookupTableIndex) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidAddressLookupTableIndex(deserializer serde.Deserializer) (TransactionError__InvalidAddressLookupTableIndex, error) {
	var obj TransactionError__InvalidAddressLookupTableIndex
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidRentPayingAccount struct{}

func (*TransactionError__InvalidRentPayingAccount) isTransactionError() {}

func (obj *TransactionError__InvalidRentPayingAccount) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(27)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidRentPayingAccount) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidRentPayingAccount(deserializer serde.Deserializer) (TransactionError__InvalidRentPayingAccount, error) {
	var obj TransactionError__InvalidRentPayingAccount
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__WouldExceedMaxVoteCostLimit struct{}

func (*TransactionError__WouldExceedMaxVoteCostLimit) isTransactionError() {}

func (obj *TransactionError__WouldExceedMaxVoteCostLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(28)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__WouldExceedMaxVoteCostLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__WouldExceedMaxVoteCostLimit(deserializer serde.Deserializer) (TransactionError__WouldExceedMaxVoteCostLimit, error) {
	var obj TransactionError__WouldExceedMaxVoteCostLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__WouldExceedAccountDataTotalLimit struct{}

func (*TransactionError__WouldExceedAccountDataTotalLimit) isTransactionError() {}

func (obj *TransactionError__WouldExceedAccountDataTotalLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(29)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__WouldExceedAccountDataTotalLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__WouldExceedAccountDataTotalLimit(deserializer serde.Deserializer) (TransactionError__WouldExceedAccountDataTotalLimit, error) {
	var obj TransactionError__WouldExceedAccountDataTotalLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__DuplicateInstruction uint8

func (*TransactionError__DuplicateInstruction) isTransactionError() {}

func (obj *TransactionError__DuplicateInstruction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(30)
	if err := serializer.SerializeU8(((uint8)(*obj))); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__DuplicateInstruction) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__DuplicateInstruction(deserializer serde.Deserializer) (TransactionError__DuplicateInstruction, error) {
	var obj uint8
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return (TransactionError__DuplicateInstruction)(obj), err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj = val
	} else {
		return ((TransactionError__DuplicateInstruction)(obj)), err
	}
	deserializer.DecreaseContainerDepth()
	return (TransactionError__DuplicateInstruction)(obj), nil
}

type TransactionError__InsufficientFundsForRent struct {
	AccountIndex uint8
}

func (*TransactionError__InsufficientFundsForRent) isTransactionError() {}

func (obj *TransactionError__InsufficientFundsForRent) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(31)
	if err := serializer.SerializeU8(obj.AccountIndex); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InsufficientFundsForRent) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InsufficientFundsForRent(deserializer serde.Deserializer) (TransactionError__InsufficientFundsForRent, error) {
	var obj TransactionError__InsufficientFundsForRent
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.AccountIndex = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__MaxLoadedAccountsDataSizeExceeded struct{}

func (*TransactionError__MaxLoadedAccountsDataSizeExceeded) isTransactionError() {}

func (obj *TransactionError__MaxLoadedAccountsDataSizeExceeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(32)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__MaxLoadedAccountsDataSizeExceeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__MaxLoadedAccountsDataSizeExceeded(deserializer serde.Deserializer) (TransactionError__MaxLoadedAccountsDataSizeExceeded, error) {
	var obj TransactionError__MaxLoadedAccountsDataSizeExceeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__InvalidLoadedAccountsDataSizeLimit struct{}

func (*TransactionError__InvalidLoadedAccountsDataSizeLimit) isTransactionError() {}

func (obj *TransactionError__InvalidLoadedAccountsDataSizeLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(33)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__InvalidLoadedAccountsDataSizeLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__InvalidLoadedAccountsDataSizeLimit(deserializer serde.Deserializer) (TransactionError__InvalidLoadedAccountsDataSizeLimit, error) {
	var obj TransactionError__InvalidLoadedAccountsDataSizeLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__ResanitizationNeeded struct{}

func (*TransactionError__ResanitizationNeeded) isTransactionError() {}

func (obj *TransactionError__ResanitizationNeeded) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(34)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__ResanitizationNeeded) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__ResanitizationNeeded(deserializer serde.Deserializer) (TransactionError__ResanitizationNeeded, error) {
	var obj TransactionError__ResanitizationNeeded
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__ProgramExecutionTemporarilyRestricted struct {
	AccountIndex uint8
}

func (*TransactionError__ProgramExecutionTemporarilyRestricted) isTransactionError() {}

func (obj *TransactionError__ProgramExecutionTemporarilyRestricted) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(35)
	if err := serializer.SerializeU8(obj.AccountIndex); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__ProgramExecutionTemporarilyRestricted) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__ProgramExecutionTemporarilyRestricted(deserializer serde.Deserializer) (TransactionError__ProgramExecutionTemporarilyRestricted, error) {
	var obj TransactionError__ProgramExecutionTemporarilyRestricted
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.AccountIndex = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__UnbalancedTransaction struct{}

func (*TransactionError__UnbalancedTransaction) isTransactionError() {}

func (obj *TransactionError__UnbalancedTransaction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(36)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__UnbalancedTransaction) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__UnbalancedTransaction(deserializer serde.Deserializer) (TransactionError__UnbalancedTransaction, error) {
	var obj TransactionError__UnbalancedTransaction
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__ProgramCacheHitMaxLimit struct{}

func (*TransactionError__ProgramCacheHitMaxLimit) isTransactionError() {}

func (obj *TransactionError__ProgramCacheHitMaxLimit) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(37)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__ProgramCacheHitMaxLimit) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__ProgramCacheHitMaxLimit(deserializer serde.Deserializer) (TransactionError__ProgramCacheHitMaxLimit, error) {
	var obj TransactionError__ProgramCacheHitMaxLimit
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionError__CommitCancelled struct{}

func (*TransactionError__CommitCancelled) isTransactionError() {}

func (obj *TransactionError__CommitCancelled) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(38)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__CommitCancelled) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__CommitCancelled(deserializer serde.Deserializer) (TransactionError__CommitCancelled, error) {
	var obj TransactionError__CommitCancelled
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

type TransactionReturnData struct {
	ProgramId Pubkey
	Data      []uint8
}

func (obj *TransactionReturnData) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := obj.ProgramId.Serialize(serializer); err != nil {
		return err
	}
	if err := serialize_vector_u8(obj.Data, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionReturnData) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeTransactionReturnData(deserializer serde.Deserializer) (TransactionReturnData, error) {
	var obj TransactionReturnData
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := DeserializePubkey(deserializer); err == nil {
		obj.ProgramId = val
	} else {
		return obj, err
	}
	if val, err := deserialize_vector_u8(deserializer); err == nil {
		obj.Data = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeTransactionReturnData(input []byte) (TransactionReturnData, error) {
	if input == nil {
		var obj TransactionReturnData
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeTransactionReturnData(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type TransactionStatusMeta struct {
	Status               Result
	Fee                  uint64
	PreBalances          []uint64
	PostBalances         []uint64
	InnerInstructions    *[]InnerInstructions
	LogMessages          *[]string
	PreTokenBalances     *[]TransactionTokenBalance
	PostTokenBalances    *[]TransactionTokenBalance
	Rewards              *[]Reward
	LoadedAddresses      LoadedAddresses
	ReturnData           *TransactionReturnData
	ComputeUnitsConsumed *uint64
}

func (obj *TransactionStatusMeta) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := obj.Status.Serialize(serializer); err != nil {
		return err
	}
	if err := serializer.SerializeU64(obj.Fee); err != nil {
		return err
	}
	if err := serialize_vector_u64(obj.PreBalances, serializer); err != nil {
		return err
	}
	if err := serialize_vector_u64(obj.PostBalances, serializer); err != nil {
		return err
	}
	if err := serialize_option_vector_InnerInstructions(obj.InnerInstructions, serializer); err != nil {
		return err
	}
	if err := serialize_option_vector_str(obj.LogMessages, serializer); err != nil {
		return err
	}
	if err := serialize_option_vector_TransactionTokenBalance(obj.PreTokenBalances, serializer); err != nil {
		return err
	}
	if err := serialize_option_vector_TransactionTokenBalance(obj.PostTokenBalances, serializer); err != nil {
		return err
	}
	if err := serialize_option_vector_Reward(obj.Rewards, serializer); err != nil {
		return err
	}
	if err := obj.LoadedAddresses.Serialize(serializer); err != nil {
		return err
	}
	if err := serialize_option_TransactionReturnData(obj.ReturnData, serializer); err != nil {
		return err
	}
	if err := serialize_option_u64(obj.ComputeUnitsConsumed, serializer); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionStatusMeta) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeTransactionStatusMeta(deserializer serde.Deserializer) (TransactionStatusMeta, error) {
	var obj TransactionStatusMeta
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, fmt.Errorf("failed to increase container depth at TransactionStatusMeta: %w", err)
	}
	if val, err := DeserializeResult(deserializer); err == nil {
		obj.Status = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Status: %w", err)
	}
	if val, err := deserializer.DeserializeU64(); err == nil {
		obj.Fee = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize Fee: %w", err)
	}
	if val, err := deserialize_vector_u64(deserializer); err == nil {
		obj.PreBalances = val
	} else {
		return obj, fmt.Errorf("Failed to deserialize PreBalances: %w", err)
	}
	if val, err := deserialize_vector_u64(deserializer); err == nil {
		obj.PostBalances = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize PostBalances: %w", err)
	}
	if val, err := deserialize_option_vector_InnerInstructions(deserializer); err == nil {
		obj.InnerInstructions = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize InnerInstructions: %w", err)
	}
	if val, err := deserialize_option_vector_str(deserializer); err == nil {
		obj.LogMessages = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize LogMessages: %w", err)
	}
	if val, err := deserialize_option_vector_TransactionTokenBalance(deserializer); err == nil {
		obj.PreTokenBalances = val
	} else {
		if errors.Is(err, io.EOF) || strings.Contains(err.Error(), "length is too large") {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize PreTokenBalances: %w", err)
	}
	if val, err := deserialize_option_vector_TransactionTokenBalance(deserializer); err == nil {
		obj.PostTokenBalances = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize PostTokenBalances: %w", err)
	}
	if val, err := deserialize_option_vector_Reward(deserializer); err == nil {
		obj.Rewards = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize Rewards: %w", err)
	}
	if val, err := DeserializeLoadedAddresses(deserializer); err == nil {
		obj.LoadedAddresses = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize LoadedAddresses: %w", err)
	}
	if val, err := deserialize_option_TransactionReturnData(deserializer); err == nil {
		obj.ReturnData = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize ReturnData: %w", err)
	}
	if val, err := deserialize_option_u64(deserializer); err == nil {
		obj.ComputeUnitsConsumed = val
	} else {
		if errors.Is(err, io.EOF) {
			deserializer.DecreaseContainerDepth()
			return obj, nil
		}
		return obj, fmt.Errorf("Failed to deserialize ComputeUnitsConsumed: %w", err)
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeTransactionStatusMeta(input []byte) (TransactionStatusMeta, error) {
	if input == nil {
		var obj TransactionStatusMeta
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeTransactionStatusMeta(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type TransactionTokenBalance struct {
	AccountIndex  uint8
	Mint          string
	UiTokenAmount UiTokenAmount
	Owner         string
	ProgramId     string
}

func (obj *TransactionTokenBalance) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.AccountIndex); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.Mint); err != nil {
		return err
	}
	if err := obj.UiTokenAmount.Serialize(serializer); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.Owner); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.ProgramId); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionTokenBalance) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeTransactionTokenBalance(deserializer serde.Deserializer) (TransactionTokenBalance, error) {
	var obj TransactionTokenBalance
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.AccountIndex = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.Mint = val
	} else {
		return obj, err
	}
	if val, err := DeserializeUiTokenAmount(deserializer); err == nil {
		obj.UiTokenAmount = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.Owner = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.ProgramId = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeTransactionTokenBalance(input []byte) (TransactionTokenBalance, error) {
	if input == nil {
		var obj TransactionTokenBalance
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeTransactionTokenBalance(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

type UiTokenAmount struct {
	UiAmount       *float64
	Decimals       uint8
	Amount         string
	UiAmountString string
}

func (obj *UiTokenAmount) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serialize_option_f64(obj.UiAmount, serializer); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.Decimals); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.Amount); err != nil {
		return err
	}
	if err := serializer.SerializeStr(obj.UiAmountString); err != nil {
		return err
	}
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *UiTokenAmount) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func DeserializeUiTokenAmount(deserializer serde.Deserializer) (UiTokenAmount, error) {
	var obj UiTokenAmount
	if err := deserializer.IncreaseContainerDepth(); err != nil {
		return obj, err
	}
	if val, err := deserialize_option_f64(deserializer); err == nil {
		obj.UiAmount = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Decimals = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.Amount = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeStr(); err == nil {
		obj.UiAmountString = val
	} else {
		return obj, err
	}
	deserializer.DecreaseContainerDepth()
	return obj, nil
}

func BincodeDeserializeUiTokenAmount(input []byte) (UiTokenAmount, error) {
	if input == nil {
		var obj UiTokenAmount
		return obj, fmt.Errorf("Cannot deserialize null array")
	}
	deserializer := bincode.NewDeserializer(input)
	obj, err := DeserializeUiTokenAmount(deserializer)
	if err == nil && deserializer.GetBufferOffset() < uint64(len(input)) {
		return obj, ErrSomeBytesNotRead
	}
	return obj, err
}

func serialize_array32_u8_array(value [32]uint8, serializer serde.Serializer) error {
	for _, item := range value {
		if err := serializer.SerializeU8(item); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_array32_u8_array(deserializer serde.Deserializer) ([32]uint8, error) {
	var obj [32]uint8
	for i := range obj {
		if val, err := deserializer.DeserializeU8(); err == nil {
			obj[i] = val
		} else {
			return obj, err
		}
	}
	return obj, nil
}

func serialize_option_RewardType(value *RewardType, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := (*value).Serialize(serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_RewardType(deserializer serde.Deserializer) (*RewardType, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(RewardType)
		if val, err := DeserializeRewardType(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_TransactionReturnData(value *TransactionReturnData, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := (*value).Serialize(serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_TransactionReturnData(deserializer serde.Deserializer) (*TransactionReturnData, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(TransactionReturnData)
		if val, err := DeserializeTransactionReturnData(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_f64(value *float64, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serializer.SerializeF64((*value)); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_f64(deserializer serde.Deserializer) (*float64, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(float64)
		if val, err := deserializer.DeserializeF64(); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_u32(value *uint32, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serializer.SerializeU32((*value)); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_u32(deserializer serde.Deserializer) (*uint32, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(uint32)
		if val, err := deserializer.DeserializeU32(); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_u64(value *uint64, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serializer.SerializeU64((*value)); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_u64(deserializer serde.Deserializer) (*uint64, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(uint64)
		if val, err := deserializer.DeserializeU64(); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_u8(value *uint8, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serializer.SerializeU8((*value)); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_u8(deserializer serde.Deserializer) (*uint8, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new(uint8)
		if val, err := deserializer.DeserializeU8(); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_vector_InnerInstructions(value *[]InnerInstructions, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serialize_vector_InnerInstructions((*value), serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_vector_InnerInstructions(deserializer serde.Deserializer) (*[]InnerInstructions, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new([]InnerInstructions)
		if val, err := deserialize_vector_InnerInstructions(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_vector_Reward(value *[]Reward, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serialize_vector_Reward((*value), serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_vector_Reward(deserializer serde.Deserializer) (*[]Reward, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new([]Reward)
		if val, err := deserialize_vector_Reward(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_vector_TransactionTokenBalance(value *[]TransactionTokenBalance, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serialize_vector_TransactionTokenBalance((*value), serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_vector_TransactionTokenBalance(deserializer serde.Deserializer) (*[]TransactionTokenBalance, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new([]TransactionTokenBalance)
		if val, err := deserialize_vector_TransactionTokenBalance(deserializer); err == nil {
			*value = val
		} else {
			return nil, fmt.Errorf("failed to deserialize TransactionTokenBalance: %w", err)
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_option_vector_str(value *[]string, serializer serde.Serializer) error {
	if value != nil {
		if err := serializer.SerializeOptionTag(true); err != nil {
			return err
		}
		if err := serialize_vector_str((*value), serializer); err != nil {
			return err
		}
	} else {
		if err := serializer.SerializeOptionTag(false); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_option_vector_str(deserializer serde.Deserializer) (*[]string, error) {
	tag, err := deserializer.DeserializeOptionTag()
	if err != nil {
		return nil, err
	}
	if tag {
		value := new([]string)
		if val, err := deserialize_vector_str(deserializer); err == nil {
			*value = val
		} else {
			return nil, err
		}
		return value, nil
	} else {
		return nil, nil
	}
}

func serialize_vector_InnerInstruction(value []InnerInstruction, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := item.Serialize(serializer); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_InnerInstruction(deserializer serde.Deserializer) ([]InnerInstruction, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]InnerInstruction, length)
	for i := range obj {
		if val, err := DeserializeInnerInstruction(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, fmt.Errorf("Failed to deserialize InnerInstruction[%d]: %w", i, err)
		}
	}
	return obj, nil
}

func serialize_vector_InnerInstructions(value []InnerInstructions, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := item.Serialize(serializer); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_InnerInstructions(deserializer serde.Deserializer) ([]InnerInstructions, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]InnerInstructions, length)
	for i := range obj {
		if val, err := DeserializeInnerInstructions(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, fmt.Errorf("Failed to deserialize InnerInstructions[%d]: %w", i, err)
		}
	}
	return obj, nil
}

func serialize_vector_Pubkey(value []Pubkey, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := item.Serialize(serializer); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_Pubkey(deserializer serde.Deserializer) ([]Pubkey, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]Pubkey, length)
	for i := range obj {
		if val, err := DeserializePubkey(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func serialize_vector_Reward(value []Reward, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := item.Serialize(serializer); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_Reward(deserializer serde.Deserializer) ([]Reward, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]Reward, length)
	for i := range obj {
		if val, err := DeserializeReward(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func serialize_vector_TransactionTokenBalance(value []TransactionTokenBalance, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := item.Serialize(serializer); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_TransactionTokenBalance(deserializer serde.Deserializer) ([]TransactionTokenBalance, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, fmt.Errorf("failed to deserialize length: %w", err)
	}
	obj := make([]TransactionTokenBalance, length)
	for i := range obj {
		if val, err := DeserializeTransactionTokenBalance(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, fmt.Errorf("Failed to deserialize TransactionTokenBalance[%d]: %w", i, err)
		}
	}
	return obj, nil
}

func serialize_vector_str(value []string, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := serializer.SerializeStr(item); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_str(deserializer serde.Deserializer) ([]string, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]string, length)
	for i := range obj {
		if val, err := deserializer.DeserializeStr(); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func serialize_vector_u64(value []uint64, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := serializer.SerializeU64(item); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_u64(deserializer serde.Deserializer) ([]uint64, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]uint64, length)
	for i := range obj {
		if val, err := deserializer.DeserializeU64(); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func serialize_vector_u8(value []uint8, serializer serde.Serializer) error {
	if err := serializer.SerializeLen(uint64(len(value))); err != nil {
		return err
	}
	for _, item := range value {
		if err := serializer.SerializeU8(item); err != nil {
			return err
		}
	}
	return nil
}

func deserialize_vector_u8(deserializer serde.Deserializer) ([]uint8, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, fmt.Errorf("Failed to deserialize length: %w", err)
	}
	obj := make([]uint8, length)
	for i := range obj {
		if val, err := deserializer.DeserializeU8(); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}

func deserialize_accounts_vector_u8(deserializer serde.Deserializer) ([]uint8, error) {
	length, err := deserializer.DeserializeU8()
	if err != nil {
		return nil, fmt.Errorf("Failed to deserialize length: %w", err)
	}
	obj := make([]uint8, length)
	for i := range obj {
		if val, err := deserializer.DeserializeU8(); err == nil {
			obj[i] = val
		} else {
			return nil, err
		}
	}
	return obj, nil
}
