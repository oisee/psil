package sandbox

import (
	"math/rand"

	"github.com/psilLang/psil/pkg/micro"
)

// TokenType classifies opcodes into functional categories for WFC.
type TokenType byte

const (
	TokSense  TokenType = iota // r0@ N — read sensor (2 bytes)
	TokPush                    // small num 0x20-0x3F, or push.b (1-2 bytes)
	TokCmp                     // =, <, >, not (1 byte)
	TokBranch                  // jnz, jz, jmp (2 bytes)
	TokMove                    // r1! 0 — write move dir (2 bytes)
	TokAction                  // r1! 1 — write action (2 bytes)
	TokTarget                  // r1! 2 — write target (2 bytes)
	TokStack                   // dup, drop, swap, over (1 byte)
	TokMath                    // +, -, *, and, or (1 byte)
	TokYield                   // yield or halt (1 byte)
	NumTokenTypes              // 10
)

// ClassifyOpcode returns the token type for the instruction at code[pc].
func ClassifyOpcode(op byte, code []byte, pc int) TokenType {
	switch {
	// Ring0 read → sensor
	case op == micro.OpRing0R:
		return TokSense

	// Ring1 write → check operand for move/action/target
	case op == micro.OpRing1W:
		if pc+1 < len(code) {
			switch code[pc+1] {
			case 0x00:
				return TokMove
			case 0x01:
				return TokAction
			case 0x02:
				return TokTarget
			}
		}
		return TokAction // default ring1 write → action

	// Branch instructions
	case op == micro.OpJumpNZ || op == micro.OpJumpZ || op == micro.OpJump:
		return TokBranch

	// Comparison ops
	case op == micro.OpEq || op == micro.OpLt || op == micro.OpGt || op == micro.OpNot:
		return TokCmp

	// Stack manipulation
	case op == micro.OpDup || op == micro.OpDrop || op == micro.OpSwap ||
		op == micro.OpOver || op == micro.OpRot || op == micro.OpNop:
		return TokStack

	// Math
	case op == micro.OpAdd || op == micro.OpSub || op == micro.OpMul ||
		op == micro.OpDiv || op == micro.OpMod || op == micro.OpAnd ||
		op == micro.OpOr || op == micro.OpInc || op == micro.OpDec ||
		op == micro.OpNeg:
		return TokMath

	// Action opcodes → TokAction (they auto-yield, so they act as action+yield)
	case op >= micro.OpActMove && op <= micro.OpActCraft:
		return TokAction

	// Yield / halt
	case op == micro.OpYield || op == micro.OpHalt:
		return TokYield

	// Small numbers and push.b → push
	case micro.IsSmallNum(op):
		return TokPush
	case op == micro.OpPushByte:
		return TokPush

	// Everything else (symbols, quotations, etc.) → push
	default:
		return TokPush
	}
}

// TokenizeGenome converts a bytecode genome into a token type sequence.
func TokenizeGenome(genome []byte) []TokenType {
	var tokens []TokenType
	pc := 0
	for pc < len(genome) {
		op := genome[pc]
		tok := ClassifyOpcode(op, genome, pc)
		tokens = append(tokens, tok)
		size := opcodeSize(op, genome, pc)
		pc += size
	}
	return tokens
}

// WFC1DCell represents one position in the 1D WFC grid.
type WFC1DCell struct {
	Possibilities uint16 // bitmask of possible token types (10 bits)
	CollapsedTo   int8   // -1 = uncollapsed
}

// WFC1D is a simplified 1D Wave Function Collapse engine for token sequences.
type WFC1D struct {
	Length      int
	Cells       []WFC1DCell
	Constraints [NumTokenTypes]uint16 // [tokenA] → bitmask of allowed next tokens
	Rng         *rand.Rand
}

// allTokensMask returns a bitmask with all token type bits set.
func allTokensMask() uint16 {
	return (1 << uint(NumTokenTypes)) - 1
}

// NewWFC1D creates a new 1D WFC with the given length and constraints.
func NewWFC1D(length int, constraints [NumTokenTypes]uint16, rng *rand.Rand) *WFC1D {
	cells := make([]WFC1DCell, length)
	all := allTokensMask()
	for i := range cells {
		cells[i] = WFC1DCell{
			Possibilities: all,
			CollapsedTo:   -1,
		}
	}
	return &WFC1D{
		Length:      length,
		Cells:       cells,
		Constraints: constraints,
		Rng:         rng,
	}
}

