package interpreter

import (
	"bytes"
	"fmt"
	"strings"
	"testing"

	"github.com/psilLang/psil/pkg/parser"
	"github.com/psilLang/psil/pkg/types"
)

// Helper to run PSIL code and get results
func runPSIL(t *testing.T, code string) *Interpreter {
	t.Helper()
	interp := New()
	prog, err := parser.Parse(code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	values, defs := prog.ToValues()
	for name, q := range defs {
		interp.Define(name, q)
	}
	if err := interp.Run(values); err != nil {
		t.Fatalf("Runtime error: %v", err)
	}
	return interp
}

// Helper to run and capture output
func runPSILWithOutput(t *testing.T, code string) (*Interpreter, string) {
	t.Helper()
	interp := New()
	var buf bytes.Buffer
	interp.Output = &buf

	prog, err := parser.Parse(code)
	if err != nil {
		t.Fatalf("Parse error: %v", err)
	}
	values, defs := prog.ToValues()
	for name, q := range defs {
		interp.Define(name, q)
	}
	if err := interp.Run(values); err != nil {
		t.Fatalf("Runtime error: %v", err)
	}
	return interp, buf.String()
}

// === Basic Tests ===

func TestHelloWorld(t *testing.T) {
	_, output := runPSILWithOutput(t, `"Hello, World!" .`)
	expected := "Hello, World!\n"
	if output != expected {
		t.Errorf("Expected %q, got %q", expected, output)
	}
}

func TestBasicArithmetic(t *testing.T) {
	tests := []struct {
		code     string
		expected types.Number
	}{
		{"2 3 +", 5},
		{"10 4 -", 6},
		{"6 7 *", 42},
		{"20 4 /", 5},
		{"17 5 mod", 2},
		{"5 neg", -5},
		{"-3 abs", 3},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			interp := runPSIL(t, tt.code)
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item on stack, got %d", len(interp.Stack))
			}
			result, ok := interp.Stack[0].(types.Number)
			if !ok {
				t.Fatalf("Expected Number, got %T", interp.Stack[0])
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestStackOperations(t *testing.T) {
	tests := []struct {
		code     string
		expected []types.Number
	}{
		{"1 2 3", []types.Number{1, 2, 3}},
		{"1 dup", []types.Number{1, 1}},
		{"1 2 drop", []types.Number{1}},
		{"1 2 swap", []types.Number{2, 1}},
		{"1 2 over", []types.Number{1, 2, 1}},
		{"1 2 3 rot", []types.Number{2, 3, 1}},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			interp := runPSIL(t, tt.code)
			if len(interp.Stack) != len(tt.expected) {
				t.Fatalf("Expected %d items, got %d", len(tt.expected), len(interp.Stack))
			}
			for i, exp := range tt.expected {
				result, ok := interp.Stack[i].(types.Number)
				if !ok {
					t.Fatalf("Item %d: Expected Number, got %T", i, interp.Stack[i])
				}
				if result != exp {
					t.Errorf("Item %d: Expected %v, got %v", i, exp, result)
				}
			}
		})
	}
}

func TestComparison(t *testing.T) {
	tests := []struct {
		code     string
		expected bool
		zFlag    bool
	}{
		{"3 5 <", true, true},
		{"5 3 <", false, false},
		{"5 5 =", true, true},
		{"5 3 =", false, false},
		{"5 3 >", true, true},
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			interp := runPSIL(t, tt.code)
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item on stack, got %d", len(interp.Stack))
			}
			result, ok := interp.Stack[0].(types.Boolean)
			if !ok {
				t.Fatalf("Expected Boolean, got %T", interp.Stack[0])
			}
			if bool(result) != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
			if interp.ZFlag != tt.zFlag {
				t.Errorf("Expected Z=%v, got Z=%v", tt.zFlag, interp.ZFlag)
			}
		})
	}
}

// === Control Flow Tests ===

