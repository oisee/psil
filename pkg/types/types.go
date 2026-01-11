// Package types defines the core value types for PSIL.
// All values that can exist on the stack implement the Value interface.
package types

import (
	"fmt"
	"image"
	"image/color"
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

// Image represents a 2D image (for graphics operations)
type Image struct {
	Img    *image.RGBA
	Width  int
	Height int
}

// NewImage creates a new image with the given dimensions
func NewImage(width, height int) *Image {
	return &Image{
		Img:    image.NewRGBA(image.Rect(0, 0, width, height)),
		Width:  width,
		Height: height,
	}
}

func (img *Image) String() string {
	return fmt.Sprintf("<image:%dx%d>", img.Width, img.Height)
}

func (img *Image) Type() string { return "image" }

func (img *Image) Equal(other Value) bool {
	// Images are equal only if they are the same object
	if o, ok := other.(*Image); ok {
		return img == o
	}
	return false
}

// SetPixel sets a pixel color at (x, y) with RGB values 0-255
func (img *Image) SetPixel(x, y int, r, g, b uint8) {
	if x >= 0 && x < img.Width && y >= 0 && y < img.Height {
		img.Img.Set(x, y, color.RGBA{r, g, b, 255})
	}
}

// GetPixel gets the RGB values at (x, y)
func (img *Image) GetPixel(x, y int) (r, g, b uint8) {
	if x >= 0 && x < img.Width && y >= 0 && y < img.Height {
		c := img.Img.At(x, y).(color.RGBA)
		return c.R, c.G, c.B
	}
	return 0, 0, 0
}

// Error codes (stored in A register when C flag is set)
const (
	ErrNone             = 0
	ErrStackUnderflow   = 1
	ErrTypeMismatch     = 2
	ErrDivisionByZero   = 3
	ErrUndefinedSymbol  = 4
	ErrGasExhausted     = 5
	ErrInvalidQuotation = 6
	ErrImageError       = 7
	ErrFileError        = 8
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
	case ErrImageError:
		return "image error"
	case ErrFileError:
		return "file error"
	default:
		return fmt.Sprintf("unknown error %d", code)
	}
}
