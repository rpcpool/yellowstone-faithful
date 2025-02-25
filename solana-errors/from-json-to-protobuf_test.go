package solanaerrors

import (
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestFromJSONToProtobuf(t *testing.T) {
	{
		candidate := map[string]any{
			"InstructionError": []any{
				2.0,
				map[string]any{
					"Custom": 6001.0,
				},
			},
		}
		buf, err := FromJSONToProtobuf(
			candidate,
		)
		require.NoError(t, err)
		require.NotNil(t, buf)
		require.Equal(t,
			[]byte{
				0x8, 0x0, 0x0, 0x0, // instruction error
				0x2,                 // error code
				0x19, 0x0, 0x0, 0x0, // instruction error type
				0x71, 0x17, 0x0, 0x0, // 6001
			},
			buf,
		)
		require.Equal(t,
			concat(
				uint32tobytes(uint32(TransactionErrorType_INSTRUCTION_ERROR)),
				[]byte{0x2},
				uint32tobytes(uint32(InstructionErrorType_CUSTOM)),
				uint32tobytes(6001),
			),
			buf,
		)
		{
			candidateAsBase64 := base64.StdEncoding.EncodeToString(buf)
			wrapped := map[string]any{
				"err": candidateAsBase64,
			}
			got, err := ParseTransactionError(wrapped)
			require.NoError(t, err)
			require.NotNil(t, got)

			require.JSONEq(t,
				toJson(t, candidate),
				toJson(t, got),
			)
		}
	}
	{
		candidate := map[string]any{
			"InstructionError": []any{
				0.0,
				map[string]any{
					"BorshIoError": "Unknown",
				},
			},
		}
		buf, err := FromJSONToProtobuf(
			candidate,
		)
		require.NoError(t, err)
		require.NotNil(t, buf)
		require.Equal(t,
			[]byte{
				0x8, 0x0, 0x0, 0x0,
				0x0,
				0x2c, 0x0, 0x0, 0x0,
				0x7, 0x55, 0x6e, 0x6b, 0x6e, 0x6f, 0x77, 0x6e, // "Unknown"
			},
			buf,
		)
		require.Equal(t,
			concat(
				uint32tobytes(uint32(TransactionErrorType_INSTRUCTION_ERROR)),
				[]byte{0x0},
				uint32tobytes(uint32(InstructionErrorType_BORSH_IO_ERROR)),
				// length of "Unknown"
				[]byte{0x7},
				[]byte("Unknown"),
			),
			buf,
		)
		{
			candidateAsBase64 := base64.StdEncoding.EncodeToString(buf)
			wrapped := map[string]any{
				"err": candidateAsBase64,
			}
			got, err := ParseTransactionError(wrapped)
			require.NoError(t, err)
			require.NotNil(t, got)

			require.JSONEq(t,
				toJson(t, candidate),
				toJson(t, got),
			)
		}
	}
}

func uint32tobytes(v uint32) []byte {
	return binary.LittleEndian.AppendUint32(nil, v)
}

func concat(bs ...[]byte) []byte {
	b := make([]byte, 0)
	for _, v := range bs {
		b = append(b, v...)
	}
	return b
}

func toJson(t *testing.T, v interface{}) string {
	b, err := json.Marshal(v)
	require.NoError(t, err)
	return string(b)
}
