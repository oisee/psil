package sandbox_test

import (
	"fmt"
	"io"
	"testing"

	"github.com/psilLang/psil/pkg/micro"
)

func TestForagerGenomeCrossValidation(t *testing.T) {
	// forager genome: r0@ 5, push 3, r1! 0, push 1, r1! 1, yield
	genome := []byte{0x8A, 0x05, 0x23, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1}

	vm := micro.New()
	vm.Output = io.Discard
	vm.MaxGas = 200
	vm.Gas = 200

	// Set Ring0 slot 5 (food distance) = 3
	vm.MemWrite(5, 3)

	vm.Load(genome)
	vm.Run()

	move := vm.MemRead(64 + 0)
	action := vm.MemRead(64 + 1)

	fmt.Printf("Forager: Ring1[move]=%d Ring1[action]=%d halted=%v\n", move, action, vm.Halted)

	if move != 3 {
		t.Errorf("expected move=3 (South), got %d", move)
	}
	if action != 1 {
		t.Errorf("expected action=1 (eat), got %d", action)
	}
	if !vm.Halted {
		t.Error("VM should be halted after yield")
	}
}

func TestRandomGenomeCrossValidation(t *testing.T) {
	// random genome: r0@ 10, push 4, mod, push 1, +, r1! 0, push 1, r1! 1, yield
	genome := []byte{0x8A, 0x0A, 0x24, 0x0A, 0x21, 0x06, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1}

	vm := micro.New()
	vm.Output = io.Discard
	vm.MaxGas = 200
	vm.Gas = 200

	// Set Ring0 slot 10 (day) = 7
	vm.MemWrite(10, 7)

	vm.Load(genome)
	vm.Run()

	move := vm.MemRead(64 + 0)
	action := vm.MemRead(64 + 1)

	// day=7, 7 mod 4 = 3, 3 + 1 = 4 (West)
	fmt.Printf("Random(day=7): Ring1[move]=%d Ring1[action]=%d\n", move, action)

	if move != 4 {
		t.Errorf("expected move=4 (West, 7%%4+1), got %d", move)
	}
	if action != 1 {
		t.Errorf("expected action=1 (eat), got %d", action)
	}
}
