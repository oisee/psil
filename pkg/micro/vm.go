package micro

import (
	"fmt"
	"io"
	"os"
)

// VM is the micro-PSIL virtual machine.
// Designed for easy Z80 translation.
type VM struct {
	// Stack holds tagged values
	// Each value: [size][data...] where size is 1-255 bytes
	Stack []byte
	SP    int // Stack pointer (points to next free byte)

	// Program
	Code []byte
	PC   int // Program counter

	// Quotations table (indexed by quotation number)
	Quotations [][]byte

	// Memory/symbols (256 slots, 2 bytes each)
	Memory [512]byte

	// Flags (Z80 style)
	ZFlag bool // Zero/comparison result
	CFlag bool // Carry/error flag
	AReg  byte // Accumulator/error code

	// Execution limits
	Gas    int
	MaxGas int

	// Call stack for quotation execution
	CallStack []int
	CallSP    int

	// Local variables (per call frame)
	Locals [16]int16

	// Output
	Output io.Writer

	// Debug mode
	Debug bool

	// Halted
	Halted bool
}

// New creates a new VM
func New() *VM {
	return &VM{
		Stack:      make([]byte, 1024),
		SP:         0,
		Quotations: make([][]byte, 256),
		CallStack:  make([]int, 64),
		Output:     os.Stdout,
		Gas:        0,
		MaxGas:     0,
	}
}

// Reset clears the VM state
func (vm *VM) Reset() {
	vm.SP = 0
	vm.PC = 0
	vm.ZFlag = false
	vm.CFlag = false
	vm.AReg = 0
	vm.CallSP = 0
	vm.Halted = false
	if vm.MaxGas > 0 {
		vm.Gas = vm.MaxGas
	}
}

// Load loads bytecode into the VM
func (vm *VM) Load(code []byte) {
	vm.Code = code
	vm.PC = 0
}

// DefineQuot defines a quotation
func (vm *VM) DefineQuot(idx int, code []byte) {
	if idx >= 0 && idx < len(vm.Quotations) {
		vm.Quotations[idx] = code
	}
}

// === Stack operations ===

// PushByte pushes a single byte value (size=1)
func (vm *VM) PushByte(v byte) {
	if vm.SP+2 > len(vm.Stack) {
		vm.CFlag = true
		vm.AReg = 1 // stack overflow
		return
	}
	vm.Stack[vm.SP] = 1 // size
	vm.Stack[vm.SP+1] = v
	vm.SP += 2
}

// PushWord pushes a 16-bit value (size=2)
func (vm *VM) PushWord(v int16) {
	if vm.SP+3 > len(vm.Stack) {
		vm.CFlag = true
		vm.AReg = 1
		return
	}
	vm.Stack[vm.SP] = 2 // size
	vm.Stack[vm.SP+1] = byte(v & 0xFF)
	vm.Stack[vm.SP+2] = byte((v >> 8) & 0xFF)
	vm.SP += 3
}

// PushInt pushes an integer (as 16-bit)
func (vm *VM) PushInt(v int) {
	vm.PushWord(int16(v))
}

// PopSize returns the size of top element without removing it
func (vm *VM) PopSize() int {
	if vm.SP < 2 {
		vm.CFlag = true
		vm.AReg = 2 // stack underflow
		return 0
	}
	// Walk back to find size byte
	// Stack: [...][size][data...]^SP
	// We need to find where current element starts
	pos := vm.SP - 1
	for pos > 0 && vm.Stack[pos-1] != 1 && vm.Stack[pos-1] != 2 {
		pos--
	}
	if pos > 0 {
		return int(vm.Stack[pos-1])
	}
	return 0
}

// PopByte pops a byte value
func (vm *VM) PopByte() byte {
	if vm.SP < 2 {
		vm.CFlag = true
		vm.AReg = 2
		return 0
	}
	size := vm.Stack[vm.SP-2]
	if size != 1 {
		// Try to coerce
		if size == 2 {
			v := vm.PopWord()
			return byte(v)
		}
		vm.CFlag = true
		vm.AReg = 3 // type error
		return 0
	}
	v := vm.Stack[vm.SP-1]
	vm.SP -= 2
	return v
}

