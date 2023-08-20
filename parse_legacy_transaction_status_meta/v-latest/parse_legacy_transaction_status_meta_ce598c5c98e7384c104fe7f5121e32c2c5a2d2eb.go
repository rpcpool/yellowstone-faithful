package parse_legacy_transaction_status_meta_ce598c5c98e7384c104fe7f5121e32c2c5a2d2eb

import (
	"fmt"

	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/bincode"
	"github.com/novifinancial/serde-reflection/serde-generate/runtime/golang/serde"
	"k8s.io/klog"
)

type CompiledInstruction struct {
	ProgramIdIndex uint8
	Accounts       struct {
		Field0 struct{ Field0 uint8 }
		Field1 uint8
		Field2 uint8
		Field3 uint8
	}
	Data struct {
		Field0 struct{ Field0 uint8 }
		Field1 uint8
		Field2 uint8
		Field3 uint8
	}
}

func (obj *CompiledInstruction) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.ProgramIdIndex); err != nil {
		return err
	}
	if err := serialize_tuple4_tuple1_u8_u8_u8_u8(obj.Accounts, serializer); err != nil {
		return err
	}
	if err := serialize_tuple4_tuple1_u8_u8_u8_u8(obj.Data, serializer); err != nil {
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
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.ProgramIdIndex = val
	} else {
		return obj, err
	}
	if val, err := deserialize_tuple4_tuple1_u8_u8_u8_u8(deserializer); err == nil {
		obj.Accounts = val
	} else {
		return obj, err
	}
	if val, err := deserialize_tuple4_tuple1_u8_u8_u8_u8(deserializer); err == nil {
		obj.Data = val
	} else {
		return obj, err
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
		return obj, fmt.Errorf("Some input bytes were not read")
	}
	return obj, err
}

type InnerInstructions struct {
	Index        uint8
	Instructions []CompiledInstruction
}

func (obj *InnerInstructions) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	if err := serializer.SerializeU8(obj.Index); err != nil {
		return err
	}
	if err := serialize_vector_CompiledInstruction(obj.Instructions, serializer); err != nil {
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
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Index = val
	} else {
		return obj, err
	}
	if val, err := deserialize_vector_CompiledInstruction(deserializer); err == nil {
		obj.Instructions = val
	} else {
		return obj, err
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
		return obj, fmt.Errorf("Some input bytes were not read")
	}
	return obj, err
}

