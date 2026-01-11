package micro

import (
	"fmt"
	"strconv"
	"strings"
)

// Assembler converts text assembly to bytecode
type Assembler struct {
	code       []byte
	quotations map[string]int
	nextQuot   int
	labels     map[string]int
	fixups     []fixup
}

type fixup struct {
	pos   int
	label string
	size  int // 1 or 2 bytes
}

// NewAssembler creates a new assembler
func NewAssembler() *Assembler {
	return &Assembler{
		code:       make([]byte, 0, 256),
		quotations: make(map[string]int),
		nextQuot:   0,
		labels:     make(map[string]int),
	}
}

// mnemonics maps text to opcodes
var mnemonics = map[string]byte{
	// 1-byte commands
	"nop":    OpNop,
	"dup":    OpDup,
	"drop":   OpDrop,
	"swap":   OpSwap,
	"over":   OpOver,
	"rot":    OpRot,
	"+":      OpAdd,
	"add":    OpAdd,
	"-":      OpSub,
	"sub":    OpSub,
	"*":      OpMul,
	"mul":    OpMul,
	"/":      OpDiv,
	"div":    OpDiv,
	"mod":    OpMod,
	"=":      OpEq,
	"eq":     OpEq,
	"<":      OpLt,
	"lt":     OpLt,
	">":      OpGt,
	"gt":     OpGt,
	"and":    OpAnd,
	"or":     OpOr,
	"not":    OpNot,
	"neg":    OpNeg,
	"exec":   OpExec,
	"i":      OpExec,
	"ifte":   OpIfte,
	"dip":    OpDip,
	"loop":   OpLoop,
	"times":  OpLoop,
	"ret":    OpRet,
	"load":   OpLoad,
	"@":      OpLoad,
	"store":  OpStore,
	"!":      OpStore,
	"print":  OpPrint,
	".":      OpPrint,
	"inc":    OpInc,
	"1+":     OpInc,
	"dec":    OpDec,
	"1-":     OpDec,
	"dup2":   OpDup2,
	"2dup":   OpDup2,
	"depth":  OpDepth,
	"clear":  OpClear,

	// Special
	"halt":   OpHalt,
	"yield":  OpYield,
	"break":  OpBreak,
	"end":    OpEnd,
	"error":  OpError,
	"clrerr": OpClearE,
	"err?":   OpCheckE,
}

// symbols maps names to inline symbol opcodes
var symbols = map[string]byte{
	"nil":     SymNil,
	"true":    SymTrue,
	"false":   SymFalse,
	"self":    SymSelf,
	"target":  SymTarget,
	"health":  SymHealth,
	"energy":  SymEnergy,
	"pos":     SymPos,
	"anger":   SymAnger,
	"fear":    SymFear,
	"trust":   SymTrust,
	"hunger":  SymHunger,
	"enemy":   SymEnemy,
	"friend":  SymFriend,
	"food":    SymFood,
	"danger":  SymDanger,
	"safe":    SymSafe,
	"near":    SymNear,
	"far":     SymFar,
	"day":     SymDay,
	"night":   SymNight,
	"result":  SymResult,
	"count":   SymCount,
	"temp":    SymTemp,
	"x":       SymX,
	"y":       SymY,
}

// Assemble converts assembly text to bytecode
func (a *Assembler) Assemble(source string) ([]byte, error) {
	a.code = a.code[:0]
	a.labels = make(map[string]int)
	a.fixups = nil

	lines := strings.Split(source, "\n")

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, ";") || strings.HasPrefix(line, "%") {
			continue
		}

		// Remove inline comments
		if idx := strings.Index(line, ";"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}
		if idx := strings.Index(line, "%"); idx >= 0 {
			line = strings.TrimSpace(line[:idx])
		}

		if line == "" {
			continue
		}

		// Check for label definition
		if strings.HasSuffix(line, ":") {
			label := strings.TrimSuffix(line, ":")
			a.labels[label] = len(a.code)
			continue
		}

		// Tokenize
		tokens := tokenize(line)
		if len(tokens) == 0 {
			continue
		}

		if err := a.assembleTokens(tokens, lineNum+1); err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNum+1, err)
		}
	}

	// Apply fixups
	for _, f := range a.fixups {
		addr, ok := a.labels[f.label]
		if !ok {
			return nil, fmt.Errorf("undefined label: %s", f.label)
		}
		offset := addr - f.pos - f.size
		if f.size == 1 {
			a.code[f.pos] = byte(offset)
		} else {
			a.code[f.pos] = byte(offset >> 8)
			a.code[f.pos+1] = byte(offset & 0xFF)
		}
	}

	return a.code, nil
}

func tokenize(line string) []string {
	var tokens []string
	var current strings.Builder
	inString := false

	for _, r := range line {
		if inString {
			current.WriteRune(r)
			if r == '"' {
				tokens = append(tokens, current.String())
				current.Reset()
				inString = false
			}
		} else if r == '"' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			current.WriteRune(r)
			inString = true
		} else if r == ' ' || r == '\t' || r == ',' {
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
		} else {
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}
	return tokens
}