// PopWord pops a 16-bit value
func (vm *VM) PopWord() int16 {
	if vm.SP < 3 {
		// Try as byte
		if vm.SP >= 2 && vm.Stack[vm.SP-2] == 1 {
			v := vm.Stack[vm.SP-1]
			vm.SP -= 2
			return int16(v)
		}
		vm.CFlag = true
		vm.AReg = 2
		return 0
	}
	size := vm.Stack[vm.SP-3]
	if size == 1 {
		// It's a byte, promote to word
		v := vm.Stack[vm.SP-2]
		vm.SP -= 2
		return int16(v)
	}
	if size != 2 {
		vm.CFlag = true
		vm.AReg = 3
		return 0
	}
	lo := vm.Stack[vm.SP-2]
	hi := vm.Stack[vm.SP-1]
	vm.SP -= 3
	return int16(lo) | (int16(hi) << 8)
}

// PopInt pops as int
func (vm *VM) PopInt() int {
	return int(vm.PopWord())
}

// PeekByte returns top byte without popping
func (vm *VM) PeekByte() byte {
	if vm.SP < 2 {
		return 0
	}
	return vm.Stack[vm.SP-1]
}

// PeekWord returns top word without popping
func (vm *VM) PeekWord() int16 {
	if vm.SP < 3 {
		if vm.SP >= 2 && vm.Stack[vm.SP-2] == 1 {
			return int16(vm.Stack[vm.SP-1])
		}
		return 0
	}
	if vm.Stack[vm.SP-3] == 2 {
		lo := vm.Stack[vm.SP-2]
		hi := vm.Stack[vm.SP-1]
		return int16(lo) | (int16(hi) << 8)
	}
	return int16(vm.Stack[vm.SP-1])
}

// Dup duplicates top value
func (vm *VM) Dup() {
	if vm.SP < 2 {
		vm.CFlag = true
		vm.AReg = 2
		return
	}
	// Find the size byte - it's at the start of the top element
	// For byte: [size=1][val], SP points after val, size at SP-2
	// For word: [size=2][lo][hi], SP points after hi, size at SP-3

	// Try word first (most common)
	if vm.SP >= 3 && vm.Stack[vm.SP-3] == 2 {
		lo := vm.Stack[vm.SP-2]
		hi := vm.Stack[vm.SP-1]
		vm.PushWord(int16(lo) | (int16(hi) << 8))
		return
	}
	// Try byte
	if vm.SP >= 2 && vm.Stack[vm.SP-2] == 1 {
		vm.PushByte(vm.Stack[vm.SP-1])
		return
	}
	vm.CFlag = true
	vm.AReg = 3 // type error
}

// Drop removes top value
func (vm *VM) Drop() {
	if vm.SP < 2 {
		vm.CFlag = true
		vm.AReg = 2
		return
	}
	// Try word first
	if vm.SP >= 3 && vm.Stack[vm.SP-3] == 2 {
		vm.SP -= 3
		return
	}
	// Try byte
	if vm.SP >= 2 && vm.Stack[vm.SP-2] == 1 {
		vm.SP -= 2
		return
	}
	// Fallback - just drop 3 bytes (word)
	if vm.SP >= 3 {
		vm.SP -= 3
	}
}

// Swap swaps top two values
func (vm *VM) Swap() {
	if vm.SP < 4 {
		vm.CFlag = true
		vm.AReg = 2
		return
	}
	// Simple case: both are bytes
	// [1][a][1][b] -> [1][b][1][a]
	a := vm.PopWord()
	b := vm.PopWord()
	vm.PushWord(a)
	vm.PushWord(b)
}

// Over copies second element to top
func (vm *VM) Over() {
	if vm.SP < 4 {
		vm.CFlag = true
		vm.AReg = 2
		return
	}
	a := vm.PopWord()
	b := vm.PeekWord()
	vm.PushWord(a)
	vm.PushWord(b)
}

// === Memory operations ===

// MemRead reads a 16-bit value from memory slot
func (vm *VM) MemRead(slot byte) int16 {
	idx := int(slot) * 2
	if idx+1 >= len(vm.Memory) {
		return 0
	}
	return int16(vm.Memory[idx]) | (int16(vm.Memory[idx+1]) << 8)
}

// MemWrite writes a 16-bit value to memory slot
func (vm *VM) MemWrite(slot byte, v int16) {
	idx := int(slot) * 2
	if idx+1 >= len(vm.Memory) {
		return
	}
	vm.Memory[idx] = byte(v & 0xFF)
	vm.Memory[idx+1] = byte((v >> 8) & 0xFF)
}

// === Execution ===