// Collapse forces a cell to a specific token type.
func (w *WFC1D) Collapse(pos int, tok TokenType) bool {
	if pos < 0 || pos >= w.Length {
		return false
	}
	bit := uint16(1) << uint(tok)
	if w.Cells[pos].Possibilities&bit == 0 {
		return false // contradiction
	}
	w.Cells[pos].Possibilities = bit
	w.Cells[pos].CollapsedTo = int8(tok)
	return true
}

// CollapseRandom collapses a cell to a random token from its possibilities.
func (w *WFC1D) CollapseRandom(pos int) bool {
	if pos < 0 || pos >= w.Length {
		return false
	}
	cell := &w.Cells[pos]
	if cell.CollapsedTo >= 0 {
		return true // already collapsed
	}

	// Count possibilities
	var options []TokenType
	for t := TokenType(0); t < NumTokenTypes; t++ {
		if cell.Possibilities&(1<<uint(t)) != 0 {
			options = append(options, t)
		}
	}
	if len(options) == 0 {
		return false // contradiction
	}

	chosen := options[w.Rng.Intn(len(options))]
	cell.Possibilities = 1 << uint(chosen)
	cell.CollapsedTo = int8(chosen)
	return true
}

// Propagate applies forward constraints from collapsed cells.
func (w *WFC1D) Propagate() bool {
	for i := 0; i < w.Length-1; i++ {
		if w.Cells[i].CollapsedTo < 0 {
			continue
		}
		tok := TokenType(w.Cells[i].CollapsedTo)
		allowed := w.Constraints[tok]
		if allowed == 0 {
			allowed = allTokensMask() // no constraints → allow all
		}
		w.Cells[i+1].Possibilities &= allowed
		if w.Cells[i+1].Possibilities == 0 {
			return false // contradiction
		}
	}
	return true
}

// Generate collapses the entire sequence left-to-right.
func (w *WFC1D) Generate() bool {
	for i := 0; i < w.Length; i++ {
		if !w.CollapseRandom(i) {
			return false
		}
		if !w.Propagate() {
			return false
		}
	}
	return true
}

// ToTokens extracts the collapsed token sequence.
func (w *WFC1D) ToTokens() []TokenType {
	tokens := make([]TokenType, w.Length)
	for i, cell := range w.Cells {
		if cell.CollapsedTo < 0 {
			tokens[i] = TokPush // fallback
		} else {
			tokens[i] = TokenType(cell.CollapsedTo)
		}
	}
	return tokens
}

// MineConstraints extracts bigram constraints from a set of genomes.
func MineConstraints(genomes [][]byte) [NumTokenTypes]uint16 {
	var counts [NumTokenTypes][NumTokenTypes]int
	for _, g := range genomes {
		tokens := TokenizeGenome(g)
		for i := 0; i < len(tokens)-1; i++ {
			a, b := tokens[i], tokens[i+1]
			if a < NumTokenTypes && b < NumTokenTypes {
				counts[a][b]++
			}
		}
	}

	var constraints [NumTokenTypes]uint16
	for a := TokenType(0); a < NumTokenTypes; a++ {
		for b := TokenType(0); b < NumTokenTypes; b++ {
			if counts[a][b] > 0 {
				constraints[a] |= 1 << uint(b)
			}
		}
	}
	return constraints
}

// BaseTokenConstraints extracts constraints from archetype genomes.
func BaseTokenConstraints(archetypes [][]byte) [NumTokenTypes]uint16 {
	return MineConstraints(archetypes)
}

// MergeConstraints unions two constraint sets.
func MergeConstraints(mined, base [NumTokenTypes]uint16) [NumTokenTypes]uint16 {
	var merged [NumTokenTypes]uint16
	for i := TokenType(0); i < NumTokenTypes; i++ {
		merged[i] = mined[i] | base[i]
	}
	return merged
}