func TestIfte(t *testing.T) {
	// ifte preserves the original stack and only runs the selected branch
	tests := []struct {
		code     string
		expected types.Number
	}{
		// Drop the input value, push result
		{"5 [dup 3 >] [drop 10] [drop 20] ifte", 10},
		{"2 [dup 3 >] [drop 10] [drop 20] ifte", 20},
		{"0 [dup 0 =] [drop 100] [drop 200] ifte", 100},
		// Using the value - condition preserves stack
		{"5 [dup 2 >] [dup *] [1 +] ifte", 25}, // 5 > 2, so 5*5 = 25
		{"1 [dup 2 >] [dup *] [1 +] ifte", 2},  // 1 <= 2, so 1+1 = 2
	}

	for _, tt := range tests {
		t.Run(tt.code, func(t *testing.T) {
			interp := runPSIL(t, tt.code)
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item on stack, got %d: %v", len(interp.Stack), interp.StackString())
			}
			result, ok := interp.Stack[0].(types.Number)
			if !ok {
				t.Fatalf("Expected Number, got %T", interp.Stack[0])
			}
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTimes(t *testing.T) {
	interp := runPSIL(t, "0 5 [1 +] times")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(types.Number)
	if result != 5 {
		t.Errorf("Expected 5, got %v", result)
	}
}

func TestWhile(t *testing.T) {
	interp := runPSIL(t, "1 [dup 10 <] [dup 1 + swap drop] while")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(types.Number)
	if result != 10 {
		t.Errorf("Expected 10, got %v", result)
	}
}

// === Classic Algorithms ===

func TestFactorial(t *testing.T) {
	code := `
		DEFINE fact == [
			[dup 0 =]
			[drop 1]
			[dup 1 -]
			[*]
			linrec
		].
	`
	tests := []struct {
		n        int
		expected types.Number
	}{
		{0, 1},
		{1, 1},
		{5, 120},
		{10, 3628800},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("%d!", tt.n), func(t *testing.T) {
			interp := runPSIL(t, fmt.Sprintf("%s %d fact", code, tt.n))
			if interp.HasError() {
				t.Fatalf("Error: %s", types.ErrorMessage(interp.ARegister))
			}
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
			}
			result := interp.Stack[0].(types.Number)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestFibonacci(t *testing.T) {
	code := `
		DEFINE fib == [
			[dup 2 <]
			[]
			[dup 1 - fib swap 2 - fib +]
			ifte
		].
	`
	// Fib sequence: 0, 1, 1, 2, 3, 5, 8, 13, 21, 34, 55
	tests := []struct {
		n        int
		expected types.Number
	}{
		{0, 0},
		{1, 1},
		{2, 1},
		{5, 5},
		{10, 55},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("fib(%d)", tt.n), func(t *testing.T) {
			interp := runPSIL(t, fmt.Sprintf("%s %d fib", code, tt.n))
			if interp.HasError() {
				t.Fatalf("Error: %s", types.ErrorMessage(interp.ARegister))
			}
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
			}
			result := interp.Stack[0].(types.Number)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

func TestTribonacci(t *testing.T) {
	// Tribonacci: T(n) = T(n-1) + T(n-2) + T(n-3), with T(0)=0, T(1)=0, T(2)=1
	// Sequence: 0, 0, 1, 1, 2, 4, 7, 13, 24, 44, 81
	code := `
		DEFINE trib == [
			[dup 2 <]
			[drop 0]
			[[dup 2 =] [drop 1] [dup 1 - trib swap dup 2 - trib swap 3 - trib + +] ifte]
			ifte
		].
	`
	tests := []struct {
		n        int
		expected types.Number
	}{
		{0, 0},
		{1, 0},
		{2, 1},
		{3, 1},
		{4, 2},
		{5, 4},
		{6, 7},
		{7, 13},
	}

	for _, tt := range tests {
		t.Run(fmt.Sprintf("trib(%d)", tt.n), func(t *testing.T) {
			interp := runPSIL(t, fmt.Sprintf("%s %d trib", code, tt.n))
			if interp.HasError() {
				t.Fatalf("Error: %s", types.ErrorMessage(interp.ARegister))
			}
			if len(interp.Stack) != 1 {
				t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
			}
			result := interp.Stack[0].(types.Number)
			if result != tt.expected {
				t.Errorf("Expected %v, got %v", tt.expected, result)
			}
		})
	}
}

// === List Operations ===

func TestMap(t *testing.T) {
	interp := runPSIL(t, "[1 2 3 4 5] [dup *] map")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result, ok := interp.Stack[0].(*types.Quotation)
	if !ok {
		t.Fatalf("Expected Quotation, got %T", interp.Stack[0])
	}
	expected := []types.Number{1, 4, 9, 16, 25}
	if len(result.Items) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result.Items))
	}
	for i, exp := range expected {
		if result.Items[i].(types.Number) != exp {
			t.Errorf("Item %d: expected %v, got %v", i, exp, result.Items[i])
		}
	}
}

func TestFilter(t *testing.T) {
	interp := runPSIL(t, "[1 2 3 4 5 6] [2 mod 0 =] filter")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(*types.Quotation)
	expected := []types.Number{2, 4, 6}
	if len(result.Items) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result.Items))
	}
}

func TestFold(t *testing.T) {
	// Sum using fold
	interp := runPSIL(t, "0 [1 2 3 4 5] [+] fold")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(types.Number)
	if result != 15 {
		t.Errorf("Expected 15, got %v", result)
	}
}

func TestRange(t *testing.T) {
	interp := runPSIL(t, "1 6 range")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(*types.Quotation)
	expected := []types.Number{1, 2, 3, 4, 5}
	if len(result.Items) != len(expected) {
		t.Fatalf("Expected %d items, got %d", len(expected), len(result.Items))
	}
}

func TestZip(t *testing.T) {
	interp := runPSIL(t, "[1 2 3] [4 5 6] zip")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(*types.Quotation)
	if len(result.Items) != 3 {
		t.Fatalf("Expected 3 pairs, got %d", len(result.Items))
	}
	// First pair should be [1 4]
	pair := result.Items[0].(*types.Quotation)
	if pair.Items[0].(types.Number) != 1 || pair.Items[1].(types.Number) != 4 {
		t.Errorf("First pair should be [1 4], got %v", pair)
	}
}

// === Error Handling ===

func TestStackUnderflow(t *testing.T) {
	interp := runPSIL(t, "drop")
	if !interp.HasError() {
		t.Error("Expected error flag to be set")
	}
	if interp.ARegister != types.ErrStackUnderflow {
		t.Errorf("Expected stack underflow error, got %d", interp.ARegister)
	}
}

func TestDivisionByZero(t *testing.T) {
	interp := runPSIL(t, "5 0 /")
	if !interp.HasError() {
		t.Error("Expected error flag to be set")
	}
	if interp.ARegister != types.ErrDivisionByZero {
		t.Errorf("Expected division by zero error, got %d", interp.ARegister)
	}
}

func TestTryCatch(t *testing.T) {
	// try should catch the error
	// Handler receives error code, so stack has: errorcode 999
	interp := runPSIL(t, "[drop] [drop 999] try")
	if interp.HasError() {
		t.Error("Error should have been caught")
	}
	// Handler drops error code, pushes 999
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d: %s", len(interp.Stack), interp.StackString())
	}
	result := interp.Stack[0].(types.Number)
	if result != 999 {
		t.Errorf("Expected 999, got %v", result)
	}
}

