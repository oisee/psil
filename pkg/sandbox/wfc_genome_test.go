package sandbox

import (
	"math/rand"
	"testing"

	"github.com/psilLang/psil/pkg/micro"
)

// Trader genome for tests (copied from cmd/sandbox/main.go).
var testTraderGenome = []byte{
	0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x08,
	0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
	0x8A, 0x12, 0x8C, 0x00, 0x24, 0x8C, 0x01, 0x8A, 0x0C, 0x8C, 0x02, 0xF1,
}

var testForagerGenome = []byte{
	0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
}

var testCrafterGenome = []byte{
	0x8A, 0x17, 0x20, 0x0D, 0x88, 0x08,
	0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1, 0xFF,
	0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x04,
	0x8A, 0x0D, 0x8C, 0x00,
	0x25, 0x8C, 0x01, 0xF1,
}

var testTeacherGenome = []byte{
	0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x08,
	0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
	0x8A, 0x07, 0x22, 0x0C, 0x88, 0x08,
	0x8A, 0x12, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1,
	0x26, 0x8C, 0x01, 0x8A, 0x0C, 0x8C, 0x02, 0x8A, 0x0D, 0x8C, 0x00, 0xF1,
}

var testArchetypes = [][]byte{testTraderGenome, testForagerGenome, testCrafterGenome, testTeacherGenome}

func TestClassifyOpcode(t *testing.T) {
	tests := []struct {
		name string
		code []byte
		pc   int
		want TokenType
	}{
		{"ring0 read", []byte{micro.OpRing0R, 0x0D}, 0, TokSense},
		{"ring1 write move", []byte{micro.OpRing1W, 0x00}, 0, TokMove},
		{"ring1 write action", []byte{micro.OpRing1W, 0x01}, 0, TokAction},
		{"ring1 write target", []byte{micro.OpRing1W, 0x02}, 0, TokTarget},
		{"jnz", []byte{micro.OpJumpNZ, 0x08}, 0, TokBranch},
		{"jz", []byte{micro.OpJumpZ, 0x04}, 0, TokBranch},
		{"eq", []byte{micro.OpEq}, 0, TokCmp},
		{"lt", []byte{micro.OpLt}, 0, TokCmp},
		{"gt", []byte{micro.OpGt}, 0, TokCmp},
		{"not", []byte{micro.OpNot}, 0, TokCmp},
		{"dup", []byte{micro.OpDup}, 0, TokStack},
		{"add", []byte{micro.OpAdd}, 0, TokMath},
		{"sub", []byte{micro.OpSub}, 0, TokMath},
		{"yield", []byte{micro.OpYield}, 0, TokYield},
		{"halt", []byte{micro.OpHalt}, 0, TokYield},
		{"small num 0", []byte{0x20}, 0, TokPush},
		{"small num 5", []byte{0x25}, 0, TokPush},
		{"push.b", []byte{micro.OpPushByte, 0x42}, 0, TokPush},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyOpcode(tt.code[tt.pc], tt.code, tt.pc)
			if got != tt.want {
				t.Errorf("ClassifyOpcode(%02x) = %d, want %d", tt.code[tt.pc], got, tt.want)
			}
		})
	}
}

func TestTokenizeGenome(t *testing.T) {
	tokens := TokenizeGenome(testTraderGenome)
	if len(tokens) == 0 {
		t.Fatal("TokenizeGenome returned empty")
	}

	// Trader starts with sense (r0@ 15)
	if tokens[0] != TokSense {
		t.Errorf("first token = %d, want TokSense (%d)", tokens[0], TokSense)
	}

	// Should contain branches and yields
	hasBranch := false
	hasYield := false
	for _, tok := range tokens {
		if tok == TokBranch {
			hasBranch = true
		}
		if tok == TokYield {
			hasYield = true
		}
	}
	if !hasBranch {
		t.Error("trader genome should contain TokBranch")
	}
	if !hasYield {
		t.Error("trader genome should contain TokYield")
	}
}

