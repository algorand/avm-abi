package abi

import (
	"fmt"
	"math"
	"regexp"
	"strconv"
	"strings"
)

// TypeKind is an enum value which indicates the kind of an ABI type.
type TypeKind uint32

const (
	// InvalidType represents an invalid and unused TypeKind.
	InvalidType TypeKind = iota
	// Uint is kind for ABI unsigned integer types, i.e. `uint<N>`.
	Uint
	// Byte is kind for the ABI `byte` type.
	Byte
	// Ufixed is the kind for ABI unsigned fixed point decimal types, i.e. `ufixed<N>x<M>`.
	Ufixed
	// Bool is the kind for the ABI `bool` type.
	Bool
	// ArrayStatic is the kind for ABI static array types, i.e. `<type>[<length>]`.
	ArrayStatic
	// Address is the kind for the ABI `address` type.
	Address
	// ArrayDynamic is the kind for ABI dynamic array types, i.e. `<type>[]`.
	ArrayDynamic
	// String is the kind for the ABI `string` type.
	String
	// Tuple is the kind for ABI tuple types, i.e. `(<type 0>,...,<type k>)`.
	Tuple
)

const (
	addressByteSize        = 32
	checksumByteSize       = 4
	singleByteSize         = 1
	singleBoolSize         = 1
	lengthEncodeByteSize   = 2
	abiEncodingLengthLimit = 1 << 16
)

// Type is the struct that represents an ABI type.
//
// Do not use the zero value of this struct. Use the `TypeOf` function to create an instance of an
// ABI type.
type Type struct {
	kind       TypeKind
	childTypes []Type

	// only can be applied to `uint` bitSize <N> or `ufixed` bitSize <N>
	bitSize uint16
	// only can be applied to `ufixed` precision <M>
	precision uint16

	// length for static array / tuple
	/*
		by ABI spec, len over binary array returns number of bytes
		the type is uint16, which allows for only length in [0, 2^16 - 1]
		representation of static length can only be constrained in uint16 type
	*/
	// NOTE may want to change back to uint32/uint64
	staticLength uint16
}

// String serialize an ABI Type to a string in ABI encoding.
func (t Type) String() string {
	switch t.kind {
	case Uint:
		return fmt.Sprintf("uint%d", t.bitSize)
	case Byte:
		return "byte"
	case Ufixed:
		return fmt.Sprintf("ufixed%dx%d", t.bitSize, t.precision)
	case Bool:
		return "bool"
	case ArrayStatic:
		return fmt.Sprintf("%s[%d]", t.childTypes[0].String(), t.staticLength)
	case Address:
		return "address"
	case ArrayDynamic:
		return t.childTypes[0].String() + "[]"
	case String:
		return "string"
	case Tuple:
		typeStrings := make([]string, len(t.childTypes))
		for i := 0; i < len(t.childTypes); i++ {
			typeStrings[i] = t.childTypes[i].String()
		}
		return "(" + strings.Join(typeStrings, ",") + ")"
	default:
		return "<invalid type>"
	}
}

var staticArrayRegexp = regexp.MustCompile(`^([a-z\d\[\](),]+)\[([1-9][\d]*)]$`)
var ufixedRegexp = regexp.MustCompile(`^ufixed([1-9][\d]*)x([1-9][\d]*)$`)

