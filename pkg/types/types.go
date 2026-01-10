// Package types defines the core value types for PSIL.
// All values that can exist on the stack implement the Value interface.
package types

import (
	"fmt"
	"strings"
)

// Value is the interface all PSIL values implement.
type Value interface {
	// String returns a human-readable representation
	String() string
	// Type returns the type name for error messages
	Type() string
	// Equal checks equality with another value
	Equal(other Value) bool
}

// Number represents a numeric value (float64 for simplicity, can handle ints too)
type Number float64

func (n Number) String() string {
	// Format nicely - no trailing zeros for whole numbers
	if n == Number(int64(n)) {
		return fmt.Sprintf("%d", int64(n))
	}
	return fmt.Sprintf("%g", n)
}

func (n Number) Type() string { return "number" }

func (n Number) Equal(other Value) bool {
	if o, ok := other.(Number); ok {
		return n == o
	}
	return false
}

// String represents a string value
type String string

func (s String) String() string { return fmt.Sprintf("%q", string(s)) }
func (s String) Type() string   { return "string" }

func (s String) Equal(other Value) bool {
	if o, ok := other.(String); ok {
		return s == o
	}
	return false
}

// Boolean represents true/false
type Boolean bool

func (b Boolean) String() string {
	if b {
		return "true"
	}
	return "false"
}

func (b Boolean) Type() string { return "boolean" }

func (b Boolean) Equal(other Value) bool {
	if o, ok := other.(Boolean); ok {
		return b == o
	}
	return false
}

// Symbol represents an unresolved identifier (resolved at runtime)
type Symbol string

func (s Symbol) String() string { return string(s) }
func (s Symbol) Type() string   { return "symbol" }

func (s Symbol) Equal(other Value) bool {
	if o, ok := other.(Symbol); ok {
		return s == o
	}
	return false
}

// Quotation represents a block of code (list of values).
// This is the key type - quotations are first-class values.
type Quotation struct {
	Items []Value
}

func (q *Quotation) String() string {
	var parts []string
	for _, item := range q.Items {
		parts = append(parts, item.String())
	}
	return "[ " + strings.Join(parts, " ") + " ]"
}

func (q *Quotation) Type() string { return "quotation" }

func (q *Quotation) Equal(other Value) bool {
	if o, ok := other.(*Quotation); ok {
		if len(q.Items) != len(o.Items) {
			return false
		}
		for i, item := range q.Items {
			if !item.Equal(o.Items[i]) {
				return false
			}
		}
		return true
	}
	return false
}

// Builtin represents a native Go function.
// It takes the interpreter and returns an error.
type Builtin struct {
	Name string
	Fn   func(interp interface{}) error
}

func (b *Builtin) String() string { return "<builtin:" + b.Name + ">" }
func (b *Builtin) Type() string   { return "builtin" }

func (b *Builtin) Equal(other Value) bool {
	if o, ok := other.(*Builtin); ok {
		return b.Name == o.Name
	}
	return false
}

// Error codes (stored in A register when C flag is set)
const (
	ErrNone           = 0
	ErrStackUnderflow = 1
	ErrTypeMismatch   = 2
	ErrDivisionByZero = 3
	ErrUndefinedSymbol = 4
	ErrGasExhausted   = 5
	ErrInvalidQuotation = 6
)

// ErrorMessage returns a human-readable error message for an error code
func ErrorMessage(code int) string {
	switch code {
	case ErrNone:
		return "no error"
	case ErrStackUnderflow:
		return "stack underflow"
	case ErrTypeMismatch:
		return "type mismatch"
	case ErrDivisionByZero:
		return "division by zero"
	case ErrUndefinedSymbol:
		return "undefined symbol"
	case ErrGasExhausted:
		return "gas exhausted"
	case ErrInvalidQuotation:
		return "invalid quotation"
	default:
		return fmt.Sprintf("unknown error %d", code)
	}
}
