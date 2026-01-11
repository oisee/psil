// Package interpreter - builtins.go contains all built-in operations
package interpreter

import (
	"fmt"
	"image/png"
	"math"
	"os"

	"github.com/psilLang/psil/pkg/types"
)

// RegisterBuiltins registers all built-in operations
func (i *Interpreter) RegisterBuiltins() {
	// Stack manipulation
	i.registerBuiltin("dup", builtinDup)
	i.registerBuiltin("drop", builtinDrop)
	i.registerBuiltin("pop", builtinDrop) // alias
	i.registerBuiltin("swap", builtinSwap)
	i.registerBuiltin("over", builtinOver)
	i.registerBuiltin("rot", builtinRot)
	i.registerBuiltin("nip", builtinNip)
	i.registerBuiltin("tuck", builtinTuck)
	i.registerBuiltin("dup2", builtinDup2)
	i.registerBuiltin("drop2", builtinDrop2)
	i.registerBuiltin("clear", builtinClear)
	i.registerBuiltin("depth", builtinDepth)
	i.registerBuiltin("roll", builtinRoll)       // n roll: rotate n items (bring nth to top)
	i.registerBuiltin("unroll", builtinRollNeg) // n unroll: rotate opposite (put top at nth)
	i.registerBuiltin("pick", builtinPick)       // n pick: copy nth item to top

	// Arithmetic
	i.registerBuiltin("+", builtinAdd)
	i.registerBuiltin("add", builtinAdd)
	i.registerBuiltin("-", builtinSub)
	i.registerBuiltin("sub", builtinSub)
	i.registerBuiltin("*", builtinMul)
	i.registerBuiltin("mul", builtinMul)
	i.registerBuiltin("/", builtinDiv)
	i.registerBuiltin("div", builtinDiv)
	i.registerBuiltin("mod", builtinMod)
	i.registerBuiltin("%", builtinMod)
	i.registerBuiltin("neg", builtinNeg)
	i.registerBuiltin("abs", builtinAbs)
	i.registerBuiltin("inc", builtinInc)
	i.registerBuiltin("dec", builtinDec)

	// Comparison (sets Z flag)
	i.registerBuiltin("<", builtinLT)
	i.registerBuiltin(">", builtinGT)
	i.registerBuiltin("<=", builtinLE)
	i.registerBuiltin(">=", builtinGE)
	i.registerBuiltin("=", builtinEQ)
	i.registerBuiltin("!=", builtinNE)
	i.registerBuiltin("eq", builtinEQ)
	i.registerBuiltin("neq", builtinNE)

	// Logic
	i.registerBuiltin("and", builtinAnd)
	i.registerBuiltin("or", builtinOr)
	i.registerBuiltin("not", builtinNot)

	// Type predicates
	i.registerBuiltin("number?", builtinIsNumber)
	i.registerBuiltin("string?", builtinIsString)
	i.registerBuiltin("boolean?", builtinIsBoolean)
	i.registerBuiltin("quotation?", builtinIsQuotation)
	i.registerBuiltin("symbol?", builtinIsSymbol)

	// Quotation operations
	i.registerBuiltin("i", builtinI)       // execute
	i.registerBuiltin("call", builtinI)    // alias
	i.registerBuiltin("x", builtinX)       // dup + execute
	i.registerBuiltin("dip", builtinDip)   // save, execute, restore
	i.registerBuiltin("concat", builtinConcat)
	i.registerBuiltin("cons", builtinCons)
	i.registerBuiltin("uncons", builtinUncons)
	i.registerBuiltin("first", builtinFirst)
	i.registerBuiltin("rest", builtinRest)
	i.registerBuiltin("size", builtinSize)
	i.registerBuiltin("length", builtinSize) // alias
	i.registerBuiltin("null?", builtinIsNull)
	i.registerBuiltin("empty?", builtinIsNull) // alias
	i.registerBuiltin("quote", builtinQuote)
	i.registerBuiltin("unit", builtinUnit) // wrap in quotation

	// List operations (native for performance)
	i.registerBuiltin("reverse", builtinReverse)
	i.registerBuiltin("nth", builtinNth)
	i.registerBuiltin("take", builtinTake)
	i.registerBuiltin("ldrop", builtinDropN) // list-drop, not stack-drop
	i.registerBuiltin("split", builtinSplit)
	i.registerBuiltin("zip", builtinZip)
	i.registerBuiltin("zipwith", builtinZipWith)
	i.registerBuiltin("range", builtinRange)
	i.registerBuiltin("iota", builtinIota)
	i.registerBuiltin("flatten", builtinFlatten)
	i.registerBuiltin("any", builtinAny)
	i.registerBuiltin("all", builtinAll)
	i.registerBuiltin("find", builtinFind)
	i.registerBuiltin("index", builtinIndex)
	i.registerBuiltin("sort", builtinSort)
	i.registerBuiltin("last", builtinLast)

	// I/O
	i.registerBuiltin(".", builtinPrint)
	i.registerBuiltin("print", builtinPrintNoNL)
	i.registerBuiltin("newline", builtinNewline)
	i.registerBuiltin("stack", builtinShowStack)

	// Error handling
	i.registerBuiltin("err?", builtinErrQ)
	i.registerBuiltin("errcode", builtinErrCode)
	i.registerBuiltin("clearerr", builtinClearErr)

	// Z flag operations
	i.registerBuiltin("z?", builtinZQ)
	i.registerBuiltin("setz", builtinSetZ)
	i.registerBuiltin("clrz", builtinClrZ)

	// Boolean constants
	i.Define("true", types.Boolean(true))
	i.Define("false", types.Boolean(false))

	// Definition (point-free style)
	i.registerBuiltin("define", builtinDefine)   // [quotation] "name" define
	i.registerBuiltin("undefine", builtinUndefine) // "name" undefine

	// Math functions
	i.registerBuiltin("sin", builtinSin)
	i.registerBuiltin("cos", builtinCos)
	i.registerBuiltin("tan", builtinTan)
	i.registerBuiltin("asin", builtinAsin)
	i.registerBuiltin("acos", builtinAcos)
	i.registerBuiltin("atan", builtinAtan)
	i.registerBuiltin("atan2", builtinAtan2)
	i.registerBuiltin("sqrt", builtinSqrt)
	i.registerBuiltin("pow", builtinPow)
	i.registerBuiltin("exp", builtinExp)
	i.registerBuiltin("log", builtinLog)
	i.registerBuiltin("floor", builtinFloor)
	i.registerBuiltin("ceil", builtinCeil)
	i.registerBuiltin("round", builtinRound)
	i.registerBuiltin("min", builtinMin)
	i.registerBuiltin("max", builtinMax)
	i.registerBuiltin("clamp", builtinClamp)
	i.registerBuiltin("lerp", builtinLerp)
	i.registerBuiltin("sign", builtinSign)
	i.registerBuiltin("fract", builtinFract)
	i.registerBuiltin("smoothstep", builtinSmoothstep)

	// Math constants
	i.Define("pi", types.Number(math.Pi))
	i.Define("e", types.Number(math.E))
	i.Define("tau", types.Number(math.Pi*2))

	// Graphics operations
	i.registerBuiltin("img-new", builtinImgNew)
	i.registerBuiltin("img-setpixel", builtinImgSetPixel)
	i.registerBuiltin("img-getpixel", builtinImgGetPixel)
	i.registerBuiltin("img-save", builtinImgSave)
	i.registerBuiltin("img-width", builtinImgWidth)
	i.registerBuiltin("img-height", builtinImgHeight)
	i.registerBuiltin("img-fill", builtinImgFill)
	i.registerBuiltin("image?", builtinIsImage)
	i.registerBuiltin("img-render", builtinImgRender) // render with shader quotation
}

