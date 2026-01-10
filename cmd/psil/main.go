// PSIL - Point-free Stack-based Interpreted Language
// A concatenative functional language inspired by Joy
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/psilLang/psil/pkg/interpreter"
	"github.com/psilLang/psil/pkg/parser"
	"github.com/psilLang/psil/pkg/types"
)

var (
	flagDebug = flag.Bool("debug", false, "Enable debug mode (show flags after each command)")
	flagGas   = flag.Int("gas", 0, "Set gas limit (0 = unlimited)")
	flagQuiet = flag.Bool("quiet", false, "Quiet mode (no banner)")
)

func main() {
	flag.Parse()

	// Create interpreter
	interp := interpreter.New()
	interp.Debug = *flagDebug
	if *flagGas > 0 {
		interp.MaxGas = *flagGas
		interp.Gas = *flagGas
	}

	args := flag.Args()

	if len(args) > 0 {
		// Run file(s)
		for _, filename := range args {
			if err := runFile(interp, filename); err != nil {
				fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
	} else {
		// Interactive REPL
		runREPL(interp)
	}
}

func runFile(interp *interpreter.Interpreter, filename string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("reading %s: %w", filename, err)
	}

	return runSource(interp, string(data), filename)
}

func runSource(interp *interpreter.Interpreter, source, filename string) error {
	// Parse
	prog, err := parser.Parse(source)
	if err != nil {
		return fmt.Errorf("parse error in %s: %w", filename, err)
	}

	// Convert to runtime values
	values, definitions := prog.ToValues()

	// Add definitions to dictionary
	for name, q := range definitions {
		interp.Define(name, q)
	}

	// Execute
	if err := interp.Run(values); err != nil {
		return fmt.Errorf("runtime error in %s: %w", filename, err)
	}

	// Check for errors
	if interp.HasError() {
		return fmt.Errorf("error flag set: %s (code %d)",
			types.ErrorMessage(interp.ARegister), interp.ARegister)
	}

	return nil
}

func runREPL(interp *interpreter.Interpreter) {
	if !*flagQuiet {
		printBanner()
	}

	reader := bufio.NewReader(os.Stdin)
	multiLineBuffer := ""
	bracketDepth := 0

	for {
		// Print prompt
		if multiLineBuffer == "" {
			fmt.Print("PSIL> ")
		} else {
			fmt.Print("....> ")
		}

		// Read line
		line, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println()
			break
		}
		line = strings.TrimRight(line, "\r\n")

		// Handle special commands
		if multiLineBuffer == "" {
			if handled := handleCommand(interp, line); handled {
				continue
			}
		}

		// Track bracket depth for multi-line input
		for _, ch := range line {
			if ch == '[' {
				bracketDepth++
			} else if ch == ']' {
				bracketDepth--
			}
		}

		multiLineBuffer += line + " "

		// If brackets are balanced, execute
		if bracketDepth <= 0 {
			if strings.TrimSpace(multiLineBuffer) != "" {
				executeREPL(interp, multiLineBuffer)
			}
			multiLineBuffer = ""
			bracketDepth = 0
		}
	}
}

func handleCommand(interp *interpreter.Interpreter, line string) bool {
	trimmed := strings.TrimSpace(line)

	switch {
	case trimmed == "":
		return true

	case trimmed == ":help" || trimmed == ":h" || trimmed == ":?":
		printHelp()
		return true

	case trimmed == ":quit" || trimmed == ":q" || trimmed == ":exit":
		fmt.Println("Goodbye!")
		os.Exit(0)

	case trimmed == ":stack" || trimmed == ":s":
		fmt.Println(interp.StackString())
		return true

	case trimmed == ":flags" || trimmed == ":f":
		fmt.Println(interp.FlagsString())
		return true

	case trimmed == ":clear" || trimmed == ":c":
		interp.Reset()
		fmt.Println("Stack cleared.")
		return true

	case trimmed == ":debug" || trimmed == ":d":
		interp.Debug = !interp.Debug
		fmt.Printf("Debug mode: %v\n", interp.Debug)
		return true

	case trimmed == ":words" || trimmed == ":w":
		printWords(interp)
		return true

	case strings.HasPrefix(trimmed, ":load ") || strings.HasPrefix(trimmed, ":l "):
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			fmt.Println("Usage: :load <filename>")
			return true
		}
		if err := runFile(interp, parts[1]); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		}
		return true

	case strings.HasPrefix(trimmed, ":gas "):
		parts := strings.Fields(trimmed)
		if len(parts) < 2 {
			fmt.Printf("Current gas: %d / %d\n", interp.Gas, interp.MaxGas)
			return true
		}
		var gas int
		fmt.Sscanf(parts[1], "%d", &gas)
		interp.MaxGas = gas
		interp.Gas = gas
		fmt.Printf("Gas limit set to %d\n", gas)
		return true
	}

	return false
}

