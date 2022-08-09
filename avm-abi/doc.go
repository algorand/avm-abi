/*
Package avm_abi provides an implementation of the Algorand ARC-4 ABI type system.

See https://arc.algorand.foundation/ARCs/arc-0004 for the corresponding specification.


Basic Operations

This package can parse ABI type names using the `avm_abi.TypeOf()` function.

That functions returns an `avm_abi.Type` struct. The `avm_abi.Type` struct's `Encode` and `Decode`
methods can convert between Go values and encoded ABI byte strings.
*/
package avm_abi