func (i *Interpreter) registerBuiltin(name string, fn func(*Interpreter) error) {
	i.Dictionary[name] = &types.Builtin{
		Name: name,
		Fn: func(interp interface{}) error {
			return fn(interp.(*Interpreter))
		},
	}
}

// === Stack manipulation ===

func builtinDup(i *Interpreter) error {
	v := i.Peek()
	if v != nil {
		i.Push(v)
	}
	return nil
}

func builtinDrop(i *Interpreter) error {
	i.Pop()
	return nil
}

func builtinSwap(i *Interpreter) error {
	if len(i.Stack) < 2 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	n := len(i.Stack)
	i.Stack[n-1], i.Stack[n-2] = i.Stack[n-2], i.Stack[n-1]
	return nil
}

func builtinOver(i *Interpreter) error {
	v := i.PeekN(1)
	if v != nil {
		i.Push(v)
	}
	return nil
}

func builtinRot(i *Interpreter) error {
	// ( a b c -- b c a )
	if len(i.Stack) < 3 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	n := len(i.Stack)
	a := i.Stack[n-3]
	i.Stack[n-3] = i.Stack[n-2]
	i.Stack[n-2] = i.Stack[n-1]
	i.Stack[n-1] = a
	return nil
}

func builtinNip(i *Interpreter) error {
	// ( a b -- b )
	if len(i.Stack) < 2 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	n := len(i.Stack)
	i.Stack[n-2] = i.Stack[n-1]
	i.Stack = i.Stack[:n-1]
	return nil
}

