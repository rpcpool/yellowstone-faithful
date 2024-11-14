package solanaerrors

import (
	"bytes"
	"encoding/json"
	"fmt"

	bin "github.com/gagliardetto/binary"
)

func FromJSONToProtobuf(j map[string]interface{}) ([]byte, error) {
	// get first key
	firstKey := getFirstKey(j)
	if firstKey == "" {
		return nil, fmt.Errorf("no keys found in map")
	}
	buf := new(bytes.Buffer)
	wr := bin.NewBinEncoder(buf)
	doer := &ChainOps{}
	switch firstKey {
	case InstructionError:
		{
			doer.Do("write transactionErrorType", func() error {
				return wr.WriteUint32(uint32(TransactionErrorType_INSTRUCTION_ERROR), bin.LE)
			})

			{
				// read instructionErrorType
				arr, ok := j[InstructionError].([]interface{})
				if !ok {
					return nil, fmt.Errorf("expected an array")
				}
				if len(arr) != 2 {
					return nil, fmt.Errorf("expected an array of length 2")
				}
				instructionErrorCodeFloat, ok := arr[0].(float64)
				if !ok {
					return nil, fmt.Errorf("expected a float64, got %T", arr[0])
				}

				instructionErrorCode := uint8(instructionErrorCodeFloat)
				doer.Do("write errorCode", func() error {
					return wr.WriteUint8(instructionErrorCode)
				})

				{
					switch as := arr[1].(type) {
					case string:
						{
							var found bool
							// if string, then map instructionErrorTypeName to code
						ixLoop:
							for k, v := range InstructionErrorType_name {
								// TODO: the conversion to PascalCase might be wrong and break things.
								if bin.ToPascalCase(v) == as {
									doer.Do("write instructionErrorType", func() error {
										return wr.WriteUint32(uint32(k), bin.LE)
									})
									found = true
									break ixLoop
								}
							}
							if !found {
								return nil, fmt.Errorf("unknown error type: %q", as)
							}
						}
					case map[string]interface{}:
						{
							// if object, then it's custom
							firstKey := getFirstKey(as)
							if firstKey == "" {
								return nil, fmt.Errorf("no keys found in map")
							}
							if firstKey != "Custom" {
								return nil, fmt.Errorf("expected a Custom key")
							}
							doer.Do("write customErrorType", func() error {
								return wr.WriteUint32(uint32(InstructionErrorType_CUSTOM), bin.LE)
							})
							customErrorTypeFloat, ok := as[firstKey].(float64)
							if !ok {
								return nil, fmt.Errorf("expected a float64")
							}
							customErrorType := uint32(customErrorTypeFloat)
							doer.Do("write customErrorType", func() error {
								return wr.WriteUint32(customErrorType, bin.LE)
							})
						}
					default:
						return nil, fmt.Errorf("unhandled type %T", arr[1])
					}
				}

			}

			err := doer.Err()
			if err != nil {
				return nil, err
			}

			return buf.Bytes(), nil
		}
	case InsufficientFundsForRent:
		{
			doer.Do("write transactionErrorType", func() error {
				return wr.WriteUint32(uint32(TransactionErrorType_INSUFFICIENT_FUNDS_FOR_RENT), bin.LE)
			})
			// write the accountIndex
			{
				// "{\"InsufficientFundsForRent\":{\"account_index\":2}}"
				// read accountIndex
				object, ok := j[InsufficientFundsForRent].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("expected an object")
				}
				accountIndexFloat, ok := object["account_index"].(float64)
				if !ok {
					return nil, fmt.Errorf("expected a float64")
				}
				accountIndex := uint8(accountIndexFloat)
				doer.Do("write accountIndex", func() error {
					return wr.WriteUint8(accountIndex)
				})

				if err := doer.Err(); err != nil {
					return nil, err
				}
			}
			return buf.Bytes(), nil
		}
	case ProgramExecutionTemporarilyRestricted:
		{
			doer.Do("write transactionErrorType", func() error {
				return wr.WriteUint32(uint32(TransactionErrorType_PROGRAM_EXECUTION_TEMPORARILY_RESTRICTED), bin.LE)
			})
			// write the accountIndex
			{
				// "{\"ProgramExecutionTemporarilyRestricted\":{\"account_index\":2}}"
				// read accountIndex
				object, ok := j[ProgramExecutionTemporarilyRestricted].(map[string]any)
				if !ok {
					return nil, fmt.Errorf("expected an object")
				}
				accountIndexFloat, ok := object["account_index"].(float64)
				if !ok {
					return nil, fmt.Errorf("expected a float64")
				}
				accountIndex := uint8(accountIndexFloat)
				doer.Do("write accountIndex", func() error {
					return wr.WriteUint8(accountIndex)
				})

				if err := doer.Err(); err != nil {
					return nil, err
				}
			}
			return buf.Bytes(), nil
		}
	case DuplicateInstruction:
		{
			doer.Do("write transactionErrorType", func() error {
				return wr.WriteUint32(uint32(TransactionErrorType_DUPLICATE_INSTRUCTION), bin.LE)
			})
			// write the instruction index
			{
				// "{\"DuplicateInstruction\":[2]}"
				// read instructionIndex
				arr, ok := j[DuplicateInstruction].([]interface{})
				if !ok {
					return nil, fmt.Errorf("expected an array")
				}
				if len(arr) != 1 {
					return nil, fmt.Errorf("expected an array of length 1")
				}
				instructionIndexFloat, ok := arr[0].(float64)
				if !ok {
					return nil, fmt.Errorf("expected a float64")
				}
				instructionIndex := uint8(instructionIndexFloat)
				doer.Do("write instructionIndex", func() error {
					return wr.WriteUint8(instructionIndex)
				})

				if err := doer.Err(); err != nil {
					return nil, err
				}
			}
			return buf.Bytes(), nil
		}

	default:
		// it's one of the single-value errors
		{
			// iterate over TransactionErrorType_name and find the matching key
			var found bool
			for k, v := range TransactionErrorType_name {
				if bin.ToPascalCase(v) == firstKey {
					doer.Do("write transactionErrorType", func() error {
						return wr.WriteUint32(uint32(k), bin.LE)
					})
					found = true
					break
				}
			}
			if !found {
				return nil, fmt.Errorf("unknown error type: %q", firstKey)
			}
		}

		err := doer.Err()
		if err != nil {
			return nil, err
		}

		return buf.Bytes(), nil
	}
}

func toJsonString(v interface{}) string {
	j, err := json.Marshal(v)
	if err != nil {
		return fmt.Sprintf("%v", v)
	}
	return string(j)
}

func getFirstKey(m map[string]interface{}) string {
	for k := range m {
		return k
	}
	return ""
}

type ChainOps struct {
	e error
}

func (c *ChainOps) Do(name string, f func() error) *ChainOps {
	if c.e != nil {
		return c
	}
	c.e = f()
	return c
}

func (c *ChainOps) Err() error {
	return c.e
}
