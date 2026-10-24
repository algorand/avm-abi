package abi

import (
	"fmt"
	"math"
	"math/rand"
	"runtime"
	"strconv"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func mustTypeOf(t *testing.T, s string) Type {
	t.Helper()
	typ, err := TypeOf(s)
	require.NoError(t, err, s)
	return typ
}

// TestByteLenOverflow checks that ByteLen reports an error instead of silently wrapping when the
// byte length of a static type does not fit in an int.
func TestByteLenOverflow(t *testing.T) {
	if strconv.IntSize != 64 {
		t.Skip("test sizes assume a 64-bit int")
	}

	// 65535^5 * 8 wraps around int64.
	for _, s := range []string{
		"uint64[65535][65535][65535][65535][65535]",
		"uint8[65535][65535][65535][65535]",
		// each element fits in an int (~5.6e18) but the sum of two does not.
		"(uint8[20000][65535][65535][65535],uint8[20000][65535][65535][65535])",
		"(uint8[20000][65535][65535][65535],uint8[20000][65535][65535][65535],uint8)",
	} {
		n, err := mustTypeOf(t, s).ByteLen()
		require.Error(t, err, "%s: got ByteLen %d", s, n)
		require.Equal(t, -1, n, s)
	}

	// large but representable sizes keep working.
	n, err := mustTypeOf(t, "uint64[65535][65535][65535]").ByteLen()
	require.NoError(t, err)
	require.EqualValues(t, int64(8)*65535*65535*65535, n)
	n, err = mustTypeOf(t, "uint8[20000][65535][65535][65535]").ByteLen()
	require.NoError(t, err)
	require.EqualValues(t, int64(20000)*65535*65535*65535, n)
}

// TestByteLenBoolArrayNoUint16Wrap checks that the byte length of a static bool array does not
// wrap around when the array length is close to the maximum uint16.
func TestByteLenBoolArrayNoUint16Wrap(t *testing.T) {
	for _, tc := range []struct {
		length   int
		expected int
	}{
		{0, 0}, {1, 1}, {8, 1}, {9, 2}, {65528, 8191}, {65529, 8192}, {65534, 8192}, {65535, 8192},
	} {
		typ := mustTypeOf(t, fmt.Sprintf("bool[%d]", tc.length))
		n, err := typ.ByteLen()
		require.NoError(t, err)
		require.Equal(t, tc.expected, n, "bool[%d]", tc.length)
	}

	// a (bool[65535],uint8) tuple must be 8193 bytes: consumers rely on ByteLen to split it.
	n, err := mustTypeOf(t, "(bool[65535],uint8)").ByteLen()
	require.NoError(t, err)
	require.Equal(t, 8193, n)
}

// TestDecodeShortInputDoesNotPanic checks that malformed (too short) input is reported as an error.
func TestDecodeShortInputDoesNotPanic(t *testing.T) {
	cases := []struct {
		typ   string
		input []byte
	}{
		{"uint64[3]", []byte{1, 2}},
		{"uint64[3]", nil},
		{"(uint64,uint64)", []byte{1, 2, 3}},
		{"(uint8,uint64)", make([]byte, 5)},
		{"(bool,uint8)", nil},
		{"bool[3]", nil},
		{"(bool,bool)", []byte{}},
		{"address[2]", make([]byte, 40)},
		{"uint64[65534][65534]", []byte{1, 2}},
		{"uint64[65534][65534][65534]", []byte{1, 2}},
		{"uint64[65534][65534][65534][65534][65534]", []byte{1, 2}},
		{"(uint64[3],string)", make([]byte, 10)},
		{"uint64[][]", []byte{0, 2, 0, 1}},
		{"uint64[]", []byte{0, 5, 1, 2, 3}},
		{"bool[]", []byte{0xff, 0xff}},
		{"string[3]", []byte{0, 1}},
		{"string[]", []byte{0xff, 0xff}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(fmt.Sprintf("%s/%d", tc.typ, len(tc.input)), func(t *testing.T) {
			typ := mustTypeOf(t, tc.typ)
			require.NotPanics(t, func() {
				_, err := typ.Decode(tc.input)
				require.Error(t, err)
			})
		})
	}
}

// decodeAllocBytes returns the smallest number of bytes allocated by a call to Decode over several
// runs, which filters out unrelated background allocations.
func decodeAllocBytes(typ Type, input []byte) uint64 {
	best := uint64(math.MaxUint64)
	for i := 0; i < 5; i++ {
		var before, after runtime.MemStats
		runtime.GC()
		runtime.ReadMemStats(&before)
		_, _ = typ.Decode(input)
		runtime.ReadMemStats(&after)
		if used := after.TotalAlloc - before.TotalAlloc; used < best {
			best = used
		}
	}
	return best
}

// TestDecodeDoesNotExpandBeforeValidatingLength checks that the memory used to decode a value is
// bounded by the size of the input, not by the length claimed by the type or by the length prefix.
func TestDecodeDoesNotExpandBeforeValidatingLength(t *testing.T) {
	const budget = 256 * 1024 // an order of magnitude below the ~2.6 MB the unfixed code allocates per call

	cases := []struct {
		name  string
		typ   string
		input []byte
	}{
		{"static byte array", "uint8[65534]", []byte{1, 2}},
		{"static array of arrays", "uint8[65534][65534]", []byte{1, 2}},
		{"static array of static arrays", "uint64[65534][65534][65534]", []byte{1, 2}},
		{"static bool array", "bool[65534]", []byte{1, 2}},
		{"static array of dynamic", "string[65534]", []byte{0, 1}},
		{"dynamic array claiming max length", "uint8[]", []byte{0xff, 0xff}},
		{"dynamic bool array claiming max length", "bool[]", []byte{0xff, 0xff, 0x80}},
		{"dynamic array of dynamic claiming max length", "string[]", []byte{0xff, 0xff, 0, 0}},
		{"nested dynamic array", "uint64[][]", []byte{0xff, 0xff, 0, 0}},
	}
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			typ := mustTypeOf(t, tc.typ)
			// warm up so one-time allocations are not counted.
			_, _ = typ.Decode(tc.input)
			allocated := decodeAllocBytes(typ, tc.input)
			require.Less(t, allocated, uint64(budget), "decoding %d bytes as %s allocated %d bytes", len(tc.input), tc.typ, allocated)
		})
	}
}