func builtinTuck(i *Interpreter) error {
	// ( a b -- b a b )
	if len(i.Stack) < 2 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	builtinSwap(i)
	builtinOver(i)
	return nil
}

func builtinDup2(i *Interpreter) error {
	// ( a b -- a b a b )
	if len(i.Stack) < 2 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	builtinOver(i)
	builtinOver(i)
	return nil
}

func builtinDrop2(i *Interpreter) error {
	if len(i.Stack) < 2 {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	i.Stack = i.Stack[:len(i.Stack)-2]
	return nil
}

func builtinClear(i *Interpreter) error {
	i.Stack = i.Stack[:0]
	return nil
}

func builtinDepth(i *Interpreter) error {
	i.Push(types.Number(len(i.Stack)))
	return nil
}

// roll: n roll - rotate top n items (bring nth item to top)
// 3 roll: a b c d -> b c d a (bring 3rd item to top)
func builtinRoll(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	count := int(n)
	if count <= 0 || count > len(i.Stack) {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	if count == 1 {
		return nil // no-op
	}
	// Get the item at position (stack_len - count)
	idx := len(i.Stack) - count
	item := i.Stack[idx]
	// Shift items down
	copy(i.Stack[idx:], i.Stack[idx+1:])
	i.Stack[len(i.Stack)-1] = item
	return nil
}

// -roll: n -roll - rotate opposite direction (put top at nth position)
// 3 -roll: a b c d -> d a b c (put top at 3rd position)
func builtinRollNeg(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	count := int(n)
	if count <= 0 || count > len(i.Stack) {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	if count == 1 {
		return nil // no-op
	}
	// Get top item
	top := i.Stack[len(i.Stack)-1]
	// Shift items up
	idx := len(i.Stack) - count
	copy(i.Stack[idx+1:], i.Stack[idx:len(i.Stack)-1])
	i.Stack[idx] = top
	return nil
}

// pick: n pick - copy nth item to top (0 = top, 1 = second, etc.)
func builtinPick(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	idx := int(n)
	if idx < 0 || idx >= len(i.Stack) {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}
	item := i.Stack[len(i.Stack)-1-idx]
	i.Push(item)
	return nil
}

// === Arithmetic ===

func builtinAdd(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a + b)
	return nil
}

func builtinSub(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a - b)
	return nil
}

func builtinMul(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a * b)
	return nil
}

func builtinDiv(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	if b == 0 {
		i.SetError(types.ErrDivisionByZero)
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a / b)
	return nil
}

func builtinMod(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	if b == 0 {
		i.SetError(types.ErrDivisionByZero)
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Mod(float64(a), float64(b))))
	return nil
}

func builtinNeg(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(-a)
	return nil
}

func builtinAbs(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	if a < 0 {
		i.Push(-a)
	} else {
		i.Push(a)
	}
	return nil
}

func builtinInc(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a + 1)
	return nil
}

func builtinDec(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(a - 1)
	return nil
}

// === Comparison (sets Z flag) ===