// tokenByteSize returns the byte size of a rendered token.
func tokenByteSize(tok TokenType) int {
	switch tok {
	case TokSense:
		return 2
	case TokPush:
		return 1 // small number (1 byte)
	case TokCmp:
		return 1
	case TokBranch:
		return 2
	case TokMove:
		return 2
	case TokAction:
		return 2
	case TokTarget:
		return 2
	case TokStack:
		return 1
	case TokMath:
		return 1
	case TokYield:
		return 1
	default:
		return 1
	}
}

// Useful sensor slots for WFC-generated genomes.
var usefulSensors = []byte{
	Ring0Health,   // 1
	Ring0Energy,   // 2
	Ring0Hunger,   // 3
	Ring0Food,     // 5
	Ring0Near,     // 7
	Ring0Day,      // 10
	Ring0NearID,   // 12
	Ring0FoodDir,  // 13
	Ring0MyGold,   // 14
	Ring0MyItem,   // 15
	Ring0NearItem, // 16
	Ring0NearDir,  // 18
	Ring0ItemDir,  // 19
	Ring0Stress,   // 21
	Ring0OnForge,    // 23
	Ring0MyAge,      // 24
	Ring0Biome,      // 26
	Ring0TileType,   // 27
	Ring0Similarity, // 28
	Ring0TileAhead,  // 29
	Ring0Cooldown,   // 30
}

var cmpOps = []byte{micro.OpEq, micro.OpLt, micro.OpGt, micro.OpNot}
var stackOps = []byte{micro.OpDup, micro.OpDrop, micro.OpSwap, micro.OpOver, micro.OpRot}
var mathOps = []byte{micro.OpAdd, micro.OpSub, micro.OpMul, micro.OpAnd, micro.OpOr, micro.OpInc, micro.OpDec}

// --- 8-Type Token Classification (Z80-compatible, byte bitmask) ---

// TokenType8 is a reduced 8-type classification fitting in a single byte bitmask.
// Merges: TokStack+TokMath→Tok8Ops, TokTarget→Tok8Action.
type TokenType8 byte

const (
	Tok8Sense  TokenType8 = 0 // r0@ N — read sensor (2 bytes)
	Tok8Push   TokenType8 = 1 // small num, push.b (1-2 bytes)
	Tok8Cmp    TokenType8 = 2 // =, <, >, not (1 byte)
	Tok8Branch TokenType8 = 3 // jnz, jz (2 bytes)
	Tok8Move   TokenType8 = 4 // act.move or r1! 0 (2 bytes)
	Tok8Action TokenType8 = 5 // act.eat..act.craft, r1! 1/2 (2 bytes)
	Tok8Ops    TokenType8 = 6 // dup/drop/swap/+/-/*/and/or (1 byte)
	Tok8Yield  TokenType8 = 7 // yield, halt (1 byte)
	Num8Types  TokenType8 = 8
)

var tok8Names = [8]string{
	"Sense", "Push", "Cmp", "Branch", "Move", "Action", "Ops", "Yield",
}

// ClassifyOpcode8 maps a bytecode opcode to the 8-type scheme.
func ClassifyOpcode8(op byte, code []byte, pc int) TokenType8 {
	tok10 := ClassifyOpcode(op, code, pc)
	switch tok10 {
	case TokSense:
		return Tok8Sense
	case TokPush:
		return Tok8Push
	case TokCmp:
		return Tok8Cmp
	case TokBranch:
		return Tok8Branch
	case TokMove:
		return Tok8Move
	case TokAction, TokTarget:
		return Tok8Action
	case TokStack, TokMath:
		return Tok8Ops
	case TokYield:
		return Tok8Yield
	default:
		return Tok8Push
	}
}

// TokenizeGenome8 converts a bytecode genome into an 8-type token sequence.
func TokenizeGenome8(genome []byte) []TokenType8 {
	var tokens []TokenType8
	pc := 0
	for pc < len(genome) {
		op := genome[pc]
		tok := ClassifyOpcode8(op, genome, pc)
		tokens = append(tokens, tok)
		size := opcodeSize(op, genome, pc)
		pc += size
	}
	return tokens
}

// MineConstraints8 extracts bigram constraints from genomes using 8-type scheme.
func MineConstraints8(genomes [][]byte) [8]byte {
	var counts [8][8]int
	for _, g := range genomes {
		tokens := TokenizeGenome8(g)
		for i := 0; i < len(tokens)-1; i++ {
			a, b := tokens[i], tokens[i+1]
			if a < Num8Types && b < Num8Types {
				counts[a][b]++
			}
		}
	}
	var constraints [8]byte
	for a := TokenType8(0); a < Num8Types; a++ {
		for b := TokenType8(0); b < Num8Types; b++ {
			if counts[a][b] > 0 {
				constraints[a] |= 1 << uint(b)
			}
		}
	}
	return constraints
}