func TestBaseConstraints(t *testing.T) {
	constraints := BaseTokenConstraints(testArchetypes)
	nonEmpty := 0
	for i := TokenType(0); i < NumTokenTypes; i++ {
		if constraints[i] != 0 {
			nonEmpty++
		}
	}
	if nonEmpty == 0 {
		t.Error("BaseTokenConstraints returned all-zero constraints")
	}
	// Sense should be able to be followed by something (push, move, etc.)
	if constraints[TokSense] == 0 {
		t.Error("TokSense has no allowed followers")
	}
}

func TestMineConstraints(t *testing.T) {
	constraints := MineConstraints(testArchetypes)

	// Sense → Push should be present (r0@ N, then push for comparison)
	if constraints[TokSense]&(1<<uint(TokPush)) == 0 {
		t.Error("expected Sense→Push bigram in mined constraints")
	}
	// Push → Cmp should be present (push 0, >)
	if constraints[TokPush]&(1<<uint(TokCmp)) == 0 {
		t.Error("expected Push→Cmp bigram in mined constraints")
	}
}

func TestMergeConstraints(t *testing.T) {
	var a, b [NumTokenTypes]uint16
	a[TokSense] = 0x01
	b[TokSense] = 0x02
	a[TokPush] = 0x04

	merged := MergeConstraints(a, b)
	if merged[TokSense] != 0x03 {
		t.Errorf("merge TokSense: got %016b, want 0x03", merged[TokSense])
	}
	if merged[TokPush] != 0x04 {
		t.Errorf("merge TokPush: got %016b, want 0x04", merged[TokPush])
	}
}

func TestWFC1DGenerate(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	constraints := BaseTokenConstraints(testArchetypes)

	wfc := NewWFC1D(10, constraints, rng)
	wfc.Collapse(0, TokSense)
	wfc.Collapse(9, TokYield)

	if !wfc.Generate() {
		t.Fatal("WFC1D.Generate() failed")
	}

	tokens := wfc.ToTokens()
	if len(tokens) != 10 {
		t.Fatalf("expected 10 tokens, got %d", len(tokens))
	}
	if tokens[0] != TokSense {
		t.Errorf("first token = %d, want TokSense", tokens[0])
	}
	if tokens[9] != TokYield {
		t.Errorf("last token = %d, want TokYield", tokens[9])
	}

	// All tokens should be valid types
	for i, tok := range tokens {
		if tok >= NumTokenTypes {
			t.Errorf("token[%d] = %d, exceeds NumTokenTypes", i, tok)
		}
	}
}

func TestRenderTokens(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	tokens := []TokenType{TokSense, TokPush, TokCmp, TokBranch, TokMove, TokAction, TokYield}

	bytecode := RenderTokens(tokens, rng)
	if len(bytecode) == 0 {
		t.Fatal("RenderTokens returned empty")
	}

	// Verify it parses correctly
	points := OpcodeAlignedPoints(bytecode)
	if len(points) < 2 {
		t.Errorf("OpcodeAlignedPoints returned %d points, expected >= 2", len(points))
	}

	// First instruction should be r0@ (sensor read)
	if bytecode[0] != micro.OpRing0R {
		t.Errorf("first byte = %02x, want %02x (OpRing0R)", bytecode[0], micro.OpRing0R)
	}

	// Last byte should be yield
	if bytecode[len(bytecode)-1] != micro.OpYield {
		t.Errorf("last byte = %02x, want %02x (OpYield)", bytecode[len(bytecode)-1], micro.OpYield)
	}
}