// TypeOf parses an ABI type string.
// For example: `TypeOf("(uint64,byte[])")`
//
// Note: this function only supports "basic" ABI types. Reference types and transaction types are
// not supported and will produce an error.
func TypeOf(str string) (Type, error) {
	switch {
	case strings.HasSuffix(str, "[]"):
		arrayArgType, err := TypeOf(str[:len(str)-2])
		if err != nil {
			return Type{}, err
		}
		return makeDynamicArrayType(arrayArgType), nil
	case strings.HasSuffix(str, "]"):
		stringMatches := staticArrayRegexp.FindStringSubmatch(str)
		// match the string itself, array element type, then array length
		if len(stringMatches) != 3 {
			return Type{}, fmt.Errorf(`static array ill formated: "%s"`, str)
		}
		// guaranteed that the length of array is existing
		arrayLengthStr := stringMatches[2]
		// allowing only decimal static array length, with limit size to 2^16 - 1
		arrayLength, err := strconv.ParseUint(arrayLengthStr, 10, 16)
		if err != nil {
			return Type{}, err
		}
		// parse the array element type
		arrayType, err := TypeOf(stringMatches[1])
		if err != nil {
			return Type{}, err
		}
		return makeStaticArrayType(arrayType, uint16(arrayLength)), nil
	case strings.HasPrefix(str, "uint"):
		typeSize, err := strconv.ParseUint(str[4:], 10, 16)
		if err != nil {
			return Type{}, fmt.Errorf(`ill formed uint type: "%s"`, str)
		}
		return makeUintType(int(typeSize))
	case str == "byte":
		return byteType, nil
	case strings.HasPrefix(str, "ufixed"):
		stringMatches := ufixedRegexp.FindStringSubmatch(str)
		// match string itself, then type-bitSize, and type-precision
		if len(stringMatches) != 3 {
			return Type{}, fmt.Errorf(`ill formed ufixed type: "%s"`, str)
		}
		// guaranteed that there are 2 uint strings in ufixed string
		ufixedSize, err := strconv.ParseUint(stringMatches[1], 10, 16)
		if err != nil {
			return Type{}, err
		}
		ufixedPrecision, err := strconv.ParseUint(stringMatches[2], 10, 16)
		if err != nil {
			return Type{}, err
		}
		return makeUfixedType(int(ufixedSize), int(ufixedPrecision))
	case str == "bool":
		return boolType, nil
	case str == "address":
		return addressType, nil
	case str == "string":
		return stringType, nil
	case len(str) >= 2 && str[0] == '(' && str[len(str)-1] == ')':
		// first start a rough check on if the number of brackets are balancing
		if strings.Count(str, "(") != strings.Count(str, ")") {
			return Type{}, fmt.Errorf("parsing error: tuple round bracket unbalanced")
		}
		if strings.Count(str, "[") != strings.Count(str, "]") {
			return Type{}, fmt.Errorf("parsing error: tuple square bracket unbalanced")
		}
		tupleContent, err := parseTupleContent(str[1 : len(str)-1])
		if err != nil {
			return Type{}, err
		}
		tupleTypes := make([]Type, len(tupleContent))
		for i := 0; i < len(tupleContent); i++ {
			ti, err := TypeOf(tupleContent[i])
			if err != nil {
				return Type{}, err
			}
			tupleTypes[i] = ti
		}
		return MakeTupleType(tupleTypes)
	default:
		return Type{}, fmt.Errorf(`cannot convert the string "%s" to an ABI type`, str)
	}
}

// segment keeps track of the start and end of a segment in a string.
type segment struct{ left, right int }

// parseTupleContent splits an ABI encoded string for tuple type into multiple sub-strings.
// Each sub-string represents a content type of the tuple type.
// The argument str is the content between parentheses of tuple, i.e.
//
// (...... str ......)
//	^               ^
func parseTupleContent(str string) ([]string, error) {
	// if the tuple type content is empty (which is also allowed)
	// just return the empty string list
	if len(str) == 0 {
		return []string{}, nil
	}

	// the following 2 checks want to make sure input string can be separated by comma
	// with form: "...substr_0,...substr_1,...,...substr_k"

	// str should noe have leading/tailing comma
	if strings.HasSuffix(str, ",") || strings.HasPrefix(str, ",") {
		return []string{}, fmt.Errorf("parsing error: tuple content should not start with comma")
	}

	// str should not have consecutive commas contained
	if strings.Contains(str, ",,") {
		return []string{}, fmt.Errorf("no consecutive commas")
	}

	var parenSegmentRecord = make([]segment, 0)
	var stack []int

	// get the most exterior parentheses segment (not overlapped by other parentheses)
	// illustration: "*****,(*****),*****" => ["*****", "(*****)", "*****"]
	// once iterate to left paren (, stack up by 1 in stack
	// iterate to right paren ), pop 1 in stack
	// if iterate to right paren ) with stack height 0, find a parenthesis segment "(******)"
	for index := 0; index < len(str); index++ {
		chr := str[index]
		if chr == '(' {
			stack = append(stack, index)
		} else if chr == ')' {
			if len(stack) == 0 {
				return []string{}, fmt.Errorf("unpaired parentheses: %s", str)
			}
			leftParenIndex := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			if len(stack) == 0 {
				// increase index til it meets comma, or end of string
				forwardIndex := index + 1
				for forwardIndex < len(str) && str[forwardIndex] != ',' {
					forwardIndex++
				}
				index = forwardIndex - 1
				parenSegmentRecord = append(parenSegmentRecord, segment{
					left:  leftParenIndex,
					right: index,
				})
			}
		}
	}
	if len(stack) != 0 {
		return []string{}, fmt.Errorf("unpaired parentheses: %s", str)
	}

	// take out tuple-formed type str in tuple argument
	strCopied := str
	for i := len(parenSegmentRecord) - 1; i >= 0; i-- {
		parenSeg := parenSegmentRecord[i]
		strCopied = strCopied[:parenSeg.left] + strCopied[parenSeg.right+1:]
	}

	// split the string without parenthesis segments
	tupleStrSegs := strings.Split(strCopied, ",")

	// the empty strings are placeholders for parenthesis segments
	// put the parenthesis segments back into segment list
	parenSegCount := 0
	for index, segStr := range tupleStrSegs {
		if segStr == "" {
			parenSeg := parenSegmentRecord[parenSegCount]
			tupleStrSegs[index] = str[parenSeg.left : parenSeg.right+1]
			parenSegCount++
		}
	}

	return tupleStrSegs, nil
}

