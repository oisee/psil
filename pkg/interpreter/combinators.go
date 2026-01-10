// Package interpreter - combinators.go contains control flow combinators
// These are the heart of PSIL's power - higher-order control structures
package interpreter

import (
	"github.com/psilLang/psil/pkg/types"
)

func init() {
	// Register combinators after interpreter is created
}

// RegisterCombinators registers all combinator operations
func (i *Interpreter) RegisterCombinators() {
	// Conditional
	i.registerBuiltin("ifte", builtinIfte)
	i.registerBuiltin("if", builtinIfThen)    // simple if
	i.registerBuiltin("ifelse", builtinIfte)  // alias
	i.registerBuiltin("branch", builtinIfte)  // alias
	i.registerBuiltin("choice", builtinChoice)

	// Recursion combinators
	i.registerBuiltin("linrec", builtinLinrec)
	i.registerBuiltin("binrec", builtinBinrec)
	i.registerBuiltin("genrec", builtinGenrec)
	i.registerBuiltin("primrec", builtinPrimrec)
	i.registerBuiltin("tailrec", builtinTailrec)

	// Iteration
	i.registerBuiltin("times", builtinTimes)
	i.registerBuiltin("while", builtinWhile)
	i.registerBuiltin("loop", builtinLoop)

	// List/Quotation combinators
	i.registerBuiltin("map", builtinMap)
	i.registerBuiltin("fold", builtinFold)
	i.registerBuiltin("filter", builtinFilter)
	i.registerBuiltin("each", builtinEach)
	i.registerBuiltin("step", builtinStep)
	i.registerBuiltin("infra", builtinInfra)
	i.registerBuiltin("cleave", builtinCleave)
	i.registerBuiltin("spread", builtinSpread)
	i.registerBuiltin("apply", builtinApply)

	// Error handling combinators
	i.registerBuiltin("onerr", builtinOnErr)
	i.registerBuiltin("try", builtinTry)
}

// === Conditional ===

// ifte - if-then-else: [cond] [then] [else] ifte
// Executes cond (non-destructively via dip-like behavior)
// If Z flag is true (or result is truthy): execute then
// Else: execute else
func builtinIfte(i *Interpreter) error {
	elseQ, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	thenQ, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	condQ, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Save stack state to restore after condition check
	savedStack := make([]types.Value, len(i.Stack))
	copy(savedStack, i.Stack)

	// Execute condition
	err := i.ExecuteQuotation(condQ)
	if err != nil {
		return err
	}

	// Get result - either from Z flag or top of stack
	result := i.ZFlag

	// Check if top of stack is a boolean
	if len(i.Stack) > 0 {
		if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
			result = bool(b)
			i.Stack = i.Stack[:len(i.Stack)-1] // pop the boolean
		}
	}

	// Restore stack (non-destructive condition evaluation)
	i.Stack = savedStack

	// Execute appropriate branch
	if result {
		return i.ExecuteQuotation(thenQ)
	}
	return i.ExecuteQuotation(elseQ)
}

// if - simple if (no else): [cond] [then] if
func builtinIfThen(i *Interpreter) error {
	thenQ, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	condQ, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Execute condition
	err := i.ExecuteQuotation(condQ)
	if err != nil {
		return err
	}

	// Check result
	result := i.ZFlag
	if len(i.Stack) > 0 {
		if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
			result = bool(b)
			i.Stack = i.Stack[:len(i.Stack)-1]
		}
	}

	if result {
		return i.ExecuteQuotation(thenQ)
	}
	return nil
}

// choice - ternary choice: a b flag choice -> a (if true) or b (if false)
func builtinChoice(i *Interpreter) error {
	flag, ok := i.PopBoolean()
	if !ok {
		return nil
	}
	b := i.Pop()
	if b == nil {
		return nil
	}
	a := i.Pop()
	if a == nil {
		return nil
	}
	if flag {
		i.Push(a)
	} else {
		i.Push(b)
	}
	return nil
}