func TestWFCGenome(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ga := NewGA(rng)
	ga.WFCEnabled = true
	ga.Archetypes = testArchetypes

	// Mine constraints from archetypes
	ga.UpdateConstraints(testArchetypes)

	genome := ga.WFCGenome(24)
	if len(genome) < MinGenome {
		t.Errorf("genome length %d < MinGenome %d", len(genome), MinGenome)
	}
	if len(genome) > MaxGenome {
		t.Errorf("genome length %d > MaxGenome %d", len(genome), MaxGenome)
	}

	// Should parse correctly
	points := OpcodeAlignedPoints(genome)
	if len(points) < 2 {
		t.Error("WFC genome has < 2 opcode-aligned points")
	}

	// Generate multiple and verify none panic
	for i := 0; i < 100; i++ {
		g := ga.WFCGenome(16 + rng.Intn(24))
		if len(g) < MinGenome || len(g) > MaxGenome {
			t.Errorf("iteration %d: genome size %d out of bounds", i, len(g))
		}
	}
}

func TestWFCGenomeFallback(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ga := NewGA(rng)
	ga.WFCEnabled = true
	ga.Archetypes = nil // no archetypes → empty constraints

	genome := ga.WFCGenome(24)
	if len(genome) < MinGenome {
		t.Errorf("fallback genome length %d < MinGenome", len(genome))
	}
	// Should still produce a valid genome via RandomGenome fallback
	if len(genome) > MaxGenome {
		t.Errorf("fallback genome length %d > MaxGenome", len(genome))
	}
}

func TestBranchOffsets(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	// Token sequence with a branch that should target the yield
	tokens := []TokenType{TokSense, TokPush, TokCmp, TokBranch, TokMove, TokAction, TokYield}
	bytecode := RenderTokens(tokens, rng)

	// Walk bytecode to find branch instructions
	pc := 0
	for pc < len(bytecode) {
		op := bytecode[pc]
		if op == micro.OpJumpNZ || op == micro.OpJumpZ {
			if pc+1 >= len(bytecode) {
				t.Fatalf("branch at %d has no offset byte", pc)
			}
			offset := bytecode[pc+1]
			target := pc + 2 + int(offset)
			if target > len(bytecode) {
				t.Errorf("branch at %d: offset %d → target %d exceeds bytecode length %d",
					pc, offset, target, len(bytecode))
			}
			if offset == 0 {
				t.Errorf("branch at %d has zero offset (infinite loop)", pc)
			}
		}
		size := opcodeSize(op, bytecode, pc)
		pc += size
	}
}

// --- 8-Type WFC Tests ---

func TestClassifyOpcode8(t *testing.T) {
	tests := []struct {
		name string
		code []byte
		want TokenType8
	}{
		{"sense", []byte{micro.OpRing0R, 0x0D}, Tok8Sense},
		{"push", []byte{0x25}, Tok8Push},
		{"cmp eq", []byte{micro.OpEq}, Tok8Cmp},
		{"branch jnz", []byte{micro.OpJumpNZ, 0x08}, Tok8Branch},
		{"move r1w0", []byte{micro.OpRing1W, 0x00}, Tok8Move},
		{"action r1w1", []byte{micro.OpRing1W, 0x01}, Tok8Action},
		{"target r1w2", []byte{micro.OpRing1W, 0x02}, Tok8Action}, // target merged into action
		{"stack dup", []byte{micro.OpDup}, Tok8Ops},                // stack merged into ops
		{"math add", []byte{micro.OpAdd}, Tok8Ops},                 // math merged into ops
		{"yield", []byte{micro.OpYield}, Tok8Yield},
		{"halt", []byte{micro.OpHalt}, Tok8Yield},
		{"act.eat", []byte{micro.OpActEat, 0x00}, Tok8Action},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ClassifyOpcode8(tt.code[0], tt.code, 0)
			if got != tt.want {
				t.Errorf("ClassifyOpcode8(%02x) = %d, want %d", tt.code[0], got, tt.want)
			}
		})
	}
}

func TestTokenizeGenome8(t *testing.T) {
	tokens := TokenizeGenome8(testTraderGenome)
	if len(tokens) == 0 {
		t.Fatal("empty")
	}
	if tokens[0] != Tok8Sense {
		t.Errorf("first = %d, want Tok8Sense", tokens[0])
	}
	// Should have no TokTarget or TokStack/TokMath — they're merged
	for _, tok := range tokens {
		if tok >= Num8Types {
			t.Errorf("token %d exceeds Num8Types", tok)
		}
	}
}