// Step executes one instruction
func (vm *VM) Step() error {
	if vm.Halted || vm.CFlag {
		return nil
	}

	if vm.PC >= len(vm.Code) {
		vm.Halted = true
		return nil
	}

	// Gas check
	if vm.MaxGas > 0 {
		vm.Gas--
		if vm.Gas <= 0 {
			vm.CFlag = true
			vm.AReg = 5 // gas exhausted
			return fmt.Errorf("gas exhausted")
		}
	}

	op := vm.Code[vm.PC]
	vm.PC++

	if vm.Debug {
		fmt.Fprintf(vm.Output, "  [%02X] %s SP=%d\n", op, OpName(op), vm.SP)
	}

	switch {
	// === 1-byte commands (0x00-0x1F) ===
	case op <= 0x1F:
		return vm.execCommand(op)

	// === Small numbers (0x20-0x3F) ===
	case IsSmallNum(op):
		vm.PushInt(SmallNumValue(op))

	// === Inline symbols (0x40-0x5F) ===
	case IsInlineSym(op):
		// Push symbol slot number (for use with @ and !)
		slot := op - 0x40
		vm.PushInt(int(slot))

	// === Inline quotations (0x60-0x7F) ===
	case IsInlineQuot(op):
		idx := InlineQuotIndex(op)
		vm.PushInt(idx | 0x8000) // Mark as quotation index

	// === 2-byte operations (0x80-0xBF) ===
	case Is2ByteOp(op):
		if vm.PC >= len(vm.Code) {
			vm.CFlag = true
			return fmt.Errorf("unexpected end of code")
		}
		arg := vm.Code[vm.PC]
		vm.PC++
		return vm.exec2Byte(op, arg)

	// === 3-byte operations (0xC0-0xDF) ===
	case Is3ByteOp(op):
		if vm.PC+1 >= len(vm.Code) {
			vm.CFlag = true
			return fmt.Errorf("unexpected end of code")
		}
		hi := vm.Code[vm.PC]
		lo := vm.Code[vm.PC+1]
		vm.PC += 2
		return vm.exec3Byte(op, hi, lo)

	// === Variable length (0xE0-0xEF) ===
	case IsVarLenOp(op):
		if vm.PC >= len(vm.Code) {
			vm.CFlag = true
			return fmt.Errorf("unexpected end of code")
		}
		length := int(vm.Code[vm.PC])
		vm.PC++
		if vm.PC+length > len(vm.Code) {
			vm.CFlag = true
			return fmt.Errorf("string extends past end")
		}
		data := vm.Code[vm.PC : vm.PC+length]
		vm.PC += length
		return vm.execVarLen(op, data)

	// === Special operations (0xF0-0xFF) ===
	case op == OpHalt:
		vm.Halted = true
	case op == OpYield:
		vm.Halted = true
		return nil
	case op == OpEnd:
		vm.Halted = true
	case op == OpError:
		vm.CFlag = true
	case op == OpClearE:
		vm.CFlag = false
		vm.AReg = 0
	case op == OpCheckE:
		if vm.CFlag {
			vm.PushInt(1)
		} else {
			vm.PushInt(0)
		}
	}

	return nil
}