// === Recursion Combinators ===

// linrec - linear recursion: [P] [T] [R1] [R2] linrec
// If P is true: execute T
// Else: execute R1, recurse, execute R2
func builtinLinrec(i *Interpreter) error {
	r2, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	r1, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	t, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	p, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	return linrecHelper(i, p, t, r1, r2)
}

func linrecHelper(i *Interpreter, p, t, r1, r2 *types.Quotation) error {
	// Check gas
	if !i.ConsumeGas(1) {
		return nil
	}

	// Save stack for condition
	savedStack := make([]types.Value, len(i.Stack))
	copy(savedStack, i.Stack)

	// Execute predicate
	if err := i.ExecuteQuotation(p); err != nil {
		return err
	}

	// Get result
	result := i.ZFlag
	if len(i.Stack) > len(savedStack) {
		if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
			result = bool(b)
			i.Stack = i.Stack[:len(i.Stack)-1]
		}
	}
	i.Stack = savedStack

	if result {
		// Base case: execute T
		return i.ExecuteQuotation(t)
	}

	// Recursive case: R1, recurse, R2
	if err := i.ExecuteQuotation(r1); err != nil {
		return err
	}
	if err := linrecHelper(i, p, t, r1, r2); err != nil {
		return err
	}
	return i.ExecuteQuotation(r2)
}

// binrec - binary recursion: [P] [T] [R1] [R2] binrec
// If P: T
// Else: R1 produces two values, recurse on each, R2 combines
func builtinBinrec(i *Interpreter) error {
	r2, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	r1, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	t, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	p, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	return binrecHelper(i, p, t, r1, r2)
}

func binrecHelper(i *Interpreter, p, t, r1, r2 *types.Quotation) error {
	if !i.ConsumeGas(1) {
		return nil
	}

	// Save stack
	savedStack := make([]types.Value, len(i.Stack))
	copy(savedStack, i.Stack)

	// Execute predicate
	if err := i.ExecuteQuotation(p); err != nil {
		return err
	}

	result := i.ZFlag
	if len(i.Stack) > len(savedStack) {
		if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
			result = bool(b)
			i.Stack = i.Stack[:len(i.Stack)-1]
		}
	}
	i.Stack = savedStack

	if result {
		return i.ExecuteQuotation(t)
	}

	// R1 should produce two values
	if err := i.ExecuteQuotation(r1); err != nil {
		return err
	}

	// Save second value
	second := i.Pop()
	if second == nil {
		return nil
	}

	// Recurse on first
	if err := binrecHelper(i, p, t, r1, r2); err != nil {
		return err
	}

	// Push second, recurse on it
	i.Push(second)
	if err := binrecHelper(i, p, t, r1, r2); err != nil {
		return err
	}

	// Combine with R2
	return i.ExecuteQuotation(r2)
}

// genrec - general recursion: [P] [T] [R1] [R2] genrec
// Most flexible: R2 receives a quotation that continues recursion
func builtinGenrec(i *Interpreter) error {
	r2, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	r1, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	t, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	p, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	return genrecHelper(i, p, t, r1, r2)
}

func genrecHelper(i *Interpreter, p, t, r1, r2 *types.Quotation) error {
	if !i.ConsumeGas(1) {
		return nil
	}

	// Save stack
	savedStack := make([]types.Value, len(i.Stack))
	copy(savedStack, i.Stack)

	// Execute predicate
	if err := i.ExecuteQuotation(p); err != nil {
		return err
	}

	result := i.ZFlag
	if len(i.Stack) > len(savedStack) {
		if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
			result = bool(b)
			i.Stack = i.Stack[:len(i.Stack)-1]
		}
	}
	i.Stack = savedStack

	if result {
		return i.ExecuteQuotation(t)
	}

	// Execute R1
	if err := i.ExecuteQuotation(r1); err != nil {
		return err
	}

	// Create continuation quotation that will call genrec again
	continuation := &types.Quotation{
		Items: []types.Value{
			p, t, r1, r2,
			types.Symbol("genrec"),
		},
	}
	i.Push(continuation)

	// Execute R2 with continuation on stack
	return i.ExecuteQuotation(r2)
}