func builtinLT(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := a < b
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinGT(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := a > b
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinLE(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := a <= b
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinGE(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := a >= b
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinEQ(i *Interpreter) error {
	b := i.Pop()
	if b == nil {
		return nil
	}
	a := i.Pop()
	if a == nil {
		return nil
	}
	result := a.Equal(b)
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinNE(i *Interpreter) error {
	b := i.Pop()
	if b == nil {
		return nil
	}
	a := i.Pop()
	if a == nil {
		return nil
	}
	result := !a.Equal(b)
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

// === Logic ===

func builtinAnd(i *Interpreter) error {
	b, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	a, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	result := bool(a) && bool(b)
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinOr(i *Interpreter) error {
	b, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	a, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	result := bool(a) || bool(b)
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

func builtinNot(i *Interpreter) error {
	a, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	result := !bool(a)
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

// === Type predicates ===

func builtinIsNumber(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(types.Number)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

func builtinIsString(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(types.String)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

func builtinIsBoolean(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(types.Boolean)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

func builtinIsQuotation(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(*types.Quotation)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

func builtinIsSymbol(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(types.Symbol)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

// === Quotation operations ===

// i (call) - execute a quotation
func builtinI(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	return i.ExecuteQuotation(q)
}

// x - dup and execute: [Q] x = [Q] [Q] i
func builtinX(i *Interpreter) error {
	q := i.Peek()
	if q == nil {
		return nil
	}
	if qu, ok := q.(*types.Quotation); ok {
		return i.ExecuteQuotation(qu)
	}
	i.SetError(types.ErrTypeMismatch)
	return nil
}

// dip - execute quotation with top value saved: a [Q] dip = Q a
func builtinDip(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	saved := i.Pop()
	if saved == nil {
		return nil
	}
	err := i.ExecuteQuotation(q)
	i.Push(saved)
	return err
}

// concat - join two quotations
func builtinConcat(i *Interpreter) error {
	b, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	a, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	items := make([]types.Value, 0, len(a.Items)+len(b.Items))
	items = append(items, a.Items...)
	items = append(items, b.Items...)
	i.Push(&types.Quotation{Items: items})
	return nil
}

// cons - prepend value to quotation: a [Q] cons = [a Q...]
func builtinCons(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	v := i.Pop()
	if v == nil {
		return nil
	}
	items := make([]types.Value, 0, len(q.Items)+1)
	items = append(items, v)
	items = append(items, q.Items...)
	i.Push(&types.Quotation{Items: items})
	return nil
}

// uncons - split quotation: [a Q...] uncons = a [Q...]
func builtinUncons(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	if len(q.Items) == 0 {
		i.SetError(types.ErrInvalidQuotation)
		return nil
	}
	i.Push(q.Items[0])
	i.Push(&types.Quotation{Items: q.Items[1:]})
	return nil
}

// first - get first element of quotation
func builtinFirst(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	if len(q.Items) == 0 {
		i.SetError(types.ErrInvalidQuotation)
		return nil
	}
	i.Push(q.Items[0])
	return nil
}

// rest - get tail of quotation
func builtinRest(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	if len(q.Items) == 0 {
		i.Push(&types.Quotation{Items: nil})
		return nil
	}
	i.Push(&types.Quotation{Items: q.Items[1:]})
	return nil
}

// size - get length of quotation
func builtinSize(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	i.Push(types.Number(len(q.Items)))
	return nil
}

// null? - check if quotation is empty
func builtinIsNull(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	result := len(q.Items) == 0
	i.ZFlag = result
	i.Push(types.Boolean(result))
	return nil
}

// quote - wrap top of stack in quotation
func builtinQuote(i *Interpreter) error {
	v := i.Pop()
	if v == nil {
		return nil
	}
	i.Push(&types.Quotation{Items: []types.Value{v}})
	return nil
}

// unit - alias for quote
func builtinUnit(i *Interpreter) error {
	return builtinQuote(i)
}

// === I/O ===

func builtinPrint(i *Interpreter) error {
	v := i.Pop()
	if v == nil {
		return nil
	}
	// Print strings without quotes
	if s, ok := v.(types.String); ok {
		fmt.Fprintln(i.Output, string(s))
	} else {
		fmt.Fprintln(i.Output, v.String())
	}
	return nil
}

func builtinPrintNoNL(i *Interpreter) error {
	v := i.Pop()
	if v == nil {
		return nil
	}
	if s, ok := v.(types.String); ok {
		fmt.Fprint(i.Output, string(s))
	} else {
		fmt.Fprint(i.Output, v.String())
	}
	return nil
}

func builtinNewline(i *Interpreter) error {
	fmt.Fprintln(i.Output)
	return nil
}

func builtinShowStack(i *Interpreter) error {
	fmt.Fprintln(i.Output, i.StackString())
	return nil
}

// === Error handling ===

func builtinErrQ(i *Interpreter) error {
	i.Push(types.Boolean(i.CFlag))
	return nil
}

func builtinErrCode(i *Interpreter) error {
	i.Push(types.Number(i.ARegister))
	return nil
}

func builtinClearErr(i *Interpreter) error {
	i.ClearError()
	return nil
}

// === Z flag operations ===

func builtinZQ(i *Interpreter) error {
	i.Push(types.Boolean(i.ZFlag))
	return nil
}

func builtinSetZ(i *Interpreter) error {
	i.ZFlag = true
	return nil
}

func builtinClrZ(i *Interpreter) error {
	i.ZFlag = false
	return nil
}

// === Definition (point-free style) ===

// define: [quotation] "name" define -> (adds to dictionary)
func builtinDefine(i *Interpreter) error {
	name, ok := i.PopString()
	if !ok {
		// Also accept symbol
		v := i.Pop()
		if v == nil {
			return nil
		}
		if sym, ok := v.(types.Symbol); ok {
			name = types.String(sym)
		} else {
			i.SetError(types.ErrTypeMismatch)
			return nil
		}
	}
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	i.Define(string(name), q)
	return nil
}

// undefine: "name" undefine -> (removes from dictionary)
func builtinUndefine(i *Interpreter) error {
	name, ok := i.PopString()
	if !ok {
		v := i.Pop()
		if v == nil {
			return nil
		}
		if sym, ok := v.(types.Symbol); ok {
			name = types.String(sym)
		} else {
			i.SetError(types.ErrTypeMismatch)
			return nil
		}
	}
	delete(i.Dictionary, string(name))
	return nil
}

// === List operations ===

// reverse - reverse a quotation
func builtinReverse(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	items := make([]types.Value, len(q.Items))
	for j, item := range q.Items {
		items[len(q.Items)-1-j] = item
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// nth - get nth element: [list] n nth
func builtinNth(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	idx := int(n)
	if idx < 0 || idx >= len(q.Items) {
		i.SetError(types.ErrInvalidQuotation)
		return nil
	}
	i.Push(q.Items[idx])
	return nil
}

// take - take first n elements: [list] n take
func builtinTake(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	count := int(n)
	if count > len(q.Items) {
		count = len(q.Items)
	}
	if count < 0 {
		count = 0
	}
	i.Push(&types.Quotation{Items: q.Items[:count]})
	return nil
}

// drop - drop first n elements: [list] n drop (as list op, not stack drop)
func builtinDropN(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	count := int(n)
	if count > len(q.Items) {
		count = len(q.Items)
	}
	if count < 0 {
		count = 0
	}
	i.Push(&types.Quotation{Items: q.Items[count:]})
	return nil
}

// split - split at index: [list] n split -> [first-n] [rest]
func builtinSplit(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	idx := int(n)
	if idx > len(q.Items) {
		idx = len(q.Items)
	}
	if idx < 0 {
		idx = 0
	}
	i.Push(&types.Quotation{Items: q.Items[:idx]})
	i.Push(&types.Quotation{Items: q.Items[idx:]})
	return nil
}

// zip - combine two lists: [a] [b] zip -> [[a1 b1] [a2 b2] ...]
func builtinZip(i *Interpreter) error {
	b, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	a, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	minLen := len(a.Items)
	if len(b.Items) < minLen {
		minLen = len(b.Items)
	}
	items := make([]types.Value, minLen)
	for j := 0; j < minLen; j++ {
		items[j] = &types.Quotation{Items: []types.Value{a.Items[j], b.Items[j]}}
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// zipwith - zip with combining function: [a] [b] [Q] zipwith
func builtinZipWith(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	b, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	a, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	minLen := len(a.Items)
	if len(b.Items) < minLen {
		minLen = len(b.Items)
	}
	items := make([]types.Value, 0, minLen)
	for j := 0; j < minLen; j++ {
		i.Push(a.Items[j])
		i.Push(b.Items[j])
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
		if len(i.Stack) > 0 {
			items = append(items, i.Pop())
		}
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// range - generate range: start end range -> [start start+1 ... end-1]
func builtinRange(i *Interpreter) error {
	end, ok := i.PopNumber()
	if !ok {
		return nil
	}
	start, ok := i.PopNumber()
	if !ok {
		return nil
	}
	var items []types.Value
	if start <= end {
		for n := start; n < end; n++ {
			items = append(items, n)
		}
	} else {
		for n := start; n > end; n-- {
			items = append(items, n)
		}
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// iota - generate 0..n-1: n iota -> [0 1 2 ... n-1]
func builtinIota(i *Interpreter) error {
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}
	count := int(n)
	if count < 0 {
		count = 0
	}
	items := make([]types.Value, count)
	for j := 0; j < count; j++ {
		items[j] = types.Number(j)
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// flatten - flatten one level: [[a] [b c] d] flatten -> [a b c d]
func builtinFlatten(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	var items []types.Value
	for _, item := range q.Items {
		if inner, ok := item.(*types.Quotation); ok {
			items = append(items, inner.Items...)
		} else {
			items = append(items, item)
		}
	}
	i.Push(&types.Quotation{Items: items})
	return nil
}

// any - check if any element matches: [list] [pred] any
func builtinAny(i *Interpreter) error {
	pred, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	for _, item := range list.Items {
		savedLen := len(i.Stack)
		i.Push(item)
		if err := i.ExecuteQuotation(pred); err != nil {
			return err
		}
		result := i.ZFlag
		if len(i.Stack) > savedLen {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}
		if result {
			i.ZFlag = true
			i.Push(types.Boolean(true))
			return nil
		}
	}
	i.ZFlag = false
	i.Push(types.Boolean(false))
	return nil
}

// all - check if all elements match: [list] [pred] all
func builtinAll(i *Interpreter) error {
	pred, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	for _, item := range list.Items {
		savedLen := len(i.Stack)
		i.Push(item)
		if err := i.ExecuteQuotation(pred); err != nil {
			return err
		}
		result := i.ZFlag
		if len(i.Stack) > savedLen {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}
		if !result {
			i.ZFlag = false
			i.Push(types.Boolean(false))
			return nil
		}
	}
	i.ZFlag = true
	i.Push(types.Boolean(true))
	return nil
}

// find - find first matching element: [list] [pred] find
// Pushes element and true, or false if not found
func builtinFind(i *Interpreter) error {
	pred, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	for _, item := range list.Items {
		savedLen := len(i.Stack)
		i.Push(item)
		if err := i.ExecuteQuotation(pred); err != nil {
			return err
		}
		result := i.ZFlag
		if len(i.Stack) > savedLen {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}
		if result {
			i.Push(item)
			i.ZFlag = true
			i.Push(types.Boolean(true))
			return nil
		}
	}
	i.ZFlag = false
	i.Push(types.Boolean(false))
	return nil
}

// index - find index of first match: [list] [pred] index
// Pushes index or -1 if not found
func builtinIndex(i *Interpreter) error {
	pred, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	for j, item := range list.Items {
		savedLen := len(i.Stack)
		i.Push(item)
		if err := i.ExecuteQuotation(pred); err != nil {
			return err
		}
		result := i.ZFlag
		if len(i.Stack) > savedLen {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}
		if result {
			i.ZFlag = true
			i.Push(types.Number(j))
			return nil
		}
	}
	i.ZFlag = false
	i.Push(types.Number(-1))
	return nil
}

// sort - sort list with comparator: [list] [cmp] sort
// cmp should return true if a < b: a b cmp -> bool
func builtinSort(i *Interpreter) error {
	cmp, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Copy items for sorting
	items := make([]types.Value, len(list.Items))
	copy(items, list.Items)

	// Simple insertion sort (good enough for small lists)
	for j := 1; j < len(items); j++ {
		key := items[j]
		k := j - 1
		for k >= 0 {
			// Compare items[k] and key
			i.Push(items[k])
			i.Push(key)
			if err := i.ExecuteQuotation(cmp); err != nil {
				return err
			}
			aLessB := i.ZFlag
			if len(i.Stack) > 0 {
				if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
					aLessB = bool(b)
					i.Stack = i.Stack[:len(i.Stack)-1]
				}
			}
			// If items[k] <= key, stop
			if aLessB {
				break
			}
			items[k+1] = items[k]
			k--
		}
		items[k+1] = key
	}

	i.Push(&types.Quotation{Items: items})
	return nil
}

// last - get last element of list
func builtinLast(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	if len(q.Items) == 0 {
		i.SetError(types.ErrInvalidQuotation)
		return nil
	}
	i.Push(q.Items[len(q.Items)-1])
	return nil
}

// === Math functions ===

func builtinSin(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Sin(float64(a))))
	return nil
}

func builtinCos(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Cos(float64(a))))
	return nil
}

func builtinTan(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Tan(float64(a))))
	return nil
}

func builtinAsin(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Asin(float64(a))))
	return nil
}

func builtinAcos(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Acos(float64(a))))
	return nil
}

func builtinAtan(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Atan(float64(a))))
	return nil
}

func builtinAtan2(i *Interpreter) error {
	x, ok := i.PopNumber()
	if !ok {
		return nil
	}
	y, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Atan2(float64(y), float64(x))))
	return nil
}

func builtinSqrt(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Sqrt(float64(a))))
	return nil
}

func builtinPow(i *Interpreter) error {
	exp, ok := i.PopNumber()
	if !ok {
		return nil
	}
	base, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Pow(float64(base), float64(exp))))
	return nil
}

func builtinExp(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Exp(float64(a))))
	return nil
}

func builtinLog(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Log(float64(a))))
	return nil
}

