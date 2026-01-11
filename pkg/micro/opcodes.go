// Package micro implements a minimal bytecode VM for PSIL.
// Designed for easy Z80/6502 implementation with UTF-8 style encoding.
package micro

// Bytecode encoding (UTF-8 style):
//
// 0x00-0x7F: 1 byte (hot path - 128 values)
//   0x00-0x1F: Commands without arguments (32)
//   0x20-0x3F: Small numbers 0-31 (32)
//   0x40-0x5F: Inline symbols (32)
//   0x60-0x7F: Inline quotations (32)
//
// 0x80-0xBF: 2 bytes [op][arg] (main path - 64 ops × 256 args)
//
// 0xC0-0xDF: 3 bytes [op][hi][lo] (rare path)
//
// 0xE0-0xEF: Variable length [op][len][data...]
//
// 0xF0-0xFF: Special/control

// === 1-byte opcodes (0x00-0x1F) - Commands ===
const (
	OpNop    = 0x00 // no operation
	OpDup    = 0x01 // duplicate top
	OpDrop   = 0x02 // remove top
	OpSwap   = 0x03 // swap top two
	OpOver   = 0x04 // copy second to top
	OpRot    = 0x05 // rotate top three
	OpAdd    = 0x06 // a b -- (a+b)
	OpSub    = 0x07 // a b -- (a-b)
	OpMul    = 0x08 // a b -- (a*b)
	OpDiv    = 0x09 // a b -- (a/b)
	OpMod    = 0x0A // a b -- (a%b)
	OpEq     = 0x0B // a b -- (a==b)
	OpLt     = 0x0C // a b -- (a<b)
	OpGt     = 0x0D // a b -- (a>b)
	OpAnd    = 0x0E // a b -- (a&b)
	OpOr     = 0x0F // a b -- (a|b)
	OpNot    = 0x10 // a -- (!a)
	OpNeg    = 0x11 // a -- (-a)
	OpExec   = 0x12 // [q] -- ... (execute quotation)
	OpIfte   = 0x13 // cond [t] [f] -- ...
	OpDip    = 0x14 // x [q] -- ... x
	OpLoop   = 0x15 // n [q] -- ... (execute n times)
	OpRet    = 0x16 // return from quotation
	OpLoad   = 0x17 // sym -- value (load from memory)
	OpStore  = 0x18 // value sym -- (store to memory)
	OpPrint  = 0x19 // a -- (print top)
	OpInc    = 0x1A // a -- (a+1)
	OpDec    = 0x1B // a -- (a-1)
	OpDup2   = 0x1C // a b -- a b a b
	OpPick   = 0x1D // ... n -- ... nth (reserved, needs arg)
	OpDepth  = 0x1E // -- n (stack depth)
	OpClear  = 0x1F // ... -- (clear stack)
)

// === 1-byte literals (0x20-0x3F) - Small numbers ===
const (
	OpNum0  = 0x20 // push 0
	OpNum31 = 0x3F // push 31
)

// IsSmallNum returns true if opcode is a small number literal
func IsSmallNum(op byte) bool {
	return op >= 0x20 && op <= 0x3F
}

// SmallNumValue returns the value of a small number opcode
func SmallNumValue(op byte) int {
	return int(op - 0x20)
}

// SmallNumOp returns the opcode for a small number (0-31)
func SmallNumOp(n int) byte {
	if n < 0 || n > 31 {
		return OpNum0
	}
	return byte(0x20 + n)
}

// === 1-byte symbols (0x40-0x5F) - Inline symbols ===
const (
	SymNil     = 0x40 // nil
	SymTrue    = 0x41 // true
	SymFalse   = 0x42 // false
	SymSelf    = 0x43 // self (current entity)
	SymTarget  = 0x44 // target
	SymHealth  = 0x45 // health
	SymEnergy  = 0x46 // energy
	SymPos     = 0x47 // position
	SymAnger   = 0x48 // anger
	SymFear    = 0x49 // fear
	SymTrust   = 0x4A // trust
	SymHunger  = 0x4B // hunger
	SymEnemy   = 0x4C // enemy
	SymFriend  = 0x4D // friend
	SymFood    = 0x4E // food
	SymDanger  = 0x4F // danger
	SymSafe    = 0x50 // safe
	SymNear    = 0x51 // near
	SymFar     = 0x52 // far
	SymDay     = 0x53 // day
	SymNight   = 0x54 // night
	SymResult  = 0x55 // result (last operation)
	SymCount   = 0x56 // count/counter
	SymTemp    = 0x57 // temp variable
	SymX       = 0x58 // x coordinate
	SymY       = 0x59 // y coordinate
	// 0x5A-0x5F reserved for future symbols
)

// IsInlineSym returns true if opcode is an inline symbol
func IsInlineSym(op byte) bool {
	return op >= 0x40 && op <= 0x5F
}

// === 1-byte quotations (0x60-0x7F) - Inline quotation indices ===
const (
	Quot0  = 0x60 // quotation index 0
	Quot31 = 0x7F // quotation index 31
)

// IsInlineQuot returns true if opcode is an inline quotation
func IsInlineQuot(op byte) bool {
	return op >= 0x60 && op <= 0x7F
}

// InlineQuotIndex returns the quotation index from opcode
func InlineQuotIndex(op byte) int {
	return int(op - 0x60)
}

