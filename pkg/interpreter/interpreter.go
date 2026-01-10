// Package interpreter provides the PSIL execution engine.
// It manages the stack, dictionary, flags (Z, C), and A register.
package interpreter

import (
	"fmt"
	"io"
	"os"

	"github.com/psilLang/psil/pkg/types"
)

// Interpreter is the PSIL execution engine
type Interpreter struct {
	// Stack is the main data stack
	Stack []types.Value

	// Dictionary maps names to values (quotations or builtins)
	Dictionary map[string]types.Value

	// ZFlag is set by boolean operations (true = Z set)
	ZFlag bool

	// CFlag indicates an error condition (true = error)
	CFlag bool

	// ARegister holds the error code when CFlag is set
	ARegister int

	// Gas is the computation budget (0 = unlimited)
	Gas int
	// MaxGas is the starting gas amount
	MaxGas int

	// Output writer (default: os.Stdout)
	Output io.Writer

	// Debug mode shows extra info
	Debug bool
}

// New creates a new Interpreter with builtins registered
func New() *Interpreter {
	interp := &Interpreter{
		Stack:      make([]types.Value, 0, 64),
		Dictionary: make(map[string]types.Value),
		Output:     os.Stdout,
		Gas:        0, // unlimited by default
	}

	// Register all builtins and combinators
	interp.RegisterBuiltins()
	interp.RegisterCombinators()

	return interp
}

// Reset clears the stack and flags, keeps dictionary
func (i *Interpreter) Reset() {
	i.Stack = i.Stack[:0]
	i.ZFlag = false
	i.CFlag = false
	i.ARegister = 0
	if i.MaxGas > 0 {
		i.Gas = i.MaxGas
	}
}

// SetError sets the error flag and code
func (i *Interpreter) SetError(code int) {
	i.CFlag = true
	i.ARegister = code
}

// ClearError clears the error flag
func (i *Interpreter) ClearError() {
	i.CFlag = false
	i.ARegister = 0
}

// HasError returns true if error flag is set
func (i *Interpreter) HasError() bool {
	return i.CFlag
}

// ConsumeGas decrements gas and returns true if execution can continue
func (i *Interpreter) ConsumeGas(amount int) bool {
	if i.MaxGas == 0 {
		return true // unlimited
	}
	i.Gas -= amount
	if i.Gas <= 0 {
		i.SetError(types.ErrGasExhausted)
		return false
	}
	return true
}

// Push pushes a value onto the stack
func (i *Interpreter) Push(v types.Value) {
	i.Stack = append(i.Stack, v)
}

// Pop removes and returns the top value from the stack
// Returns nil and sets error if stack is empty
func (i *Interpreter) Pop() types.Value {
	if len(i.Stack) == 0 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	v := i.Stack[len(i.Stack)-1]
	i.Stack = i.Stack[:len(i.Stack)-1]
	return v
}

// Peek returns the top value without removing it
func (i *Interpreter) Peek() types.Value {
	if len(i.Stack) == 0 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	return i.Stack[len(i.Stack)-1]
}

// PeekN returns the Nth value from top (0 = top)
func (i *Interpreter) PeekN(n int) types.Value {
	idx := len(i.Stack) - 1 - n
	if idx < 0 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	return i.Stack[idx]
}

// PopNumber pops a number, sets error if not a number
func (i *Interpreter) PopNumber() (types.Number, bool) {
	v := i.Pop()
	if v == nil {
		return 0, false
	}
	n, ok := v.(types.Number)
	if !ok {
		i.SetError(types.ErrTypeMismatch)
		return 0, false
	}
	return n, true
}

// PopQuotation pops a quotation, sets error if not a quotation
func (i *Interpreter) PopQuotation() (*types.Quotation, bool) {
	v := i.Pop()
	if v == nil {
		return nil, false
	}
	q, ok := v.(*types.Quotation)
	if !ok {
		i.SetError(types.ErrTypeMismatch)
		return nil, false
	}
	return q, true
}

// PopBoolean pops a boolean, sets error if not boolean
func (i *Interpreter) PopBoolean() (types.Boolean, bool) {
	v := i.Pop()
	if v == nil {
		return false, false
	}
	b, ok := v.(types.Boolean)
	if !ok {
		i.SetError(types.ErrTypeMismatch)
		return false, false
	}
	return b, true
}

// Define adds a definition to the dictionary
func (i *Interpreter) Define(name string, value types.Value) {
	i.Dictionary[name] = value
}

// Lookup looks up a name in the dictionary
func (i *Interpreter) Lookup(name string) (types.Value, bool) {
	v, ok := i.Dictionary[name]
	return v, ok
}

// Execute executes a single value
func (i *Interpreter) Execute(v types.Value) error {
	// Check for error propagation - skip if error is set
	if i.CFlag {
		return nil
	}

	// Consume gas
	if !i.ConsumeGas(1) {
		return fmt.Errorf("gas exhausted")
	}

	switch val := v.(type) {
	case types.Number:
		i.Push(val)

	case types.String:
		i.Push(val)

	case types.Boolean:
		i.Push(val)

	case *types.Quotation:
		// Quotations are pushed, not executed
		i.Push(val)

	case types.Symbol:
		// Look up and execute
		if def, ok := i.Dictionary[string(val)]; ok {
			switch d := def.(type) {
			case *types.Quotation:
				// Execute the quotation's contents
				return i.ExecuteQuotation(d)
			case *types.Builtin:
				// Execute the builtin
				return d.Fn(i)
			default:
				// Push other values
				i.Push(def)
			}
		} else {
			i.SetError(types.ErrUndefinedSymbol)
			return fmt.Errorf("undefined symbol: %s", val)
		}

	case *types.Builtin:
		return val.Fn(i)
	}

	return nil
}

// ExecuteQuotation executes all items in a quotation
func (i *Interpreter) ExecuteQuotation(q *types.Quotation) error {
	for _, item := range q.Items {
		if err := i.Execute(item); err != nil {
			return err
		}
		if i.CFlag {
			break // Stop on error
		}
	}
	return nil
}

// Run executes a slice of values (the main program)
func (i *Interpreter) Run(values []types.Value) error {
	for _, v := range values {
		if err := i.Execute(v); err != nil {
			return err
		}
		if i.CFlag {
			break
		}
	}
	return nil
}

// StackString returns a string representation of the stack
func (i *Interpreter) StackString() string {
	if len(i.Stack) == 0 {
		return "[]"
	}
	s := "[ "
	for _, v := range i.Stack {
		s += v.String() + " "
	}
	return s + "]"
}

// FlagsString returns a string representation of flags
func (i *Interpreter) FlagsString() string {
	z := "0"
	if i.ZFlag {
		z = "1"
	}
	c := "0"
	if i.CFlag {
		c = "1"
	}
	return fmt.Sprintf("Z=%s C=%s A=%d", z, c, i.ARegister)
}