func builtinFloor(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Floor(float64(a))))
	return nil
}

func builtinCeil(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Ceil(float64(a))))
	return nil
}

func builtinRound(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Round(float64(a))))
	return nil
}

func builtinMin(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Min(float64(a), float64(b))))
	return nil
}

func builtinMax(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	i.Push(types.Number(math.Max(float64(a), float64(b))))
	return nil
}

// clamp: value min max -> clamped_value
func builtinClamp(i *Interpreter) error {
	maxVal, ok := i.PopNumber()
	if !ok {
		return nil
	}
	minVal, ok := i.PopNumber()
	if !ok {
		return nil
	}
	val, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := math.Max(float64(minVal), math.Min(float64(maxVal), float64(val)))
	i.Push(types.Number(result))
	return nil
}

// lerp: a b t -> interpolated (linear interpolation)
func builtinLerp(i *Interpreter) error {
	t, ok := i.PopNumber()
	if !ok {
		return nil
	}
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := float64(a) + (float64(b)-float64(a))*float64(t)
	i.Push(types.Number(result))
	return nil
}

func builtinSign(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	var result float64
	if a > 0 {
		result = 1
	} else if a < 0 {
		result = -1
	} else {
		result = 0
	}
	i.Push(types.Number(result))
	return nil
}