// === Definitions ===

func TestDefine(t *testing.T) {
	code := `
		DEFINE sq == [dup *].
		DEFINE cube == [dup sq *].
		3 cube
	`
	interp := runPSIL(t, code)
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	result := interp.Stack[0].(types.Number)
	if result != 27 {
		t.Errorf("Expected 27, got %v", result)
	}
}

// === Integration Tests ===

func TestComplexProgram(t *testing.T) {
	// Sum of squares of first 10 numbers
	code := `
		DEFINE sq == [dup *].
		DEFINE sum == [0 swap [+] fold].
		10 iota [sq] map sum
	`
	interp := runPSIL(t, code)
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	// 0^2 + 1^2 + 2^2 + ... + 9^2 = 285
	result := interp.Stack[0].(types.Number)
	if result != 285 {
		t.Errorf("Expected 285, got %v", result)
	}
}

func TestQuotationComposition(t *testing.T) {
	interp := runPSIL(t, "[1 +] [2 *] concat 5 swap i")
	if len(interp.Stack) != 1 {
		t.Fatalf("Expected 1 item, got %d", len(interp.Stack))
	}
	// 5 [1 + 2 *] i = (5 + 1) * 2 = 12
	result := interp.Stack[0].(types.Number)
	if result != 12 {
		t.Errorf("Expected 12, got %v", result)
	}
}

// === Output Tests ===

func TestPrintNumbers(t *testing.T) {
	_, output := runPSILWithOutput(t, "42 . 3.14 .")
	lines := strings.Split(strings.TrimSpace(output), "\n")
	if len(lines) != 2 {
		t.Fatalf("Expected 2 lines, got %d", len(lines))
	}
	if lines[0] != "42" {
		t.Errorf("Line 1: expected '42', got '%s'", lines[0])
	}
	if lines[1] != "3.14" {
		t.Errorf("Line 2: expected '3.14', got '%s'", lines[1])
	}
}

func TestPrintStrings(t *testing.T) {
	_, output := runPSILWithOutput(t, `"test" .`)
	if strings.TrimSpace(output) != "test" {
		t.Errorf("Expected 'test', got '%s'", output)
	}
}