// InlineQuotOp returns the opcode for a quotation index (0-31)
func InlineQuotOp(idx int) byte {
	if idx < 0 || idx > 31 {
		return Quot0
	}
	return byte(0x60 + idx)
}

// === 2-byte opcodes (0x80-0xBF) [op][arg] ===
const (
	OpPushByte  = 0x80 // [n] push byte value
	OpSymbol    = 0x81 // [n] push extended symbol
	OpQuotation = 0x82 // [n] push extended quotation index
	OpLocal     = 0x83 // [n] push local variable n
	OpSetLocal  = 0x84 // [n] store to local variable n
	OpJump      = 0x85 // [n] jump forward n bytes
	OpJumpBack  = 0x86 // [n] jump backward n bytes
	OpJumpZ     = 0x87 // [n] jump if zero
	OpJumpNZ    = 0x88 // [n] jump if not zero
	OpCall      = 0x89 // [n] call builtin n
	OpRing0R    = 0x8A // [n] read ring0 slot n
	OpRing1R    = 0x8B // [n] read ring1 slot n
	OpRing1W    = 0x8C // [n] write ring1 slot n
	OpInspect   = 0x8D // [n] inspect depth n
	OpGas       = 0x8E // [n] check/consume n gas
	OpPickN     = 0x8F // [n] pick nth element
	OpRollN     = 0x90 // [n] roll nth element to top
	OpLoopN     = 0x91 // [n] loop n times (next bytes = body)
	OpString    = 0x92 // [len] followed by len bytes
	// 0x93-0xBF reserved
)

// Is2ByteOp returns true if opcode is a 2-byte operation
func Is2ByteOp(op byte) bool {
	return op >= 0x80 && op <= 0xBF
}

// === 3-byte opcodes (0xC0-0xDF) [op][hi][lo] ===
const (
	OpPushWord  = 0xC0 // [hi][lo] push 16-bit value
	OpSymbol16  = 0xC1 // [hi][lo] extended symbol (16-bit)
	OpQuot16    = 0xC2 // [hi][lo] extended quotation (16-bit)
	OpJumpFar   = 0xC3 // [hi][lo] far jump
	OpJumpZFar  = 0xC4 // [hi][lo] far jump if zero
	OpCallFar   = 0xC5 // [hi][lo] call address
	// 0xC6-0xDF reserved
)

// Is3ByteOp returns true if opcode is a 3-byte operation
func Is3ByteOp(op byte) bool {
	return op >= 0xC0 && op <= 0xDF
}

// === Variable length opcodes (0xE0-0xEF) [op][len][data...] ===
const (
	OpStringVar = 0xE0 // [len][bytes...] string literal
	OpBytesVar  = 0xE1 // [len][bytes...] raw bytes
	OpVectorVar = 0xE2 // [len][items...] vector of values
	OpQuotVar   = 0xE3 // [len][bytes...] inline quotation body
	// 0xE4-0xEF reserved
)

// IsVarLenOp returns true if opcode is variable length
func IsVarLenOp(op byte) bool {
	return op >= 0xE0 && op <= 0xEF
}

// === Special opcodes (0xF0-0xFF) ===
const (
	OpHalt    = 0xF0 // halt execution
	OpYield   = 0xF1 // yield to scheduler
	OpBreak   = 0xF2 // breakpoint
	OpDebug   = 0xF3 // debug print
	OpError   = 0xF4 // set error flag
	OpClearE  = 0xF5 // clear error
	OpCheckE  = 0xF6 // check error flag
	OpExtend  = 0xFE // [ext][...] extended opcode
	OpEnd     = 0xFF // end marker
)

// IsSpecialOp returns true if opcode is a special operation
func IsSpecialOp(op byte) bool {
	return op >= 0xF0
}

// OpName returns the name of an opcode for debugging
func OpName(op byte) string {
	switch {
	case op <= 0x1F:
		names := []string{
			"nop", "dup", "drop", "swap", "over", "rot",
			"+", "-", "*", "/", "mod", "=", "<", ">",
			"and", "or", "not", "neg", "exec", "ifte",
			"dip", "loop", "ret", "load", "store", "print",
			"inc", "dec", "dup2", "pick", "depth", "clear",
		}
		return names[op]
	case IsSmallNum(op):
		return "num"
	case IsInlineSym(op):
		return "sym"
	case IsInlineQuot(op):
		return "quot"
	case Is2ByteOp(op):
		names := map[byte]string{
			OpPushByte: "push.b", OpSymbol: "sym.x", OpQuotation: "quot.x",
			OpLocal: "local", OpSetLocal: "local!", OpJump: "jmp",
			OpJumpBack: "jmp-", OpJumpZ: "jz", OpJumpNZ: "jnz",
			OpCall: "call", OpRing0R: "r0@", OpRing1R: "r1@",
			OpRing1W: "r1!", OpInspect: "inspect", OpGas: "gas",
			OpPickN: "pick.n", OpRollN: "roll.n", OpLoopN: "loop.n",
			OpString: "str",
		}
		if n, ok := names[op]; ok {
			return n
		}
		return "2op"
	case Is3ByteOp(op):
		return "3op"
	case IsVarLenOp(op):
		return "var"
	case op == OpHalt:
		return "halt"
	case op == OpYield:
		return "yield"
	case op == OpEnd:
		return "end"
	default:
		return "?"
	}
}