// fract: returns fractional part of number
func builtinFract(i *Interpreter) error {
	a, ok := i.PopNumber()
	if !ok {
		return nil
	}
	result := float64(a) - math.Floor(float64(a))
	i.Push(types.Number(result))
	return nil
}

// smoothstep: edge0 edge1 x -> smooth interpolation
func builtinSmoothstep(i *Interpreter) error {
	x, ok := i.PopNumber()
	if !ok {
		return nil
	}
	edge1, ok := i.PopNumber()
	if !ok {
		return nil
	}
	edge0, ok := i.PopNumber()
	if !ok {
		return nil
	}
	// Clamp x to [0, 1]
	t := (float64(x) - float64(edge0)) / (float64(edge1) - float64(edge0))
	t = math.Max(0, math.Min(1, t))
	// Smooth Hermite interpolation
	result := t * t * (3 - 2*t)
	i.Push(types.Number(result))
	return nil
}

// === Graphics functions ===

// img-new: width height -> image
func builtinImgNew(i *Interpreter) error {
	height, ok := i.PopNumber()
	if !ok {
		return nil
	}
	width, ok := i.PopNumber()
	if !ok {
		return nil
	}
	img := types.NewImage(int(width), int(height))
	i.Push(img)
	return nil
}

// img-setpixel: image x y r g b -> image
func builtinImgSetPixel(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	g, ok := i.PopNumber()
	if !ok {
		return nil
	}
	r, ok := i.PopNumber()
	if !ok {
		return nil
	}
	y, ok := i.PopNumber()
	if !ok {
		return nil
	}
	x, ok := i.PopNumber()
	if !ok {
		return nil
	}
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	// Clamp RGB to 0-255
	rr := uint8(math.Max(0, math.Min(255, float64(r))))
	gg := uint8(math.Max(0, math.Min(255, float64(g))))
	bb := uint8(math.Max(0, math.Min(255, float64(b))))
	img.SetPixel(int(x), int(y), rr, gg, bb)
	i.Push(img)
	return nil
}