// primrec - primitive recursion: init [base] [combine] primrec
// For natural number recursion
func builtinPrimrec(i *Interpreter) error {
	combine, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	base, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}

	if n <= 0 {
		return i.ExecuteQuotation(base)
	}

	// Push n-1, recurse, then combine with n
	i.Push(n - 1)
	if err := builtinPrimrec(i); err != nil {
		return err
	}
	i.Push(n)
	return i.ExecuteQuotation(combine)
}

// tailrec - tail recursive: [P] [T] [R] tailrec
// If P: T (and stop)
// Else: R (and loop back)
func builtinTailrec(i *Interpreter) error {
	r, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	t, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	p, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	for {
		if !i.ConsumeGas(1) {
			return nil
		}

		// Save stack
		savedStack := make([]types.Value, len(i.Stack))
		copy(savedStack, i.Stack)

		// Execute predicate
		if err := i.ExecuteQuotation(p); err != nil {
			return err
		}

		result := i.ZFlag
		if len(i.Stack) > len(savedStack) {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}
		i.Stack = savedStack

		if result {
			return i.ExecuteQuotation(t)
		}

		if err := i.ExecuteQuotation(r); err != nil {
			return err
		}
	}
}

// === Iteration ===

// times - repeat n times: n [Q] times
func builtinTimes(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	n, ok := i.PopNumber()
	if !ok {
		return nil
	}

	for j := 0; j < int(n); j++ {
		if !i.ConsumeGas(1) {
			return nil
		}
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
		if i.CFlag {
			break
		}
	}
	return nil
}

// while - loop while condition: [cond] [body] while
func builtinWhile(i *Interpreter) error {
	body, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	cond, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	for {
		if !i.ConsumeGas(1) {
			return nil
		}

		// Check condition
		if err := i.ExecuteQuotation(cond); err != nil {
			return err
		}

		result := i.ZFlag
		if len(i.Stack) > 0 {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				result = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}

		if !result {
			break
		}

		// Execute body
		if err := i.ExecuteQuotation(body); err != nil {
			return err
		}
		if i.CFlag {
			break
		}
	}
	return nil
}

// loop - infinite loop (until error/gas): [body] loop
func builtinLoop(i *Interpreter) error {
	body, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	for {
		if !i.ConsumeGas(1) {
			return nil
		}
		if err := i.ExecuteQuotation(body); err != nil {
			return err
		}
		if i.CFlag {
			break
		}
	}
	return nil
}

// === List/Quotation Combinators ===

// map - apply quotation to each element: [list] [Q] map
func builtinMap(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	results := make([]types.Value, 0, len(list.Items))
	for _, item := range list.Items {
		if !i.ConsumeGas(1) {
			return nil
		}
		i.Push(item)
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
		if i.CFlag {
			return nil
		}
		if len(i.Stack) > 0 {
			results = append(results, i.Pop())
		}
	}

	i.Push(&types.Quotation{Items: results})
	return nil
}

// fold - fold with accumulator: init [list] [Q] fold
// Q is called with (acc item -- newacc)
func builtinFold(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	// acc is already on stack

	for _, item := range list.Items {
		if !i.ConsumeGas(1) {
			return nil
		}
		i.Push(item)
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
		if i.CFlag {
			return nil
		}
	}
	return nil
}