// execCommand executes a 1-byte command
func (vm *VM) execCommand(op byte) error {
	switch op {
	case OpNop:
		// nothing

	case OpDup:
		vm.Dup()

	case OpDrop:
		vm.Drop()

	case OpSwap:
		vm.Swap()

	case OpOver:
		vm.Over()

	case OpRot:
		c := vm.PopWord()
		b := vm.PopWord()
		a := vm.PopWord()
		vm.PushWord(b)
		vm.PushWord(c)
		vm.PushWord(a)

	case OpAdd:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.PushInt(a + b)

	case OpSub:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.PushInt(a - b)

	case OpMul:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.PushInt(a * b)

	case OpDiv:
		b := vm.PopInt()
		a := vm.PopInt()
		if b == 0 {
			vm.CFlag = true
			vm.AReg = 4 // division by zero
			vm.PushInt(0)
		} else {
			vm.PushInt(a / b)
		}

	case OpMod:
		b := vm.PopInt()
		a := vm.PopInt()
		if b == 0 {
			vm.CFlag = true
			vm.AReg = 4
			vm.PushInt(0)
		} else {
			vm.PushInt(a % b)
		}

	case OpEq:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.ZFlag = (a == b)
		if a == b {
			vm.PushInt(1)
		} else {
			vm.PushInt(0)
		}

	case OpLt:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.ZFlag = (a < b)
		if a < b {
			vm.PushInt(1)
		} else {
			vm.PushInt(0)
		}

	case OpGt:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.ZFlag = (a > b)
		if a > b {
			vm.PushInt(1)
		} else {
			vm.PushInt(0)
		}

	case OpAnd:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.PushInt(a & b)

	case OpOr:
		b := vm.PopInt()
		a := vm.PopInt()
		vm.PushInt(a | b)

	case OpNot:
		a := vm.PopInt()
		if a == 0 {
			vm.PushInt(1)
		} else {
			vm.PushInt(0)
		}

	case OpNeg:
		a := vm.PopInt()
		vm.PushInt(-a)

	case OpExec:
		// Pop quotation index and execute
		idx := vm.PopInt()
		if idx&0x8000 != 0 {
			idx &= 0x7FFF
		}
		return vm.execQuotation(idx)

	case OpIfte:
		// [cond] [then] [else] -> result
		elseQ := vm.PopInt() & 0x7FFF
		thenQ := vm.PopInt() & 0x7FFF
		cond := vm.PopInt()
		if cond != 0 {
			return vm.execQuotation(thenQ)
		} else {
			return vm.execQuotation(elseQ)
		}

	case OpDip:
		// x [q] -> ... x
		qIdx := vm.PopInt() & 0x7FFF
		x := vm.PopWord()
		if err := vm.execQuotation(qIdx); err != nil {
			return err
		}
		vm.PushWord(x)

	case OpLoop:
		// n [q] -> ...
		qIdx := vm.PopInt() & 0x7FFF
		n := vm.PopInt()
		for i := 0; i < n && !vm.CFlag; i++ {
			if err := vm.execQuotation(qIdx); err != nil {
				return err
			}
		}

	case OpRet:
		// Return from quotation (handled by execQuotation)
		vm.Halted = true

	case OpLoad:
		// sym -> value
		slot := byte(vm.PopInt())
		v := vm.MemRead(slot)
		vm.PushWord(v)

	case OpStore:
		// value sym ->
		slot := byte(vm.PopInt())
		v := vm.PopWord()
		vm.MemWrite(slot, v)

	case OpPrint:
		v := vm.PopInt()
		fmt.Fprintf(vm.Output, "%d", v)

	case OpInc:
		a := vm.PopInt()
		vm.PushInt(a + 1)

	case OpDec:
		a := vm.PopInt()
		vm.PushInt(a - 1)

	case OpDup2:
		b := vm.PopWord()
		a := vm.PopWord()
		vm.PushWord(a)
		vm.PushWord(b)
		vm.PushWord(a)
		vm.PushWord(b)

	case OpDepth:
		// Count stack elements (rough estimate)
		vm.PushInt(vm.SP / 2)

	case OpClear:
		vm.SP = 0
	}

	return nil
}

// exec2Byte executes a 2-byte operation
func (vm *VM) exec2Byte(op, arg byte) error {
	switch op {
	case OpPushByte:
		vm.PushByte(arg)

	case OpSymbol:
		v := vm.MemRead(arg)
		vm.PushWord(v)

	case OpQuotation:
		vm.PushInt(int(arg) | 0x8000)

	case OpLocal:
		if arg < 16 {
			vm.PushWord(vm.Locals[arg])
		}

	case OpSetLocal:
		if arg < 16 {
			vm.Locals[arg] = vm.PopWord()
		}

	case OpJump:
		vm.PC += int(arg)

	case OpJumpBack:
		vm.PC -= int(arg)

	case OpJumpZ:
		v := vm.PopInt()
		if v == 0 {
			vm.PC += int(arg)
		}

	case OpJumpNZ:
		v := vm.PopInt()
		if v != 0 {
			vm.PC += int(arg)
		}

	case OpCall:
		// Call builtin by number
		return vm.callBuiltin(int(arg))

	case OpRing0R:
		// Read-only slot
		v := vm.MemRead(arg)
		vm.PushWord(v)

	case OpRing1R:
		// Read slot 64+
		v := vm.MemRead(64 + arg)
		vm.PushWord(v)

	case OpRing1W:
		// Write slot 64+
		v := vm.PopWord()
		vm.MemWrite(64+arg, v)

	case OpInspect:
		// Push stack depth at position arg
		vm.PushInt(vm.SP)

	case OpGas:
		// Consume gas
		vm.Gas -= int(arg)
		if vm.Gas < 0 {
			vm.CFlag = true
			vm.AReg = 5
		}

	case OpPickN:
		// This is tricky with tagged values
		// For now, simple implementation
		n := int(arg)
		if n*3 > vm.SP {
			vm.CFlag = true
			return nil
		}
		// Read value at position n from top
		pos := vm.SP - (n+1)*3
		if pos >= 0 && vm.Stack[pos] == 2 {
			lo := vm.Stack[pos+1]
			hi := vm.Stack[pos+2]
			vm.PushWord(int16(lo) | (int16(hi) << 8))
		}

	case OpLoopN:
		// Loop next quotation N times
		// The quotation follows inline
		qIdx := vm.PopInt() & 0x7FFF
		for i := 0; i < int(arg) && !vm.CFlag; i++ {
			if err := vm.execQuotation(qIdx); err != nil {
				return err
			}
		}

	case OpString:
		// Push string length and data pointer (simplified)
		vm.PushInt(int(arg))
	}

	return nil
}

