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