// makeUintType makes `Uint` ABI type by taking a type bitSize argument.
// The range of type bitSize is [8, 512] and type bitSize % 8 == 0.
func makeUintType(typeSize int) (Type, error) {
	if typeSize%8 != 0 || typeSize < 8 || typeSize > 512 {
		return Type{}, fmt.Errorf("unsupported uint type bitSize: %d", typeSize)
	}
	return Type{
		kind:    Uint,
		bitSize: uint16(typeSize),
	}, nil
}

var (
	// byteType is ABI type constant for byte
	byteType = Type{kind: Byte}

	// boolType is ABI type constant for bool
	boolType = Type{kind: Bool}

	// addressType is ABI type constant for address
	addressType = Type{kind: Address}

	// stringType is ABI type constant for string
	stringType = Type{kind: String}
)

// makeUfixedType makes `UFixed` ABI type by taking type bitSize and type precision as arguments.
// The range of type bitSize is [8, 512] and type bitSize % 8 == 0.
// The range of type precision is [1, 160].
func makeUfixedType(typeSize int, typePrecision int) (Type, error) {
	if typeSize%8 != 0 || typeSize < 8 || typeSize > 512 {
		return Type{}, fmt.Errorf("unsupported ufixed type bitSize: %d", typeSize)
	}
	if typePrecision > 160 || typePrecision < 1 {
		return Type{}, fmt.Errorf("unsupported ufixed type precision: %d", typePrecision)
	}
	return Type{
		kind:      Ufixed,
		bitSize:   uint16(typeSize),
		precision: uint16(typePrecision),
	}, nil
}

// makeStaticArrayType makes static length array ABI type by taking
// array element type and array length as arguments.
func makeStaticArrayType(argumentType Type, arrayLength uint16) Type {
	return Type{
		kind:         ArrayStatic,
		childTypes:   []Type{argumentType},
		staticLength: arrayLength,
	}
}

// makeDynamicArrayType makes dynamic length array by taking array element type as argument.
func makeDynamicArrayType(argumentType Type) Type {
	return Type{
		kind:       ArrayDynamic,
		childTypes: []Type{argumentType},
	}
}

// MakeTupleType makes tuple ABI type by taking an array of tuple element types as argument.
func MakeTupleType(argumentTypes []Type) (Type, error) {
	if len(argumentTypes) >= math.MaxUint16 {
		return Type{}, fmt.Errorf("tuple type child type number larger than maximum uint16 error")
	}
	return Type{
		kind:         Tuple,
		childTypes:   argumentTypes,
		staticLength: uint16(len(argumentTypes)),
	}, nil
}

// Equal method decides the equality of two types: t == t0.
func (t Type) Equal(t0 Type) bool {
	if t.kind != t0.kind {
		return false
	}
	if t.precision != t0.precision || t.bitSize != t0.bitSize {
		return false
	}
	if t.staticLength != t0.staticLength {
		return false
	}
	if len(t.childTypes) != len(t0.childTypes) {
		return false
	}
	for i := 0; i < len(t.childTypes); i++ {
		if !t.childTypes[i].Equal(t0.childTypes[i]) {
			return false
		}
	}

	return true
}