func executeREPL(interp *interpreter.Interpreter, source string) {
	// Parse
	prog, err := parser.Parse(source)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Parse error: %v\n", err)
		return
	}

	// Convert and execute
	values, definitions := prog.ToValues()

	// Add definitions
	for name, q := range definitions {
		interp.Define(name, q)
		fmt.Printf("Defined: %s\n", name)
	}

	// Execute expressions
	if err := interp.Run(values); err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
	}

	// Show status
	if interp.Debug {
		fmt.Printf("  Stack: %s\n", interp.StackString())
		fmt.Printf("  Flags: %s\n", interp.FlagsString())
	} else if interp.HasError() {
		fmt.Printf("  Error: %s (code %d)\n",
			types.ErrorMessage(interp.ARegister), interp.ARegister)
	} else if len(interp.Stack) > 0 {
		// Show top of stack
		fmt.Printf("  => %s\n", interp.Stack[len(interp.Stack)-1].String())
	}
}

func printBanner() {
	fmt.Print(`
╔═══════════════════════════════════════════════════════════╗
║  PSIL - Point-free Stack-based Interpreted Language       ║
║  A concatenative functional language inspired by Joy      ║
╠═══════════════════════════════════════════════════════════╣
║  Type :help for commands, :quit to exit                   ║
╚═══════════════════════════════════════════════════════════╝
`)
}

func printHelp() {
	fmt.Print(`
PSIL Commands:
  :help, :h, :?    Show this help
  :quit, :q        Exit PSIL
  :stack, :s       Show current stack
  :flags, :f       Show Z, C flags and A register
  :clear, :c       Clear stack and reset flags
  :debug, :d       Toggle debug mode
  :words, :w       List defined words
  :load <file>     Load and execute a file
  :gas <n>         Set gas limit (0 = unlimited)

Language Basics:
  42 3.14          Numbers (push to stack)
  "hello"          Strings (push to stack)
  true false       Booleans (push to stack)
  [ ... ]          Quotation (push code block)
  dup drop swap    Stack operations
  + - * /          Arithmetic
  < > = !=         Comparison (sets Z flag)
  ifte             [cond] [then] [else] ifte
  linrec           [P] [T] [R1] [R2] linrec
  .                Print top of stack

Example:
  DEFINE fact == [ [0 =] [drop 1] [dup 1 -] [*] linrec ].
  5 fact .
`)
}

func printWords(interp *interpreter.Interpreter) {
	fmt.Println("Defined words:")

	// Separate builtins from user definitions
	var builtins, userDefs []string

	for name, val := range interp.Dictionary {
		if _, ok := val.(*types.Builtin); ok {
			builtins = append(builtins, name)
		} else {
			userDefs = append(userDefs, name)
		}
	}

	if len(userDefs) > 0 {
		fmt.Println("\nUser definitions:")
		for _, name := range userDefs {
			fmt.Printf("  %s == %s\n", name, interp.Dictionary[name].String())
		}
	}

	fmt.Printf("\nBuiltins: %d words\n", len(builtins))
	// Print builtins in columns
	cols := 6
	for i, name := range builtins {
		fmt.Printf("%-12s", name)
		if (i+1)%cols == 0 {
			fmt.Println()
		}
	}
	if len(builtins)%cols != 0 {
		fmt.Println()
	}
}