// img-getpixel: image x y -> r g b
func builtinImgGetPixel(i *Interpreter) error {
	y, ok := i.PopNumber()
	if !ok {
		return nil
	}
	x, ok := i.PopNumber()
	if !ok {
		return nil
	}
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	r, g, b := img.GetPixel(int(x), int(y))
	i.Push(types.Number(r))
	i.Push(types.Number(g))
	i.Push(types.Number(b))
	return nil
}

// img-save: image filename ->
func builtinImgSave(i *Interpreter) error {
	filename, ok := i.PopString()
	if !ok {
		return nil
	}
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	file, err := os.Create(string(filename))
	if err != nil {
		i.SetError(types.ErrFileError)
		return nil
	}
	defer file.Close()
	if err := png.Encode(file, img.Img); err != nil {
		i.SetError(types.ErrImageError)
		return nil
	}
	fmt.Fprintf(i.Output, "Saved: %s\n", filename)
	return nil
}

// img-width: image -> width
func builtinImgWidth(i *Interpreter) error {
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	i.Push(types.Number(img.Width))
	return nil
}

// img-height: image -> height
func builtinImgHeight(i *Interpreter) error {
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	i.Push(types.Number(img.Height))
	return nil
}

// img-fill: image r g b -> image
func builtinImgFill(i *Interpreter) error {
	b, ok := i.PopNumber()
	if !ok {
		return nil
	}
	g, ok := i.PopNumber()
	if !ok {
		return nil
	}
	r, ok := i.PopNumber()
	if !ok {
		return nil
	}
	img, ok := i.PopImage()
	if !ok {
		return nil
	}
	rr := uint8(math.Max(0, math.Min(255, float64(r))))
	gg := uint8(math.Max(0, math.Min(255, float64(g))))
	bb := uint8(math.Max(0, math.Min(255, float64(b))))
	for y := 0; y < img.Height; y++ {
		for x := 0; x < img.Width; x++ {
			img.SetPixel(x, y, rr, gg, bb)
		}
	}
	i.Push(img)
	return nil
}