// BaseConstraints8 returns archetype-derived constraints for the 8-type scheme.
// Mined from handcrafted archetype genomes (trader, forager, crafter, teacher)
// merged with structural patterns from evolved-brain analysis.
func BaseConstraints8() [8]byte {
	return [8]byte{
		// Sense(0):  Push, Cmp, Move, Action, Ops
		(1 << Tok8Push) | (1 << Tok8Cmp) | (1 << Tok8Move) | (1 << Tok8Action) | (1 << Tok8Ops),
		// Push(1):   Sense, Cmp, Action, Ops
		(1 << Tok8Sense) | (1 << Tok8Cmp) | (1 << Tok8Action) | (1 << Tok8Ops),
		// Cmp(2):    Branch, Ops
		(1 << Tok8Branch) | (1 << Tok8Ops),
		// Branch(3): Sense, Move, Action
		(1 << Tok8Sense) | (1 << Tok8Move) | (1 << Tok8Action),
		// Move(4):   Sense, Push, Action, Yield
		(1 << Tok8Sense) | (1 << Tok8Push) | (1 << Tok8Action) | (1 << Tok8Yield),
		// Action(5): Sense, Move, Yield
		(1 << Tok8Sense) | (1 << Tok8Move) | (1 << Tok8Yield),
		// Ops(6):    Push, Cmp, Ops
		(1 << Tok8Push) | (1 << Tok8Cmp) | (1 << Tok8Ops),
		// Yield(7):  Sense, Push
		(1 << Tok8Sense) | (1 << Tok8Push),
	}
}

// MergeConstraints8 unions two 8-type constraint sets.
func MergeConstraints8(a, b [8]byte) [8]byte {
	var merged [8]byte
	for i := 0; i < 8; i++ {
		merged[i] = a[i] | b[i]
	}
	return merged
}

// --- 8-Type WFC1D Engine ---

// WFC1D8Cell represents one position in the 8-type 1D WFC grid.
type WFC1D8Cell struct {
	Possibilities byte // bitmask of possible token types (8 bits)
	CollapsedTo   int8 // -1 = uncollapsed
}

// WFC1D8 is a 1D Wave Function Collapse engine using 8-type byte bitmasks.
type WFC1D8 struct {
	Length      int
	Cells       []WFC1D8Cell
	Constraints [8]byte
	Rng         *rand.Rand
}

// NewWFC1D8 creates a new 8-type 1D WFC engine.
func NewWFC1D8(length int, constraints [8]byte, rng *rand.Rand) *WFC1D8 {
	cells := make([]WFC1D8Cell, length)
	for i := range cells {
		cells[i] = WFC1D8Cell{
			Possibilities: 0xFF, // all 8 types
			CollapsedTo:   -1,
		}
	}
	return &WFC1D8{
		Length:      length,
		Cells:       cells,
		Constraints: constraints,
		Rng:         rng,
	}
}

// Collapse8 forces a cell to a specific token type.
func (w *WFC1D8) Collapse8(pos int, tok TokenType8) bool {
	if pos < 0 || pos >= w.Length {
		return false
	}
	bit := byte(1) << uint(tok)
	if w.Cells[pos].Possibilities&bit == 0 {
		return false
	}
	w.Cells[pos].Possibilities = bit
	w.Cells[pos].CollapsedTo = int8(tok)
	return true
}

// CollapseRandom8 collapses a cell to a random token from its possibilities.
func (w *WFC1D8) CollapseRandom8(pos int) bool {
	if pos < 0 || pos >= w.Length {
		return false
	}
	cell := &w.Cells[pos]
	if cell.CollapsedTo >= 0 {
		return true
	}
	var options []TokenType8
	for t := TokenType8(0); t < Num8Types; t++ {
		if cell.Possibilities&(1<<uint(t)) != 0 {
			options = append(options, t)
		}
	}
	if len(options) == 0 {
		return false
	}
	chosen := options[w.Rng.Intn(len(options))]
	cell.Possibilities = 1 << uint(chosen)
	cell.CollapsedTo = int8(chosen)
	return true
}

