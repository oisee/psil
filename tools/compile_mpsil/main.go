// compile_mpsil compiles .mpsil assembly to raw bytecode binary files.
// Output: main.bin (main bytecode) and optionally quots.bin (quotation data).
//
// Usage: go run tools/compile_mpsil/main.go examples/micro/arithmetic.mpsil
//
// The binary format for quotations:
//   quots.bin = [n_quots:u8] [offset0:u16 len0:u16] ... [body0] [body1] ...
//
// For programs without quotations, only main.bin is produced.
package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/psilLang/psil/pkg/micro"
)

func main() {
	outDir := flag.String("o", "z80/build", "Output directory")
	disasm := flag.Bool("disasm", false, "Print disassembly")
	flag.Parse()

	if flag.NArg() == 0 {
		fmt.Fprintln(os.Stderr, "Usage: compile_mpsil [-o outdir] [-disasm] <file.mpsil>")
		os.Exit(1)
	}

	for _, path := range flag.Args() {
		if err := compileFile(path, *outDir, *disasm); err != nil {
			fmt.Fprintf(os.Stderr, "Error compiling %s: %v\n", path, err)
			os.Exit(1)
		}
	}
}

func compileFile(path, outDir string, showDisasm bool) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	source := string(data)
	baseName := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))

	// Extract main code (everything outside QUOT/ENDQUOT blocks)
	mainSource := extractMain(source)

	// Assemble main code
	asm := micro.NewAssembler()
	mainCode, err := asm.Assemble(mainSource)
	if err != nil {
		return fmt.Errorf("main assembly: %w", err)
	}

	// Parse and assemble quotations
	quots := parseQuotations(source)

	if showDisasm {
		fmt.Printf("=== %s: Main Code (%d bytes) ===\n", baseName, len(mainCode))
		fmt.Print(micro.Disassemble(mainCode))
		fmt.Printf("Hex: ")
		for _, b := range mainCode {
			fmt.Printf("%02X ", b)
		}
		fmt.Println()
	}

	// Write main bytecode
	mainPath := filepath.Join(outDir, baseName+".bin")
	if err := os.WriteFile(mainPath, mainCode, 0644); err != nil {
		return fmt.Errorf("write main: %w", err)
	}
	fmt.Printf("%s: %d bytes -> %s\n", baseName, len(mainCode), mainPath)

	// Write quotations if any
	if len(quots) > 0 {
		quotData, err := buildQuotBinary(quots)
		if err != nil {
			return fmt.Errorf("quotation assembly: %w", err)
		}
		quotPath := filepath.Join(outDir, baseName+"_quots.bin")
		if err := os.WriteFile(quotPath, quotData, 0644); err != nil {
			return fmt.Errorf("write quots: %w", err)
		}
		fmt.Printf("%s: %d quotations -> %s\n", baseName, len(quots), quotPath)

		if showDisasm {
			for _, q := range quots {
				fmt.Printf("\n=== Quotation [%d] %s ===\n", q.idx, q.name)
				qasm := micro.NewAssembler()
				qcode, _ := qasm.Assemble(q.body)
				fmt.Print(micro.Disassemble(qcode))
				fmt.Printf("Hex: ")
				for _, b := range qcode {
					fmt.Printf("%02X ", b)
				}
				fmt.Println()
			}
		}
	}

	return nil
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

// buildQuotBinary builds a binary blob with quotation data.
// Format:
//   [n_quots: u8]
//   For each quotation (indexed 0..max_idx):
//     [body_len: u16 LE]
//   Then all bodies concatenated.
//
// The Z80 VM will parse this at load time to build its quotation pointer table.
func buildQuotBinary(quots []quotDef) ([]byte, error) {
	// Find max index
	maxIdx := 0
	for _, q := range quots {
		if q.idx > maxIdx {
			maxIdx = q.idx
		}
	}

	// Assemble all quotation bodies
	bodies := make([][]byte, maxIdx+1)
	for _, q := range quots {
		qasm := micro.NewAssembler()
		code, err := qasm.Assemble(q.body)
		if err != nil {
			return nil, fmt.Errorf("quotation %s: %w", q.name, err)
		}
		bodies[q.idx] = code
	}

	// Build binary
	var buf []byte
	nQuots := byte(maxIdx + 1)
	buf = append(buf, nQuots)

	// Length table
	for i := 0; i < int(nQuots); i++ {
		l := uint16(0)
		if bodies[i] != nil {
			l = uint16(len(bodies[i]))
		}
		var lb [2]byte
		binary.LittleEndian.PutUint16(lb[:], l)
		buf = append(buf, lb[:]...)
	}

	// Bodies
	for i := 0; i < int(nQuots); i++ {
		if bodies[i] != nil {
			buf = append(buf, bodies[i]...)
		}
	}

	return buf, nil
}