// IsDynamic method decides if an ABI type is dynamic or static.
func (t Type) IsDynamic() bool {
	switch t.kind {
	case ArrayDynamic, String:
		return true
	default:
		for _, childT := range t.childTypes {
			if childT.IsDynamic() {
				return true
			}
		}
		return false
	}
}

// Assume that the current index on the list of type is an ABI bool type.
// It returns the difference between the current index and the index of the furthest consecutive Bool type.
func findBoolLR(typeList []Type, index int, delta int) int {
	until := 0
	for {
		curr := index + delta*until
		if typeList[curr].kind == Bool {
			if curr != len(typeList)-1 && delta > 0 {
				until++
			} else if curr > 0 && delta < 0 {
				until++
			} else {
				break
			}
		} else {
			until--
			break
		}
	}
	return until
}

// ByteLen method calculates the byte length of a static ABI type.
func (t Type) ByteLen() (int, error) {
	switch t.kind {
	case Address:
		return addressByteSize, nil
	case Byte:
		return singleByteSize, nil
	case Uint, Ufixed:
		return int(t.bitSize / 8), nil
	case Bool:
		return singleBoolSize, nil
	case ArrayStatic:
		if t.childTypes[0].kind == Bool {
			byteLen := int(t.staticLength+7) / 8
			return byteLen, nil
		}
		elemByteLen, err := t.childTypes[0].ByteLen()
		if err != nil {
			return -1, err
		}
		return int(t.staticLength) * elemByteLen, nil
	case Tuple:
		size := 0
		for i := 0; i < len(t.childTypes); i++ {
			if t.childTypes[i].kind == Bool {
				// search after bool
				after := findBoolLR(t.childTypes, i, 1)
				// shift the index
				i += after
				// get number of bool
				boolNum := after + 1
				size += (boolNum + 7) / 8
			} else {
				childByteSize, err := t.childTypes[i].ByteLen()
				if err != nil {
					return -1, err
				}
				size += childByteSize
			}
		}
		return size, nil
	default:
		return -1, fmt.Errorf("%s is a dynamic type", t.String())
	}
}

// AnyTransactionType is the ABI argument type string for a nonspecific transaction argument
const AnyTransactionType = "txn"

// PaymentTransactionType is the ABI argument type string for a payment transaction argument
const PaymentTransactionType = "pay"

// KeyRegistrationTransactionType is the ABI argument type string for a key registration transaction
// argument
const KeyRegistrationTransactionType = "keyreg"

// AssetConfigTransactionType is the ABI argument type string for an asset configuration transaction
// argument
const AssetConfigTransactionType = "acfg"

// AssetTransferTransactionType is the ABI argument type string for an asset transfer transaction
// argument
const AssetTransferTransactionType = "axfer"

// AssetFreezeTransactionType is the ABI argument type string for an asset freeze transaction
// argument
const AssetFreezeTransactionType = "afrz"

// ApplicationCallTransactionType is the ABI argument type string for an application call
// transaction argument
const ApplicationCallTransactionType = "appl"

// IsTransactionType checks if a type string represents a transaction type
// argument, such as "txn", "pay", "keyreg", etc.
func IsTransactionType(s string) bool {
	switch s {
	case AnyTransactionType,
		PaymentTransactionType,
		KeyRegistrationTransactionType,
		AssetConfigTransactionType,
		AssetTransferTransactionType,
		AssetFreezeTransactionType,
		ApplicationCallTransactionType:
		return true
	default:
		return false
	}
}

// AccountReferenceType is the ABI argument type string for account references
const AccountReferenceType = "account"

// AssetReferenceType is the ABI argument type string for asset references
const AssetReferenceType = "asset"

// ApplicationReferenceType is the ABI argument type string for application references
const ApplicationReferenceType = "application"

// IsReferenceType checks if a type string represents a reference type argument,
// such as "account", "asset", or "application".
func IsReferenceType(s string) bool {
	switch s {
	case AccountReferenceType, AssetReferenceType, ApplicationReferenceType:
		return true
	default:
		return false
	}
}

// VoidReturnType is the ABI return type string for a method that does not return any value
const VoidReturnType = "void"
