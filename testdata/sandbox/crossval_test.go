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

func TestThorinGenomeCrossValidation(t *testing.T) {
	// thorin genome: danger > 5 → flee N; food < 5 → go S + eat; else idle
	genome := []byte{
		0x8A, 0x06, 0x25, 0x0D, 0x88, 0x07, // r0@ 6, push 5, >, jnz 7
		0x8A, 0x05, 0x25, 0x0C, 0x88, 0x05, // r0@ 5, push 5, <, jnz 5
		0xF1,                                 // yield (idle)
		0x21, 0x8C, 0x00, 0xF1,              // push 1, r1! 0, yield (flee N)
		0x23, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1, // push 3, r1! 0, push 1, r1! 1, yield (eat S)
	}

	t.Run("flee", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(6, 8) // danger = 8 (> 5)
		vm.MemWrite(5, 3) // food = 3
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Thorin(danger=8): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 1 {
			t.Errorf("expected move=1 (North/flee), got %d", move)
		}
		if action != 0 {
			t.Errorf("expected action=0 (idle), got %d", action)
		}
	})

	t.Run("eat", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(6, 2) // danger = 2 (≤ 5)
		vm.MemWrite(5, 3) // food = 3 (< 5)
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Thorin(danger=2,food=3): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 3 {
			t.Errorf("expected move=3 (South/eat), got %d", move)
		}
		if action != 1 {
			t.Errorf("expected action=1 (eat), got %d", action)
		}
	})

	t.Run("idle", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(6, 0) // danger = 0 (≤ 5)
		vm.MemWrite(5, 10) // food = 10 (≥ 5)
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Thorin(danger=0,food=10): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 0 {
			t.Errorf("expected move=0 (idle), got %d", move)
		}
		if action != 0 {
			t.Errorf("expected action=0 (idle), got %d", action)
		}
	})
}

func TestWarriorGenomeCrossValidation(t *testing.T) {
	// warrior genome: hunger > 20 → eat S; fear < 3 → attack E; else wander
	genome := []byte{
		0x8A, 0x03, 0x34, 0x0D, 0x88, 0x0F, // r0@ 3, push 20, >, jnz 15
		0x8A, 0x04, 0x23, 0x0C, 0x88, 0x10, // r0@ 4, push 3, <, jnz 16
		0x8A, 0x0A, 0x24, 0x0A, 0x21, 0x06, 0x8C, 0x00, 0xF1, // r0@ 10, push 4, mod, push 1, +, r1! 0, yield (wander)
		0x23, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1, // push 3, r1! 0, push 1, r1! 1, yield (eat S)
		0x22, 0x8C, 0x00, 0x22, 0x8C, 0x01, 0xF1, // push 2, r1! 0, push 2, r1! 1, yield (attack E)
	}

	t.Run("eat", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(3, 25)  // hunger = 25 (> 20)
		vm.MemWrite(4, 10)  // fear = 10
		vm.MemWrite(10, 7)  // day = 7
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Warrior(hunger=25): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 3 {
			t.Errorf("expected move=3 (South/eat), got %d", move)
		}
		if action != 1 {
			t.Errorf("expected action=1 (eat), got %d", action)
		}
	})

	t.Run("attack", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(3, 5)   // hunger = 5 (≤ 20)
		vm.MemWrite(4, 2)   // fear = 2 (< 3, enemy close)
		vm.MemWrite(10, 7)  // day = 7
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Warrior(fear=2): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 2 {
			t.Errorf("expected move=2 (East/attack), got %d", move)
		}
		if action != 2 {
			t.Errorf("expected action=2 (attack), got %d", action)
		}
	})

	t.Run("wander", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(3, 5)   // hunger = 5 (≤ 20)
		vm.MemWrite(4, 10)  // fear = 10 (≥ 3)
		vm.MemWrite(10, 7)  // day = 7 → 7 mod 4 + 1 = 4 (West)
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Warrior(day=7): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 4 {
			t.Errorf("expected move=4 (West, 7%%4+1), got %d", move)
		}
		if action != 0 {
			t.Errorf("expected action=0 (idle/wander), got %d", action)
		}
	})
}

func TestTraderGenomeCrossValidation(t *testing.T) {
	// Goal-based trader genome:
	// r0@ 15, push 0, >, jnz +8,
	//   forage: r0@ 13(food_dir), r1! 0, push 1, r1! 1, yield
	//   trade:  r0@ 18(near_dir), r1! 0, push 4, r1! 1, r0@ 12(near_id), r1! 2, yield
	genome := []byte{
		0x8A, 0x0F, 0x20, 0x0D, 0x88, 0x08, // r0@ 15, push 0, >, jnz +8
		0x8A, 0x0D, 0x8C, 0x00, 0x21, 0x8C, 0x01, 0xF1, // forage block (8 bytes)
		0x8A, 0x12, 0x8C, 0x00, 0x24, 0x8C, 0x01, 0x8A, 0x0C, 0x8C, 0x02, 0xF1, // trade block
	}

	t.Run("forage", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(13, 3) // food direction = 3 (South)
		vm.MemWrite(15, 0) // my_item = 0 (no item)
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		fmt.Printf("Trader(item=0,food_dir=3): Ring1[move]=%d Ring1[action]=%d\n", move, action)

		if move != 3 {
			t.Errorf("expected move=3 (South/toward food), got %d", move)
		}
		if action != 1 {
			t.Errorf("expected action=1 (eat), got %d", action)
		}
	})

	t.Run("trade", func(t *testing.T) {
		vm := micro.New()
		vm.Output = io.Discard
		vm.MaxGas = 200
		vm.Gas = 200
		vm.MemWrite(12, 7) // nearest NPC ID = 7
		vm.MemWrite(15, 2) // my_item = 2 (holding tool)
		vm.MemWrite(18, 1) // nearest NPC direction = 1 (North)
		vm.Load(genome)
		vm.Run()

		move := vm.MemRead(64 + 0)
		action := vm.MemRead(64 + 1)
		target := vm.MemRead(64 + 2)
		fmt.Printf("Trader(item=2,near_dir=1,near_id=7): Ring1[move]=%d Ring1[action]=%d Ring1[target]=%d\n", move, action, target)

		if move != 1 {
			t.Errorf("expected move=1 (North/toward NPC), got %d", move)
		}
		if action != 4 {
			t.Errorf("expected action=4 (trade), got %d", action)
		}
		if target != 7 {
			t.Errorf("expected target=7 (nearest NPC ID), got %d", target)
		}
	})
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