func (a *Assembler) assembleTokens(tokens []string, lineNum int) error {
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		tok = strings.ToLower(tok)

		// Check for mnemonic
		if op, ok := mnemonics[tok]; ok {
			a.code = append(a.code, op)
			continue
		}

		// Check for symbol
		if strings.HasPrefix(tok, "'") {
			name := tok[1:]
			if sym, ok := symbols[name]; ok {
				a.code = append(a.code, sym)
			} else {
				// Extended symbol
				a.code = append(a.code, OpSymbol, 0) // TODO: symbol table lookup
			}
			continue
		}

		// Check for inline symbol by name
		if sym, ok := symbols[tok]; ok {
			a.code = append(a.code, sym)
			continue
		}

		// Check for quotation reference [n] or [name]
		if strings.HasPrefix(tok, "[") && strings.HasSuffix(tok, "]") {
			inner := tok[1 : len(tok)-1]
			if n, err := strconv.Atoi(inner); err == nil {
				// Numeric quotation index
				if n < 32 {
					a.code = append(a.code, InlineQuotOp(n))
				} else {
					a.code = append(a.code, OpQuotation, byte(n))
				}
			} else {
				// Named quotation
				if idx, ok := a.quotations[inner]; ok {
					if idx < 32 {
						a.code = append(a.code, InlineQuotOp(idx))
					} else {
						a.code = append(a.code, OpQuotation, byte(idx))
					}
				} else {
					// Create new quotation slot
					a.quotations[inner] = a.nextQuot
					if a.nextQuot < 32 {
						a.code = append(a.code, InlineQuotOp(a.nextQuot))
					} else {
						a.code = append(a.code, OpQuotation, byte(a.nextQuot))
					}
					a.nextQuot++
				}
			}
			continue
		}

		// Check for number
		if n, err := strconv.ParseInt(tok, 0, 16); err == nil {
			a.emitNumber(int(n))
			continue
		}

		// Check for push.b, push.w instructions
		if tok == "push.b" || tok == "pushb" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("push.b requires argument")
			}
			i++
			n, err := strconv.ParseInt(tokens[i], 0, 16)
			if err != nil {
				return fmt.Errorf("invalid number: %s", tokens[i])
			}
			a.code = append(a.code, OpPushByte, byte(n))
			continue
		}

		if tok == "push.w" || tok == "pushw" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("push.w requires argument")
			}
			i++
			n, err := strconv.ParseInt(tokens[i], 0, 16)
			if err != nil {
				return fmt.Errorf("invalid number: %s", tokens[i])
			}
			a.code = append(a.code, OpPushWord, byte(n>>8), byte(n&0xFF))
			continue
		}

		// Jump instructions
		if tok == "jmp" || tok == "jump" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("jmp requires target")
			}
			i++
			target := tokens[i]
			if n, err := strconv.Atoi(target); err == nil {
				// Numeric offset
				if n >= 0 {
					a.code = append(a.code, OpJump, byte(n))
				} else {
					a.code = append(a.code, OpJumpBack, byte(-n))
				}
			} else {
				// Label - add fixup
				a.code = append(a.code, OpJump, 0)
				a.fixups = append(a.fixups, fixup{len(a.code) - 1, target, 1})
			}
			continue
		}

		if tok == "jz" || tok == "jumpz" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("jz requires target")
			}
			i++
			target := tokens[i]
			if n, err := strconv.Atoi(target); err == nil {
				a.code = append(a.code, OpJumpZ, byte(n))
			} else {
				a.code = append(a.code, OpJumpZ, 0)
				a.fixups = append(a.fixups, fixup{len(a.code) - 1, target, 1})
			}
			continue
		}

		if tok == "jnz" || tok == "jumpnz" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("jnz requires target")
			}
			i++
			target := tokens[i]
			if n, err := strconv.Atoi(target); err == nil {
				a.code = append(a.code, OpJumpNZ, byte(n))
			} else {
				a.code = append(a.code, OpJumpNZ, 0)
				a.fixups = append(a.fixups, fixup{len(a.code) - 1, target, 1})
			}
			continue
		}

		// Local variable access
		if tok == "local" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("local requires slot number")
			}
			i++
			n, err := strconv.Atoi(tokens[i])
			if err != nil || n < 0 || n > 15 {
				return fmt.Errorf("invalid local slot: %s", tokens[i])
			}
			a.code = append(a.code, OpLocal, byte(n))
			continue
		}

		if tok == "local!" || tok == "setlocal" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("local! requires slot number")
			}
			i++
			n, err := strconv.Atoi(tokens[i])
			if err != nil || n < 0 || n > 15 {
				return fmt.Errorf("invalid local slot: %s", tokens[i])
			}
			a.code = append(a.code, OpSetLocal, byte(n))
			continue
		}

		// Call builtin
		if tok == "call" {
			if i+1 >= len(tokens) {
				return fmt.Errorf("call requires builtin number")
			}
			i++
			n, err := strconv.Atoi(tokens[i])
			if err != nil {
				return fmt.Errorf("invalid builtin: %s", tokens[i])
			}
			a.code = append(a.code, OpCall, byte(n))
			continue
		}

		// String literal
		if strings.HasPrefix(tok, "\"") && strings.HasSuffix(tok, "\"") {
			str := tok[1 : len(tok)-1]
			a.code = append(a.code, OpStringVar, byte(len(str)))
			a.code = append(a.code, []byte(str)...)
			continue
		}

		return fmt.Errorf("unknown token: %s", tok)
	}

	return nil
}