// Propagate8 applies forward constraints from collapsed cells.
func (w *WFC1D8) Propagate8() bool {
	for i := 0; i < w.Length-1; i++ {
		if w.Cells[i].CollapsedTo < 0 {
			continue
		}
		tok := TokenType8(w.Cells[i].CollapsedTo)
		allowed := w.Constraints[tok]
		if allowed == 0 {
			allowed = 0xFF
		}
		w.Cells[i+1].Possibilities &= allowed
		if w.Cells[i+1].Possibilities == 0 {
			return false
		}
	}
	return true
}

// getPredecessors8 returns which types can precede any of the given target possibilities.
func (w *WFC1D8) getPredecessors8(target byte) byte {
	var result byte
	for t := TokenType8(0); t < Num8Types; t++ {
		if w.Constraints[t]&target != 0 {
			result |= 1 << uint(t)
		}
	}
	return result
}

// PropagateBack8 applies backward constraints from anchored/collapsed cells.
func (w *WFC1D8) PropagateBack8() bool {
	for i := w.Length - 1; i > 0; i-- {
		preds := w.getPredecessors8(w.Cells[i].Possibilities)
		if preds == 0 {
			preds = 0xFF
		}
		w.Cells[i-1].Possibilities &= preds
		if w.Cells[i-1].Possibilities == 0 {
			return false
		}
	}
	return true
}

// Generate8 collapses the entire sequence left-to-right.
// Runs backward propagation first to ensure anchors are reachable.
func (w *WFC1D8) Generate8() bool {
	// Backward pass: constrain cells so end anchors are reachable
	if !w.PropagateBack8() {
		return false
	}
	// Forward pass: constrain cells from start anchors
	if !w.Propagate8() {
		return false
	}
	// Collapse left-to-right
	for i := 0; i < w.Length; i++ {
		if !w.CollapseRandom8(i) {
			return false
		}
		if !w.Propagate8() {
			return false
		}
	}
	return true
}

// ToTokens8 extracts the collapsed 8-type token sequence.
func (w *WFC1D8) ToTokens8() []TokenType8 {
	tokens := make([]TokenType8, w.Length)
	for i, cell := range w.Cells {
		if cell.CollapsedTo < 0 {
			tokens[i] = Tok8Push
		} else {
			tokens[i] = TokenType8(cell.CollapsedTo)
		}
	}
	return tokens
}

// tokenByteSize8 returns the rendered byte size for an 8-type token.
func tokenByteSize8(tok TokenType8) int {
	switch tok {
	case Tok8Sense:
		return 2
	case Tok8Push:
		return 1
	case Tok8Cmp:
		return 1
	case Tok8Branch:
		return 2
	case Tok8Move:
		return 2
	case Tok8Action:
		return 2
	case Tok8Ops:
		return 1
	case Tok8Yield:
		return 1
	default:
		return 1
	}
}

var opsOps = []byte{
	micro.OpDup, micro.OpDrop, micro.OpSwap, micro.OpOver, micro.OpRot,
	micro.OpAdd, micro.OpSub, micro.OpMul, micro.OpAnd, micro.OpOr,
	micro.OpInc, micro.OpDec,
}