// image? - check if top is an image
func builtinIsImage(i *Interpreter) error {
	v := i.Peek()
	if v == nil {
		return nil
	}
	_, ok := v.(*types.Image)
	i.ZFlag = ok
	i.Push(types.Boolean(ok))
	return nil
}

// img-render: image [shader] -> image
// shader quotation: x y width height -> r g b
// Renders each pixel by calling shader with pixel coords
func builtinImgRender(i *Interpreter) error {
	shader, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	img, ok := i.PopImage()
	if !ok {
		return nil
	}

	width := img.Width
	height := img.Height

	for py := 0; py < height; py++ {
		for px := 0; px < width; px++ {
			// Push x, y, width, height for the shader
			i.Push(types.Number(px))
			i.Push(types.Number(py))
			i.Push(types.Number(width))
			i.Push(types.Number(height))

			// Execute shader
			if err := i.ExecuteQuotation(shader); err != nil {
				return err
			}

			// Pop r, g, b from stack
			b, ok := i.PopNumber()
			if !ok {
				return nil
			}
			g, ok := i.PopNumber()
			if !ok {
				return nil
			}
			r, ok := i.PopNumber()
			if !ok {
				return nil
			}

			// Clamp and set pixel
			rr := uint8(math.Max(0, math.Min(255, float64(r))))
			gg := uint8(math.Max(0, math.Min(255, float64(g))))
			bb := uint8(math.Max(0, math.Min(255, float64(b))))
			img.SetPixel(px, py, rr, gg, bb)
		}
	}

	i.Push(img)
	return nil
}