// TestDecodeValidLargeArrays checks that decoding still works for large, valid inputs.
func TestDecodeValidLargeArrays(t *testing.T) {
	typ := mustTypeOf(t, "uint8[1000]")
	input := make([]byte, 1000)
	for i := range input {
		input[i] = byte(i)
	}
	decoded, err := typ.Decode(input)
	require.NoError(t, err)
	require.Len(t, decoded, 1000)
	require.Equal(t, byte(7), decoded.([]interface{})[7])

	typ = mustTypeOf(t, "uint16[][]")
	encoded, err := typ.Encode([][]uint16{{1, 2, 3}, {}, {4}})
	require.NoError(t, err)
	decoded, err = typ.Decode(encoded)
	require.NoError(t, err)
	require.Len(t, decoded, 3)
}

// TestTypeOfDepthLimit checks that TypeOf rejects pathologically nested types instead of spending
// seconds of CPU on a short string, while ordinary nesting still parses.
func TestTypeOfDepthLimit(t *testing.T) {
	for _, d := range []int{1, 2, 10, 64, 100} {
		tuple := strings.Repeat("(", d) + "uint8" + strings.Repeat(")", d)
		typ, err := TypeOf(tuple)
		require.NoError(t, err, "tuple depth %d", d)
		require.Equal(t, tuple, typ.String())

		arr := "uint8" + strings.Repeat("[2]", d)
		typ, err = TypeOf(arr)
		require.NoError(t, err, "array depth %d", d)
		require.Equal(t, arr, typ.String())

		dyn := "uint8" + strings.Repeat("[]", d)
		_, err = TypeOf(dyn)
		require.NoError(t, err, "dynamic array depth %d", d)
	}

	const tooDeep = 5000
	for _, s := range []string{
		strings.Repeat("(", tooDeep) + "uint8" + strings.Repeat(")", tooDeep),
		"uint8" + strings.Repeat("[2]", tooDeep),
		"uint8" + strings.Repeat("[]", tooDeep),
		strings.Repeat("(", tooDeep) + strings.Repeat(")", tooDeep),
	} {
		_, err := TypeOf(s)
		require.Error(t, err, "depth %d", tooDeep)
		require.Contains(t, err.Error(), "nesting")
	}
}

// TestDecodeNeverPanics decodes every prefix of valid encodings, and randomly corrupted encodings,
// of several types: malformed input must produce an error, never a panic.
func TestDecodeNeverPanics(t *testing.T) {
	cases := []struct {
		typ   string
		value interface{}
	}{
		{"(string,uint8)", []interface{}{"hello", uint8(3)}},
		{"(uint8,bool,bool,string,uint64[2])", []interface{}{uint8(1), true, false, "x", []interface{}{uint64(1), uint64(2)}}},
		{"(bool[10],uint16)", []interface{}{[]interface{}{true, false, true, false, true, false, true, false, true, false}, uint16(9)}},
		{"string[]", []interface{}{"a", "bc", ""}},
		{"(uint8[],(string,bool))[2]", []interface{}{
			[]interface{}{[]interface{}{uint8(1)}, []interface{}{"a", true}},
			[]interface{}{[]interface{}{}, []interface{}{"", false}},
		}},
		{"address[2]", []interface{}{make([]byte, 32), make([]byte, 32)}},
	}
	rng := rand.New(rand.NewSource(1))
	for _, tc := range cases {
		typ := mustTypeOf(t, tc.typ)
		encoded, err := typ.Encode(tc.value)
		require.NoError(t, err, tc.typ)
		_, err = typ.Decode(encoded)
		require.NoError(t, err, tc.typ)

		for n := 0; n < len(encoded); n++ {
			require.NotPanics(t, func() { _, _ = typ.Decode(encoded[:n]) }, "%s prefix %d", tc.typ, n)
		}
		for i := 0; i < 500; i++ {
			mutated := append([]byte(nil), encoded...)
			for j := 0; j <= rng.Intn(3); j++ {
				mutated[rng.Intn(len(mutated))] = byte(rng.Intn(256))
			}
			require.NotPanics(t, func() { _, _ = typ.Decode(mutated) }, "%s mutated %x", tc.typ, mutated)
		}
	}
}

// TestDecodeZeroSizeElementsBounded checks that arrays of elements that take zero bytes to encode,
// whose count the input length cannot bound, do not multiply when nested.
func TestDecodeZeroSizeElementsBounded(t *testing.T) {
	const budget = 8 << 20 // one level of 65534 expanded types is about 4.7 MB
	for _, s := range []string{
		"()[65534][65534]",
		"()[65534][65534][65534]",
		"(()[65534],()[65534],()[65534],()[65534])",
	} {
		typ := mustTypeOf(t, s)
		for _, input := range [][]byte{nil, {0}, {0, 0}} {
			_, err := typ.Decode(input)
			require.Error(t, err, "%s %x", s, input)
			allocated := decodeAllocBytes(typ, input)
			require.Less(t, allocated, uint64(budget), "%s %x allocated %d bytes", s, input, allocated)
		}
	}
}
