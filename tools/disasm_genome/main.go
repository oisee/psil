package main

import (
	"encoding/hex"
	"fmt"
	"os"

	"github.com/psilLang/psil/pkg/micro"
)

var sensorNames = map[byte]string{
	0: "self", 1: "health", 2: "energy", 3: "hunger", 4: "fear",
	5: "food_dist", 6: "danger", 7: "near_dist", 8: "x", 9: "y",
	10: "day", 11: "count", 12: "near_id", 13: "food_dir",
	14: "my_gold", 15: "my_item", 16: "item_dist", 17: "near_trust",
	18: "near_dir", 19: "item_dir", 20: "rng", 21: "stress",
	22: "my_gas", 23: "on_forge", 24: "my_age", 25: "taught",
	26: "biome", 27: "tile_type", 28: "similarity", 29: "tile_ahead",
	30: "cooldown",
}

var ring1Names = map[byte]string{0: "move", 1: "action", 2: "target"}

var moveArgs = map[byte]string{
	1: "N", 2: "E", 3: "S", 4: "W", 5: "→food", 6: "→npc", 7: "→item",
}

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: disasm_genome <hex>")
		os.Exit(1)
	}
	code, err := hex.DecodeString(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "bad hex:", err)
		os.Exit(1)
	}

	pc := 0
	for pc < len(code) {
		op := code[pc]
		addr := fmt.Sprintf("%03d", pc)

		switch {
		case op == micro.OpHalt:
			fmt.Printf("%s  halt\n", addr)
			pc++
		case op == micro.OpYield:
			fmt.Printf("%s  yield\n", addr)
			pc++
		case op == micro.OpEnd:
			fmt.Printf("%s  end\n", addr)
			pc++
		case micro.IsSmallNum(op):
			fmt.Printf("%s  push %d\n", addr, micro.SmallNumValue(op))
			pc++
		case micro.IsInlineSym(op):
			fmt.Printf("%s  sym 0x%02x\n", addr, op)
			pc++
		case micro.IsInlineQuot(op):
			fmt.Printf("%s  quot[%d]\n", addr, micro.InlineQuotIndex(op))
			pc++
		case op == micro.OpRing0R && pc+1 < len(code):
			slot := code[pc+1]
			name := sensorNames[slot]
			if name == "" {
				name = fmt.Sprintf("?%d", slot)
			}
			fmt.Printf("%s  r0@ %s\t\t; sensor[%d]\n", addr, name, slot)
			pc += 2
		case op == micro.OpRing1W && pc+1 < len(code):
			slot := code[pc+1]
			name := ring1Names[slot]
			if name == "" {
				name = fmt.Sprintf("?%d", slot)
			}
			fmt.Printf("%s  r1! %s\t\t; ring1[%d]\n", addr, name, slot)
			pc += 2
		case op == micro.OpJumpNZ && pc+1 < len(code):
			fmt.Printf("%s  jnz +%d\t\t; → %03d\n", addr, code[pc+1], pc+2+int(code[pc+1]))
			pc += 2
		case op == micro.OpJumpZ && pc+1 < len(code):
			fmt.Printf("%s  jz +%d\t\t; → %03d\n", addr, code[pc+1], pc+2+int(code[pc+1]))
			pc += 2
		case op == micro.OpJump && pc+1 < len(code):
			fmt.Printf("%s  jmp +%d\t\t; → %03d\n", addr, code[pc+1], pc+2+int(code[pc+1]))
			pc += 2
		case op == micro.OpJumpBack && pc+1 < len(code):
			fmt.Printf("%s  jmp -%d\t\t; → %03d\n", addr, code[pc+1], pc+2-int(code[pc+1]))
			pc += 2
		case op == micro.OpActMove && pc+1 < len(code):
			arg := code[pc+1]
			dir := moveArgs[arg]
			if dir == "" {
				dir = fmt.Sprintf("%d", arg)
			}
			fmt.Printf("%s  act.move %s\n", addr, dir)
			pc += 2
		case op == micro.OpActAttack && pc+1 < len(code):
			fmt.Printf("%s  act.attack\n", addr)
			pc += 2
		case op == micro.OpActHeal && pc+1 < len(code):
			fmt.Printf("%s  act.heal\n", addr)
			pc += 2
		case op == micro.OpActEat && pc+1 < len(code):
			fmt.Printf("%s  act.eat\n", addr)
			pc += 2
		case op == micro.OpActHarvest && pc+1 < len(code):
			fmt.Printf("%s  act.harvest\n", addr)
			pc += 2
		case op == micro.OpActTerraform && pc+1 < len(code):
			fmt.Printf("%s  act.terraform\n", addr)
			pc += 2
		case op == micro.OpActShare && pc+1 < len(code):
			fmt.Printf("%s  act.share\n", addr)
			pc += 2
		case op == micro.OpActTrade && pc+1 < len(code):
			fmt.Printf("%s  act.trade\n", addr)
			pc += 2
		case op == micro.OpActCraft && pc+1 < len(code):
			fmt.Printf("%s  act.craft\n", addr)
			pc += 2
		case op == micro.OpPushByte && pc+1 < len(code):
			fmt.Printf("%s  push.b %d\n", addr, code[pc+1])
			pc += 2
		case micro.Is2ByteOp(op) && pc+1 < len(code):
			fmt.Printf("%s  %s %d\n", addr, micro.OpName(op), code[pc+1])
			pc += 2
		case micro.Is3ByteOp(op) && pc+2 < len(code):
			val := int(code[pc+1])<<8 | int(code[pc+2])
			fmt.Printf("%s  %s %d\n", addr, micro.OpName(op), val)
			pc += 3
		case micro.IsVarLenOp(op) && pc+1 < len(code):
			length := int(code[pc+1])
			fmt.Printf("%s  %s [%d bytes]\n", addr, micro.OpName(op), length)
			pc += 2 + length
		default:
			name := micro.OpName(op)
			fmt.Printf("%s  %s\n", addr, name)
			pc++
		}
	}
}