func TestMineConstraints8(t *testing.T) {
	constraints := MineConstraints8(testArchetypes)
	// Sense → Push should be present
	if constraints[Tok8Sense]&(1<<uint(Tok8Push)) == 0 {
		t.Error("expected Sense→Push")
	}
	// Push → Cmp should be present
	if constraints[Tok8Push]&(1<<uint(Tok8Cmp)) == 0 {
		t.Error("expected Push→Cmp")
	}
}

func TestBaseConstraints8(t *testing.T) {
	base := BaseConstraints8()
	// Sense should allow Push and Cmp
	if base[Tok8Sense]&(1<<uint(Tok8Push)) == 0 {
		t.Error("base: Sense should allow Push")
	}
	if base[Tok8Sense]&(1<<uint(Tok8Cmp)) == 0 {
		t.Error("base: Sense should allow Cmp")
	}
	// Yield should allow Sense
	if base[Tok8Yield]&(1<<uint(Tok8Sense)) == 0 {
		t.Error("base: Yield should allow Sense")
	}
}

func TestWFC8Generate(t *testing.T) {
	base := BaseConstraints8()
	mined := MineConstraints8(testArchetypes)
	merged := MergeConstraints8(mined, base)

	// Try multiple seeds — some may hit contradictions (expected)
	successes := 0
	for seed := int64(0); seed < 50; seed++ {
		rng := rand.New(rand.NewSource(seed))
		wfc := NewWFC1D8(10, merged, rng)
		wfc.Collapse8(0, Tok8Sense)
		wfc.Collapse8(9, Tok8Yield)

		if !wfc.Generate8() {
			continue
		}
		successes++

		tokens := wfc.ToTokens8()
		if len(tokens) != 10 {
			t.Fatalf("seed %d: expected 10 tokens, got %d", seed, len(tokens))
		}
		if tokens[0] != Tok8Sense {
			t.Errorf("seed %d: first = %d, want Tok8Sense", seed, tokens[0])
		}
		if tokens[9] != Tok8Yield {
			t.Errorf("seed %d: last = %d, want Tok8Yield", seed, tokens[9])
		}
		for i, tok := range tokens {
			if tok >= Num8Types {
				t.Errorf("seed %d: token[%d] = %d exceeds Num8Types", seed, i, tok)
			}
		}
	}
	if successes == 0 {
		t.Fatal("WFC1D8.Generate8() failed for all 50 seeds")
	}
	t.Logf("WFC8 succeeded %d/50 seeds", successes)
}

func TestRenderTokens8(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	tokens := []TokenType8{Tok8Sense, Tok8Push, Tok8Cmp, Tok8Branch, Tok8Move, Tok8Action, Tok8Yield}
	bytecode := RenderTokens8(tokens, rng)
	if len(bytecode) == 0 {
		t.Fatal("empty bytecode")
	}
	if bytecode[0] != micro.OpRing0R {
		t.Errorf("first byte = %02x, want OpRing0R", bytecode[0])
	}
	if bytecode[len(bytecode)-1] != micro.OpYield {
		t.Errorf("last byte = %02x, want OpYield", bytecode[len(bytecode)-1])
	}
}

func TestWFC8MinedConstraints(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	mined := MineConstraints8(testArchetypes)
	base := BaseConstraints8()
	merged := MergeConstraints8(mined, base)

	// Generate 100 genomes, verify they tokenize to valid sequences
	for i := 0; i < 100; i++ {
		numTokens := 6 + rng.Intn(8)
		wfc := NewWFC1D8(numTokens, merged, rng)
		wfc.Collapse8(0, Tok8Sense)
		wfc.Collapse8(numTokens-1, Tok8Yield)
		if !wfc.Generate8() {
			continue // some contradictions are ok
		}
		tokens := wfc.ToTokens8()
		bytecode := RenderTokens8(tokens, rng)
		// Re-tokenize and verify round-trip
		retok := TokenizeGenome8(bytecode)
		if len(retok) == 0 {
			t.Errorf("iteration %d: re-tokenize produced empty", i)
		}
	}
}

