package apps

import (
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math"
	"testing"

	"github.com/stretchr/testify/require"

	"github.com/algorand/avm-abi/abi"
	"github.com/algorand/avm-abi/address"
)

func TestNewAppCallBytes(t *testing.T) {
	t.Parallel()

	t.Run("errors", func(t *testing.T) {
		t.Parallel()
		_, err := NewAppCallBytes("hello")
		require.Error(t, err)

		for _, v := range []string{":x", "int:-1"} {
			acb, _ := NewAppCallBytes(v)
			_, err = acb.Raw()
			require.Error(t, err)
		}
	})

	for _, v := range []string{"hello", "1:2"} {
		for _, e := range []string{"str", "string"} {
			v, e := v, e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, v), func(t *testing.T) {
				t.Parallel()
				acb, err := NewAppCallBytes(fmt.Sprintf("%v:%v", e, v))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				require.Equal(t, v, string(r))
			})
		}

		for _, e := range []string{"b32", "base32", "byte base32"} {
			ve := base32.StdEncoding.EncodeToString([]byte(v))
			e := e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, ve), func(t *testing.T) {
				acb, err := NewAppCallBytes(fmt.Sprintf("%v:%v", e, ve))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				require.Equal(t, ve, base32.StdEncoding.EncodeToString(r))
			})
		}

		for _, e := range []string{"b64", "base64", "byte base64"} {
			ve := base64.StdEncoding.EncodeToString([]byte(v))
			e := e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, ve), func(t *testing.T) {
				t.Parallel()
				acb, err := NewAppCallBytes(fmt.Sprintf("%v:%v", e, ve))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				require.Equal(t, ve, base64.StdEncoding.EncodeToString(r))
			})
		}
	}

	for _, v := range []uint64{1, 0, math.MaxUint64} {
		for _, e := range []string{"int", "integer"} {
			v, e := v, e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, v), func(t *testing.T) {
				t.Parallel()
				acb, err := NewAppCallBytes(fmt.Sprintf("%v:%v", e, v))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				require.Equal(t, v, binary.BigEndian.Uint64(r))
			})
		}
	}

	for _, v := range []string{"737777777777777777777777777777777777777777777777777UFEJ2CI"} {
		for _, e := range []string{"addr", "address"} {
			v, e := v, e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, v), func(t *testing.T) {
				t.Parallel()
				acb, err := NewAppCallBytes(fmt.Sprintf("%v:%v", e, v))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				addr, err := address.FromString(v)
				require.NoError(t, err)
				expectedBytes := addr[:]
				require.Equal(t, expectedBytes, r)
			})
		}
	}

	type abiCase struct {
		abiType, rawValue string
	}
	for _, v := range []abiCase{
		{
			`(uint64,string,bool[])`,
			`[399,"should pass",[true,false,false,true]]`,
		}} {
		for _, e := range []string{"abi"} {
			v, e := v, e
			t.Run(fmt.Sprintf("encoding=%v,value=%v", e, v), func(t *testing.T) {
				t.Parallel()
				acb, err := NewAppCallBytes(fmt.Sprintf(
					"%v:%v:%v", e, v.abiType, v.rawValue))
				require.NoError(t, err)
				r, err := acb.Raw()
				require.NoError(t, err)
				require.NotEmpty(t, r)

				// Confirm round-trip works.
				abiType, err := abi.TypeOf(v.abiType)
				require.NoError(t, err)
				d, err := abiType.Decode(r)
				require.NoError(t, err)
				vv, err := abiType.Encode(d)
				require.NoError(t, err)
				require.Equal(t, r, vv)
			})
		}
	}
}