// RenderTokens8 converts an 8-type token sequence into concrete bytecode.
func RenderTokens8(tokens []TokenType8, rng *rand.Rand) []byte {
	// Pass 1: compute byte positions
	positions := make([]int, len(tokens))
	pos := 0
	for i, tok := range tokens {
		positions[i] = pos
		pos += tokenByteSize8(tok)
	}

	// Pass 2: emit bytes
	out := make([]byte, 0, pos)
	for i, tok := range tokens {
		switch tok {
		case Tok8Sense:
			out = append(out, micro.OpRing0R, usefulSensors[rng.Intn(len(usefulSensors))])
		case Tok8Push:
			out = append(out, byte(0x20+rng.Intn(10)))
		case Tok8Cmp:
			out = append(out, cmpOps[rng.Intn(len(cmpOps))])
		case Tok8Branch:
			offset := byte(4)
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j] == Tok8Yield || tokens[j] == Tok8Action || tokens[j] == Tok8Move {
					byteOffset := positions[j] - (positions[i] + 2)
					if byteOffset > 0 && byteOffset < 128 {
						offset = byte(byteOffset)
					}
					break
				}
			}
			if rng.Intn(4) == 0 {
				out = append(out, micro.OpJumpZ, offset)
			} else {
				out = append(out, micro.OpJumpNZ, offset)
			}
		case Tok8Move:
			if rng.Intn(2) == 0 {
				args := []byte{1, 2, 3, 4, 5, 6, 7}
				out = append(out, micro.OpActMove, args[rng.Intn(len(args))])
			} else {
				out = append(out, micro.OpRing1W, 0x00)
			}
		case Tok8Action:
			if rng.Intn(10) < 7 {
				actOps := []byte{
					micro.OpActAttack, micro.OpActHeal, micro.OpActEat,
					micro.OpActHarvest, micro.OpActTerraform,
					micro.OpActShare, micro.OpActTrade, micro.OpActCraft,
				}
				out = append(out, actOps[rng.Intn(len(actOps))], 0x00)
			} else {
				slot := byte(1)
				if rng.Intn(3) == 0 {
					slot = 2 // target
				}
				out = append(out, micro.OpRing1W, slot)
			}
		case Tok8Ops:
			out = append(out, opsOps[rng.Intn(len(opsOps))])
		case Tok8Yield:
			out = append(out, micro.OpYield)
		}
	}
	return out
}

// RenderTokens converts a token sequence into concrete bytecode.
func RenderTokens(tokens []TokenType, rng *rand.Rand) []byte {
	// Pass 1: compute byte positions for each token
	positions := make([]int, len(tokens))
	pos := 0
	for i, tok := range tokens {
		positions[i] = pos
		pos += tokenByteSize(tok)
	}
	totalBytes := pos

	// Pass 2: emit bytes
	out := make([]byte, 0, totalBytes)
	for i, tok := range tokens {
		switch tok {
		case TokSense:
			out = append(out, micro.OpRing0R, usefulSensors[rng.Intn(len(usefulSensors))])

		case TokPush:
			out = append(out, byte(0x20+rng.Intn(10))) // push 0-9

		case TokCmp:
			out = append(out, cmpOps[rng.Intn(len(cmpOps))])

		case TokBranch:
			// Scan forward for next TokYield or TokAction (auto-yields) to compute offset
			offset := byte(4) // default small forward jump
			for j := i + 1; j < len(tokens); j++ {
				if tokens[j] == TokYield || tokens[j] == TokAction || tokens[j] == TokMove {
					byteOffset := positions[j] - (positions[i] + 2) // offset from after branch instr
					if byteOffset > 0 && byteOffset < 128 {
						offset = byte(byteOffset)
					}
					break
				}
			}
			// 75% jnz, 25% jz
			if rng.Intn(4) == 0 {
				out = append(out, micro.OpJumpZ, offset)
			} else {
				out = append(out, micro.OpJumpNZ, offset)
			}

		case TokMove:
			// 50% action opcode, 50% Ring1 write
			if rng.Intn(2) == 0 {
				args := []byte{1, 2, 3, 4, 5, 6, 7} // dirs + toward food/NPC/item
				out = append(out, micro.OpActMove, args[rng.Intn(len(args))])
			} else {
				out = append(out, micro.OpRing1W, 0x00)
			}

		case TokAction:
			// 70% action opcode (easier to evolve), 30% Ring1 write
			if rng.Intn(10) < 7 {
				actOps := []byte{
					micro.OpActAttack, micro.OpActHeal, micro.OpActEat,
					micro.OpActHarvest, micro.OpActTerraform,
					micro.OpActShare, micro.OpActTrade, micro.OpActCraft,
				}
				out = append(out, actOps[rng.Intn(len(actOps))], 0x00)
			} else {
				out = append(out, micro.OpRing1W, 0x01)
			}

		case TokTarget:
			out = append(out, micro.OpRing1W, 0x02)

		case TokStack:
			out = append(out, stackOps[rng.Intn(len(stackOps))])

		case TokMath:
			out = append(out, mathOps[rng.Intn(len(mathOps))])

		case TokYield:
			out = append(out, micro.OpYield)
		}
	}
	return out
}