// filter - keep elements matching predicate: [list] [pred] filter
func builtinFilter(i *Interpreter) error {
	pred, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	results := make([]types.Value, 0)
	for _, item := range list.Items {
		if !i.ConsumeGas(1) {
			return nil
		}

		// Save stack
		savedLen := len(i.Stack)

		i.Push(item)
		if err := i.ExecuteQuotation(pred); err != nil {
			return err
		}

		// Check result
		keep := i.ZFlag
		if len(i.Stack) > savedLen {
			if b, ok := i.Stack[len(i.Stack)-1].(types.Boolean); ok {
				keep = bool(b)
				i.Stack = i.Stack[:len(i.Stack)-1]
			}
		}

		if keep {
			results = append(results, item)
		}
	}

	i.Push(&types.Quotation{Items: results})
	return nil
}

// each - execute quotation for each element (no results): [list] [Q] each
func builtinEach(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	for _, item := range list.Items {
		if !i.ConsumeGas(1) {
			return nil
		}
		i.Push(item)
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
		if i.CFlag {
			break
		}
	}
	return nil
}

// step - like each but leaves stack alone between iterations
func builtinStep(i *Interpreter) error {
	return builtinEach(i)
}

// infra - execute quotation on a list as if it were the stack
func builtinInfra(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	list, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Save current stack
	savedStack := i.Stack

	// Use list as new stack
	i.Stack = make([]types.Value, len(list.Items))
	copy(i.Stack, list.Items)

	// Execute quotation
	err := i.ExecuteQuotation(q)

	// Capture result as new list
	result := &types.Quotation{Items: i.Stack}

	// Restore stack and push result
	i.Stack = savedStack
	i.Push(result)

	return err
}

// cleave - apply multiple quotations to same value: x [Q1] [Q2] ... cleave
func builtinCleave(i *Interpreter) error {
	// Get quotation of quotations
	qs, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Get value to cleave
	x := i.Peek()
	if x == nil {
		return nil
	}

	for _, qv := range qs.Items {
		q, ok := qv.(*types.Quotation)
		if !ok {
			continue
		}
		i.Push(x)
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
	}
	return nil
}

// spread - apply quotations to respective stack elements
func builtinSpread(i *Interpreter) error {
	qs, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	n := len(qs.Items)
	if len(i.Stack) < n {
		i.SetError(types.ErrStackUnderflow)
		return nil
	}

	// Pop n values
	values := make([]types.Value, n)
	for j := n - 1; j >= 0; j-- {
		values[j] = i.Pop()
	}

	// Apply each quotation to each value
	for j, qv := range qs.Items {
		q, ok := qv.(*types.Quotation)
		if !ok {
			i.Push(values[j])
			continue
		}
		i.Push(values[j])
		if err := i.ExecuteQuotation(q); err != nil {
			return err
		}
	}
	return nil
}

// apply - apply quotation to list elements as arguments
func builtinApply(i *Interpreter) error {
	q, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	args, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Push all arguments
	for _, arg := range args.Items {
		i.Push(arg)
	}

	return i.ExecuteQuotation(q)
}

// === Error Handling Combinators ===

// onerr - handle error: [handler] onerr
// Executes handler if C flag is set
func builtinOnErr(i *Interpreter) error {
	handler, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	if i.CFlag {
		// Clear error and execute handler
		savedErr := i.ARegister
		i.ClearError()
		i.Push(types.Number(savedErr))
		return i.ExecuteQuotation(handler)
	}
	return nil
}

// try - protected execution: [body] [handler] try
// Execute body, if error: clear and execute handler
func builtinTry(i *Interpreter) error {
	handler, ok := i.PopQuotation()
	if !ok {
		return nil
	}
	body, ok := i.PopQuotation()
	if !ok {
		return nil
	}

	// Save error state
	savedC := i.CFlag
	savedA := i.ARegister
	i.ClearError()

	// Execute body
	err := i.ExecuteQuotation(body)

	if i.CFlag {
		// Error occurred - execute handler
		errCode := i.ARegister
		i.ClearError()
		i.Push(types.Number(errCode))
		return i.ExecuteQuotation(handler)
	}

	// Restore previous error state if no new error
	if savedC && !i.CFlag {
		i.CFlag = savedC
		i.ARegister = savedA
	}

	return err
}