func (a *Assembler) emitNumber(n int) {
	if n >= 0 && n <= 31 {
		a.code = append(a.code, SmallNumOp(n))
	} else if n >= 0 && n <= 255 {
		a.code = append(a.code, OpPushByte, byte(n))
	} else {
		a.code = append(a.code, OpPushWord, byte(n>>8), byte(n&0xFF))
	}
}

// GetQuotations returns the quotation name to index mapping
func (a *Assembler) GetQuotations() map[string]int {
	return a.quotations
}

// Disassemble converts bytecode back to text
func Disassemble(code []byte) string {
	var sb strings.Builder
	pc := 0

	for pc < len(code) {
		op := code[pc]
		sb.WriteString(fmt.Sprintf("%04X: ", pc))

		switch {
		case op <= 0x1F:
			sb.WriteString(OpName(op))
			pc++

		case IsSmallNum(op):
			sb.WriteString(fmt.Sprintf("%d", SmallNumValue(op)))
			pc++

		case IsInlineSym(op):
			// Find symbol name
			found := false
			for name, sym := range symbols {
				if sym == op {
					sb.WriteString("'" + name)
					found = true
					break
				}
			}
			if !found {
				sb.WriteString(fmt.Sprintf("sym.%02X", op-0x40))
			}
			pc++

		case IsInlineQuot(op):
			sb.WriteString(fmt.Sprintf("[%d]", InlineQuotIndex(op)))
			pc++

		case Is2ByteOp(op):
			if pc+1 >= len(code) {
				sb.WriteString("?? (truncated)")
				pc++
				break
			}
			arg := code[pc+1]
			switch op {
			case OpPushByte:
				sb.WriteString(fmt.Sprintf("push.b %d", arg))
			case OpSymbol:
				sb.WriteString(fmt.Sprintf("sym.x %d", arg))
			case OpQuotation:
				sb.WriteString(fmt.Sprintf("[%d]", arg))
			case OpJump:
				sb.WriteString(fmt.Sprintf("jmp +%d", arg))
			case OpJumpBack:
				sb.WriteString(fmt.Sprintf("jmp -%d", arg))
			case OpJumpZ:
				sb.WriteString(fmt.Sprintf("jz +%d", arg))
			case OpJumpNZ:
				sb.WriteString(fmt.Sprintf("jnz +%d", arg))
			case OpLocal:
				sb.WriteString(fmt.Sprintf("local %d", arg))
			case OpSetLocal:
				sb.WriteString(fmt.Sprintf("local! %d", arg))
			case OpCall:
				sb.WriteString(fmt.Sprintf("call %d", arg))
			default:
				sb.WriteString(fmt.Sprintf("%s %d", OpName(op), arg))
			}
			pc += 2

		case Is3ByteOp(op):
			if pc+2 >= len(code) {
				sb.WriteString("?? (truncated)")
				pc++
				break
			}
			hi := code[pc+1]
			lo := code[pc+2]
			val := int16(lo) | (int16(hi) << 8)
			switch op {
			case OpPushWord:
				sb.WriteString(fmt.Sprintf("push.w %d", val))
			default:
				sb.WriteString(fmt.Sprintf("3op.%02X %d", op, val))
			}
			pc += 3

		case IsVarLenOp(op):
			if pc+1 >= len(code) {
				sb.WriteString("?? (truncated)")
				pc++
				break
			}
			length := int(code[pc+1])
			if pc+2+length > len(code) {
				sb.WriteString("?? (truncated)")
				pc++
				break
			}
			data := code[pc+2 : pc+2+length]
			switch op {
			case OpStringVar:
				sb.WriteString(fmt.Sprintf("\"%s\"", string(data)))
			default:
				sb.WriteString(fmt.Sprintf("var.%02X [%d bytes]", op, length))
			}
			pc += 2 + length

		case op == OpHalt:
			sb.WriteString("halt")
			pc++
		case op == OpEnd:
			sb.WriteString("end")
			pc++
		default:
			sb.WriteString(fmt.Sprintf("?%02X", op))
			pc++
		}

		sb.WriteString("\n")
	}

	return sb.String()
}

// AssembleQuotation assembles a quotation body
func (a *Assembler) AssembleQuotation(source string) ([]byte, error) {
	return a.Assemble(source)
}
