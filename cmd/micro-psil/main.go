// micro-psil is a minimal bytecode VM for PSIL.
// Designed for easy Z80/6502 implementation.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/psilLang/psil/pkg/micro"
)

func main() {
	debug := flag.Bool("debug", false, "Enable debug output")
	disasm := flag.Bool("disasm", false, "Disassemble instead of run")
	gas := flag.Int("gas", 0, "Gas limit (0 = unlimited)")
	flag.Parse()

	args := flag.Args()

	if len(args) == 0 {
		repl(*debug, *gas)
		return
	}

	// Load and run file
	data, err := os.ReadFile(args[0])
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	source := string(data)

	// Check if it's assembly (text) or raw bytecode
	if isBytecode(data) {
		// Raw bytecode
		if *disasm {
			fmt.Print(micro.Disassemble(data))
			return
		}
		runBytecode(data, *debug, *gas)
	} else {
		// Assembly text
		code, quots, err := assembleSource(source)
		if err != nil {
			fmt.Fprintf(os.Stderr, "Assembly error: %v\n", err)
			os.Exit(1)
		}

		if *disasm {
			fmt.Println("=== Main ===")
			fmt.Print(micro.Disassemble(code))
			for name, idx := range quots {
				fmt.Printf("\n=== [%s] (idx=%d) ===\n", name, idx)
			}
			return
		}

		vm := micro.New()
		vm.Debug = *debug
		if *gas > 0 {
			vm.MaxGas = *gas
			vm.Gas = *gas
		}

		// Load quotations
		for _, q := range parseQuotations(source) {
			asm := micro.NewAssembler()
			qcode, err := asm.Assemble(q.body)
			if err != nil {
				fmt.Fprintf(os.Stderr, "Quotation %s error: %v\n", q.name, err)
				os.Exit(1)
			}
			vm.DefineQuot(q.idx, qcode)
		}

		vm.Load(code)
		if err := vm.Run(); err != nil {
			fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
			os.Exit(1)
		}

		fmt.Println()
		fmt.Println("Stack:", vm.StackDump())
	}
}

func isBytecode(data []byte) bool {
	// Heuristic: if starts with printable text, it's assembly
	if len(data) == 0 {
		return false
	}
	for i := 0; i < len(data) && i < 10; i++ {
		c := data[i]
		if c == '\n' || c == '\r' || c == '\t' || c == ' ' {
			continue
		}
		if c >= 0x20 && c <= 0x7E {
			continue
		}
		return true // Found non-printable = bytecode
	}
	return false
}

func assembleSource(source string) ([]byte, map[string]int, error) {
	// Extract main code (everything before first QUOT or DEFINE)
	mainCode := extractMain(source)

	asm := micro.NewAssembler()
	code, err := asm.Assemble(mainCode)
	if err != nil {
		return nil, nil, err
	}

	return code, asm.GetQuotations(), nil
}

func extractMain(source string) string {
	lines := strings.Split(source, "\n")
	var mainLines []string
	inQuot := false

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "QUOT ") || strings.HasPrefix(trimmed, "DEFINE ") {
			inQuot = true
			continue
		}
		if strings.HasPrefix(trimmed, "ENDQUOT") || strings.HasPrefix(trimmed, "ENDDEF") {
			inQuot = false
			continue
		}
		if !inQuot {
			mainLines = append(mainLines, line)
		}
	}

	return strings.Join(mainLines, "\n")
}

type quotDef struct {
	name string
	idx  int
	body string
}

func parseQuotations(source string) []quotDef {
	var quots []quotDef
	lines := strings.Split(source, "\n")
	var current *quotDef
	var body []string

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		if strings.HasPrefix(trimmed, "QUOT ") {
			parts := strings.Fields(trimmed)
			if len(parts) >= 2 {
				idx := len(quots)
				if len(parts) >= 3 {
					fmt.Sscanf(parts[2], "%d", &idx)
				}
				current = &quotDef{name: parts[1], idx: idx}
				body = nil
			}
			continue
		}

		if strings.HasPrefix(trimmed, "ENDQUOT") {
			if current != nil {
				current.body = strings.Join(body, "\n")
				quots = append(quots, *current)
				current = nil
			}
			continue
		}

		if current != nil {
			body = append(body, line)
		}
	}

	return quots
}

func runBytecode(code []byte, debug bool, gas int) {
	vm := micro.New()
	vm.Debug = debug
	if gas > 0 {
		vm.MaxGas = gas
		vm.Gas = gas
	}
	vm.Load(code)

	if err := vm.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Runtime error: %v\n", err)
		os.Exit(1)
	}

	fmt.Println()
	fmt.Println("Stack:", vm.StackDump())
}

func repl(debug bool, gas int) {
	fmt.Println("micro-PSIL VM")
	fmt.Println("Type 'help' for commands, 'quit' to exit")
	fmt.Println()

	vm := micro.New()
	vm.Debug = debug
	if gas > 0 {
		vm.MaxGas = gas
		vm.Gas = gas
	}

	scanner := bufio.NewScanner(os.Stdin)

	for {
		fmt.Print("μ> ")
		if !scanner.Scan() {
			break
		}

		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue
		}

		switch line {
		case "quit", "exit":
			return
		case "help":
			printHelp()
		case "stack":
			fmt.Println(vm.StackDump())
		case "clear":
			vm.Reset()
			fmt.Println("Cleared")
		case "debug":
			vm.Debug = !vm.Debug
			fmt.Printf("Debug: %v\n", vm.Debug)
		default:
			// Try to assemble and run
			asm := micro.NewAssembler()
			code, err := asm.Assemble(line)
			if err != nil {
				fmt.Printf("Error: %v\n", err)
				continue
			}

			if vm.Debug {
				fmt.Println("Bytecode:", micro.Disassemble(code))
			}

			vm.Load(code)
			vm.Halted = false
			if err := vm.Run(); err != nil {
				fmt.Printf("Error: %v\n", err)
			}

			fmt.Println("->", vm.StackDump())
		}
	}
}

func printHelp() {
	fmt.Print(`Commands:
  quit     - Exit REPL
  stack    - Show stack
  clear    - Clear stack and reset
  debug    - Toggle debug mode
  help     - Show this help

Instructions:
  Numbers: 0-31 (inline), push.b N (byte), push.w N (word)
  Stack:   dup drop swap over rot dup2 depth clear
  Math:    + - * / mod neg inc dec (or: add sub mul div 1+ 1-)
  Compare: = < > (or: eq lt gt)
  Logic:   and or not
  Control: exec ifte loop halt
  Memory:  @ ! (load store)
  I/O:     print . call 0 (newline)

Symbols (prefixed with '):
  'health 'energy 'fear 'anger 'hunger 'enemy 'friend etc.

Quotations: [0] [1] ... [31] for inline, [name] for named

Example:
  5 3 + .           ; prints 8
  10 dup * .        ; prints 100
  'health @ .       ; prints health value
`)
}