type InstructionError interface {
	isInstructionError()
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
		return obj, fmt.Errorf("Some input bytes were not read")
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
		return obj, fmt.Errorf("Some input bytes were not read")
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

type TransactionError interface {
	isTransactionError()
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
		if val, err := load_TransactionError__DuplicateSignature(deserializer); err == nil {
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
		return obj, fmt.Errorf("Some input bytes were not read")
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

type TransactionError__DuplicateSignature struct{}

func (*TransactionError__DuplicateSignature) isTransactionError() {}

func (obj *TransactionError__DuplicateSignature) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(6)
	serializer.DecreaseContainerDepth()
	return nil
}

func (obj *TransactionError__DuplicateSignature) BincodeSerialize() ([]byte, error) {
	if obj == nil {
		return nil, fmt.Errorf("Cannot serialize null object")
	}
	serializer := bincode.NewSerializer()
	if err := obj.Serialize(serializer); err != nil {
		return nil, err
	}
	return serializer.GetBytes(), nil
}

func load_TransactionError__DuplicateSignature(deserializer serde.Deserializer) (TransactionError__DuplicateSignature, error) {
	var obj TransactionError__DuplicateSignature
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
	Field0 uint8
	Field1 InstructionError
}

func (*TransactionError__InstructionError) isTransactionError() {}

func (obj *TransactionError__InstructionError) Serialize(serializer serde.Serializer) error {
	if err := serializer.IncreaseContainerDepth(); err != nil {
		return err
	}
	serializer.SerializeVariantIndex(8)
	if err := serializer.SerializeU8(obj.Field0); err != nil {
		return err
	}
	if err := obj.Field1.Serialize(serializer); err != nil {
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
		obj.Field0 = val
	} else {
		return obj, err
	}
	if val, err := DeserializeInstructionError(deserializer); err == nil {
		obj.Field1 = val
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

type TransactionStatusMeta struct {
	Status            Result
	Fee               uint64
	PreBalances       []uint64
	PostBalances      []uint64
	InnerInstructions *[]InnerInstructions
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
		return obj, err
	}
	if val, err := DeserializeResult(deserializer); err == nil {
		obj.Status = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU64(); err == nil {
		obj.Fee = val
	} else {
		return obj, err
	}
	if val, err := deserialize_vector_u64(deserializer); err == nil {
		obj.PreBalances = val
	} else {
		return obj, err
	}
	if val, err := deserialize_vector_u64(deserializer); err == nil {
		obj.PostBalances = val
	} else {
		return obj, err
	}
	if val, err := deserialize_option_vector_InnerInstructions(deserializer); err == nil {
		obj.InnerInstructions = val
	} else {
		return obj, err
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
		// return obj, fmt.Errorf("Some input bytes were not read")
		// TODO: fix this
		klog.Warningf(
			"Parsed %d bytes, but input was %d bytes (%d bytes not read)",
			deserializer.GetBufferOffset(),
			len(input),
			len(input)-int(deserializer.GetBufferOffset()),
		)
	}
	return obj, err
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

func serialize_tuple1_u8(value struct{ Field0 uint8 }, serializer serde.Serializer) error {
	if err := serializer.SerializeU8(value.Field0); err != nil {
		return err
	}
	return nil
}

func deserialize_tuple1_u8(deserializer serde.Deserializer) (struct{ Field0 uint8 }, error) {
	var obj struct{ Field0 uint8 }
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Field0 = val
	} else {
		return obj, err
	}
	return obj, nil
}

func serialize_tuple4_tuple1_u8_u8_u8_u8(value struct {
	Field0 struct{ Field0 uint8 }
	Field1 uint8
	Field2 uint8
	Field3 uint8
}, serializer serde.Serializer,
) error {
	if err := serialize_tuple1_u8(value.Field0, serializer); err != nil {
		return err
	}
	if err := serializer.SerializeU8(value.Field1); err != nil {
		return err
	}
	if err := serializer.SerializeU8(value.Field2); err != nil {
		return err
	}
	if err := serializer.SerializeU8(value.Field3); err != nil {
		return err
	}
	return nil
}

func deserialize_tuple4_tuple1_u8_u8_u8_u8(deserializer serde.Deserializer) (struct {
	Field0 struct{ Field0 uint8 }
	Field1 uint8
	Field2 uint8
	Field3 uint8
}, error,
) {
	var obj struct {
		Field0 struct{ Field0 uint8 }
		Field1 uint8
		Field2 uint8
		Field3 uint8
	}
	if val, err := deserialize_tuple1_u8(deserializer); err == nil {
		obj.Field0 = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Field1 = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Field2 = val
	} else {
		return obj, err
	}
	if val, err := deserializer.DeserializeU8(); err == nil {
		obj.Field3 = val
	} else {
		return obj, err
	}
	return obj, nil
}

func serialize_vector_CompiledInstruction(value []CompiledInstruction, serializer serde.Serializer) error {
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

func deserialize_vector_CompiledInstruction(deserializer serde.Deserializer) ([]CompiledInstruction, error) {
	length, err := deserializer.DeserializeLen()
	if err != nil {
		return nil, err
	}
	obj := make([]CompiledInstruction, length)
	for i := range obj {
		if val, err := DeserializeCompiledInstruction(deserializer); err == nil {
			obj[i] = val
		} else {
			return nil, err
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