// exec3Byte executes a 3-byte operation
func (vm *VM) exec3Byte(op, hi, lo byte) error {
	val := int16(lo) | (int16(hi) << 8)

	switch op {
	case OpPushWord:
		vm.PushWord(val)

	case OpSymbol16:
		v := vm.MemRead(lo) // Use lo as slot, ignore hi for now
		vm.PushWord(v)

	case OpQuot16:
		vm.PushInt(int(val) | 0x8000)

	case OpJumpFar:
		vm.PC += int(val)

	case OpJumpZFar:
		v := vm.PopInt()
		if v == 0 {
			vm.PC += int(val)
		}

	case OpCallFar:
		// Save return address and jump
		vm.CallStack[vm.CallSP] = vm.PC
		vm.CallSP++
		vm.PC = int(val)
	}

	return nil
}

// execVarLen executes a variable-length operation
func (vm *VM) execVarLen(op byte, data []byte) error {
	switch op {
	case OpStringVar:
		// For now, just push the length
		vm.PushInt(len(data))

	case OpQuotVar:
		// Inline quotation - execute it
		oldPC := vm.PC
		oldCode := vm.Code
		vm.Code = data
		vm.PC = 0
		for vm.PC < len(data) && !vm.CFlag && !vm.Halted {
			if err := vm.Step(); err != nil {
				vm.Code = oldCode
				vm.PC = oldPC
				return err
			}
		}
		vm.Code = oldCode
		vm.PC = oldPC
		vm.Halted = false
	}

	return nil
}

// execQuotation executes a quotation by index
func (vm *VM) execQuotation(idx int) error {
	if idx < 0 || idx >= len(vm.Quotations) || vm.Quotations[idx] == nil {
		vm.CFlag = true
		vm.AReg = 6 // invalid quotation
		return fmt.Errorf("invalid quotation %d", idx)
	}

	// Save state
	oldPC := vm.PC
	oldCode := vm.Code

	// Execute quotation
	vm.Code = vm.Quotations[idx]
	vm.PC = 0

	for vm.PC < len(vm.Code) && !vm.CFlag {
		if err := vm.Step(); err != nil {
			vm.Code = oldCode
			vm.PC = oldPC
			return err
		}
		if vm.Halted {
			vm.Halted = false // Reset for return
			break
		}
	}

	// Restore state
	vm.Code = oldCode
	vm.PC = oldPC

	return nil
}

// callBuiltin calls a builtin function by number
func (vm *VM) callBuiltin(n int) error {
	switch n {
	case 0: // print newline
		fmt.Fprintln(vm.Output)
	case 1: // print space
		fmt.Fprint(vm.Output, " ")
	case 2: // print as char
		c := vm.PopInt()
		fmt.Fprintf(vm.Output, "%c", c)
	case 3: // abs
		a := vm.PopInt()
		if a < 0 {
			a = -a
		}
		vm.PushInt(a)
	case 4: // min
		b := vm.PopInt()
		a := vm.PopInt()
		if a < b {
			vm.PushInt(a)
		} else {
			vm.PushInt(b)
		}
	case 5: // max
		b := vm.PopInt()
		a := vm.PopInt()
		if a > b {
			vm.PushInt(a)
		} else {
			vm.PushInt(b)
		}
	}
	return nil
}

// Run executes until halted or error
func (vm *VM) Run() error {
	for !vm.Halted && !vm.CFlag {
		if err := vm.Step(); err != nil {
			return err
		}
	}
	return nil
}

// StackDump returns a string representation of the stack
func (vm *VM) StackDump() string {
	if vm.SP == 0 {
		return "[]"
	}
	s := "[ "
	pos := 0
	for pos < vm.SP {
		if pos+1 >= vm.SP {
			break
		}
		size := int(vm.Stack[pos])
		if size == 1 && pos+1 < vm.SP {
			s += fmt.Sprintf("%d ", vm.Stack[pos+1])
			pos += 2
		} else if size == 2 && pos+2 < vm.SP {
			v := int16(vm.Stack[pos+1]) | (int16(vm.Stack[pos+2]) << 8)
			s += fmt.Sprintf("%d ", v)
			pos += 3
		} else {
			break
		}
	}
	return s + "]"
}