func TestWFC8vsPureRandom(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	mined := MineConstraints8(testArchetypes)
	base := BaseConstraints8()
	merged := MergeConstraints8(mined, base)

	// Count sense→cmp→branch patterns in WFC vs random
	wfcPatterns := 0
	randPatterns := 0
	trials := 200

	for i := 0; i < trials; i++ {
		// WFC genome
		numTokens := 8 + rng.Intn(6)
		wfc := NewWFC1D8(numTokens, merged, rng)
		wfc.Collapse8(0, Tok8Sense)
		wfc.Collapse8(numTokens-1, Tok8Yield)
		if wfc.Generate8() {
			tokens := wfc.ToTokens8()
			wfcPatterns += countSCBPatterns8(tokens)
		}

		// Random genome (same size, no constraints)
		randTokens := make([]TokenType8, numTokens)
		for j := range randTokens {
			randTokens[j] = TokenType8(rng.Intn(int(Num8Types)))
		}
		randPatterns += countSCBPatterns8(randTokens)
	}

	t.Logf("WFC sense→cmp→branch patterns: %d / %d trials", wfcPatterns, trials)
	t.Logf("Random sense→cmp→branch patterns: %d / %d trials", randPatterns, trials)

	// WFC should produce more structured patterns than random
	if wfcPatterns <= randPatterns {
		t.Errorf("WFC patterns (%d) should exceed random (%d)", wfcPatterns, randPatterns)
	}
}

// countSCBPatterns8 counts sense→cmp→branch subsequences.
func countSCBPatterns8(tokens []TokenType8) int {
	count := 0
	for i := 0; i < len(tokens)-2; i++ {
		if tokens[i] == Tok8Sense {
			// Look for cmp within next 3 tokens
			for j := i + 1; j < len(tokens)-1 && j <= i+3; j++ {
				if tokens[j] == Tok8Cmp {
					// Look for branch within next 2 tokens
					for k := j + 1; k < len(tokens) && k <= j+2; k++ {
						if tokens[k] == Tok8Branch {
							count++
							break
						}
					}
					break
				}
			}
		}
	}
	return count
}

func TestWFC8Genome(t *testing.T) {
	rng := rand.New(rand.NewSource(42))
	ga := NewGA(rng)
	ga.WFCEnabled = true
	ga.Archetypes = testArchetypes
	ga.UpdateConstraints(testArchetypes)

	genome := ga.WFC8Genome(24)
	if len(genome) < MinGenome {
		t.Errorf("genome length %d < MinGenome", len(genome))
	}
	if len(genome) > MaxGenome {
		t.Errorf("genome length %d > MaxGenome", len(genome))
	}

	// Generate multiple and verify none panic
	for i := 0; i < 100; i++ {
		g := ga.WFC8Genome(16 + rng.Intn(24))
		if len(g) < MinGenome || len(g) > MaxGenome {
			t.Errorf("iteration %d: genome size %d out of bounds", i, len(g))
		}
	}
}

func TestWFCGenomeStressNoPanic(t *testing.T) {
	// Generate many WFC genomes with various seeds to check for panics
	for seed := int64(0); seed < 50; seed++ {
		rng := rand.New(rand.NewSource(seed))
		ga := NewGA(rng)
		ga.WFCEnabled = true
		ga.Archetypes = testArchetypes
		ga.UpdateConstraints(testArchetypes)

		for i := 0; i < 20; i++ {
			size := 16 + rng.Intn(32)
			g := ga.WFCGenome(size)
			if len(g) < MinGenome || len(g) > MaxGenome {
				t.Errorf("seed=%d i=%d: genome size %d out of bounds", seed, i, len(g))
			}
		}
	}
}
