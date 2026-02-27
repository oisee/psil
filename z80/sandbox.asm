; ============================================================================
; NPC Sandbox — Z80 Scheduler + Tick Loop (128K Spectrum)
; ============================================================================
;
; Build:  sjasmplus z80/sandbox.asm --raw=z80/build/sandbox.bin
;
; Run:    mzx --run z80/build/sandbox.bin@8000 --console-io --frames DI:HALT
;
; Memory map:
;   $5B00-$5EFF  World grid (32×32 = 1024 bytes)
;   $5F00-$5F15  Ring0 buffer (11 slots × 2 bytes = 22 bytes)
;   $5F16-$5F1D  Ring1 buffer (4 slots × 2 bytes = 8 bytes)
;   $5F20-$5FFF  NPC table (max 16 NPCs × 14 bytes = 224 bytes)
;   $8000-$9FFF  VM code + scheduler (this file includes VM)
;   $C000-$DFFF  NPC genomes (16 × 64 = 1024 bytes, page 1)
;
; NPC table entry (14 bytes):
;   +0  ID        (1 byte, 0 = dead/empty)
;   +1  X         (1 byte)
;   +2  Y         (1 byte)
;   +3  Health    (1 byte)
;   +4  Energy    (1 byte)
;   +5  Age low   (1 byte)
;   +6  Age high  (1 byte)
;   +7  Hunger    (1 byte)
;   +8  Food eaten(1 byte)
;   +9  Fitness lo(1 byte)
;   +10 Fitness hi(1 byte)
;   +11 Genome len(1 byte)
;   +12 Genome ptr lo (1 byte) — offset into genome bank
;   +13 Genome ptr hi (1 byte)
; ============================================================================

    ORG $8000

; Jump to sandbox entry (VM routines follow, then sandbox code)
    JP sandbox_entry

; --- Constants ---
WORLD_SIZE  EQU 32
MAX_NPCS    EQU 16
NPC_SIZE    EQU 14
GAS_LIMIT   EQU 200
FOOD_MAX    EQU 32
TICK_MAX    EQU 500
EVOLVE_EVERY EQU 128          ; must be power of 2 for AND mask

; --- Memory layout ---
WORLD_GRID  EQU $5B00
RING0_BUF   EQU $5F00
RING1_BUF   EQU $5F16
NPC_TABLE   EQU $5F20
GENOME_BANK EQU $C000       ; Page 1

; Include VM as library (skips entry point and ORG)
    DEFINE VM_LIB_MODE
    INCLUDE "micro_psil_vm.asm"

; ============================================================================
; Sandbox entry point
; ============================================================================

sandbox_entry:
    DI
    LD SP, $FF00

    ; Clear world grid
    LD HL, WORLD_GRID
    LD DE, WORLD_GRID + 1
    LD BC, 1023
    LD (HL), 0
    LDIR

    ; Clear NPC table
    LD HL, NPC_TABLE
    LD DE, NPC_TABLE + 1
    LD BC, (MAX_NPCS * NPC_SIZE) - 1
    LD (HL), 0
    LDIR

    ; Initialize NPCs with random genomes
    CALL init_npcs

    ; Seed food
    CALL seed_food

    ; Print banner
    LD HL, str_start
    CALL print_str

    ; Main tick loop
    LD HL, 0
    LD (tick_count), HL

.tick_loop:
    ; For each living NPC: sense, think, act, decay
    LD IX, NPC_TABLE
    LD B, MAX_NPCS

.npc_loop:
    PUSH BC

    ; Skip dead NPCs
    LD A, (IX+0)           ; ID
    OR A
    JP Z, .next_npc

    ; 1. Sense
    CALL fill_ring0

    ; 2. Think: run NPC genome through VM
    CALL run_brain

    ; 3. Act: apply Ring1 outputs
    CALL apply_actions

    ; 4. Decay: energy--, health if energy=0
    LD A, (IX+4)           ; energy
    OR A
    JR Z, .no_energy
    DEC A
    LD (IX+4), A
    JR .decay_done
.no_energy:
    LD A, (IX+3)           ; health
    SUB 5
    JR NC, .hp_ok
    XOR A                  ; dead
.hp_ok:
    LD (IX+3), A
    OR A
    JR NZ, .decay_done
    ; NPC died — clear tile
    CALL clear_npc_tile
    LD (IX+0), 0           ; mark dead

.decay_done:
    ; Increment age
    LD L, (IX+5)
    LD H, (IX+6)
    INC HL
    LD (IX+5), L
    LD (IX+6), H

    ; Increment hunger
    LD A, (IX+7)
    INC A
    LD (IX+7), A

    ; Score fitness = age + food_eaten*10 + health
    LD L, (IX+5)
    LD H, (IX+6)           ; HL = age
    LD A, (IX+8)           ; food eaten
    LD D, 0
    LD E, A
    ; food * 10
    SLA E : RL D            ; *2
    PUSH DE                 ; save *2
    SLA E : RL D            ; *4
    SLA E : RL D            ; *8
    POP BC
    EX DE, HL
    ADD HL, BC              ; *8 + *2 = *10
    EX DE, HL               ; DE = food*10
    ADD HL, DE              ; HL = age + food*10
    LD A, (IX+3)
    LD E, A
    LD D, 0
    ADD HL, DE              ; HL = age + food*10 + health
    LD (IX+9), L
    LD (IX+10), H

.next_npc:
    POP BC
    LD DE, NPC_SIZE
    ADD IX, DE
    DEC B
    JP NZ, .npc_loop

    ; Respawn food
    CALL respawn_food

    ; Increment tick
    LD HL, (tick_count)
    INC HL
    LD (tick_count), HL

    ; Check evolution + stats trigger (every EVOLVE_EVERY ticks)
    LD A, L
    AND EVOLVE_EVERY - 1   ; works if EVOLVE_EVERY is power of 2
    OR A
    JR NZ, .no_evolve
    CALL evolve_step
    CALL print_stats
.no_evolve:

    ; Check tick limit (reload HL since calls above may corrupt it)
    LD HL, (tick_count)
    LD DE, TICK_MAX
    OR A
    SBC HL, DE
    JP C, .tick_loop

    ; Done
    LD HL, str_done
    CALL print_str

    DI
    HALT

; ============================================================================
; fill_ring0: Fill Ring0 sensor buffer from world state
; IX = NPC table entry
; ============================================================================
fill_ring0:
    ; Slot 0: self ID
    LD A, (IX+0)
    LD HL, RING0_BUF
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 1: health
    INC HL
    LD A, (IX+3)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 2: energy
    INC HL
    LD A, (IX+4)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 3: hunger
    INC HL
    LD A, (IX+7)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 4: fear (nearest NPC distance, simplified)
    INC HL
    LD A, 31              ; placeholder — far
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 5: food distance (simplified — scan nearby)
    INC HL
    LD A, 31
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 6: danger
    INC HL
    LD (HL), 0
    INC HL
    LD (HL), 0
    ; Slot 7: near
    INC HL
    LD A, 31
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 8: X
    INC HL
    LD A, (IX+1)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 9: Y
    INC HL
    LD A, (IX+2)
    LD (HL), A
    INC HL
    LD (HL), 0
    ; Slot 10: day (tick mod 256)
    INC HL
    LD A, (tick_count)
    LD (HL), A
    INC HL
    LD (HL), 0
    RET

; ============================================================================
; run_brain: Load NPC genome and run VM
; IX = NPC table entry
; ============================================================================
run_brain:
    ; Suppress print during brain execution (redirect output)
    LD A, 1
    LD (vm_mute), A

    ; Set up VM state
    LD HL, VM_STACK
    LD (vm_sp), HL

    ; Clear VM memory and copy Ring0 into memory slots 0-10
    LD HL, RING0_BUF
    LD DE, VM_MEM
    LD BC, 22              ; 11 slots × 2 bytes
    LDIR

    ; Clear Ring1 slots in VM memory (slots 64-67)
    LD HL, VM_MEM + 128    ; slot 64 = byte offset 128
    LD (HL), 0
    LD DE, VM_MEM + 129
    LD BC, 7
    LDIR

    ; Set gas
    LD HL, GAS_LIMIT
    LD (vm_gas), HL

    ; Load genome pointer
    LD L, (IX+12)
    LD H, (IX+13)
    LD (bc_pc), HL

    ; Clear ret flag
    XOR A
    LD (vm_retf), A

    ; Run VM
    CALL vm_run

    ; Unmute
    XOR A
    LD (vm_mute), A

    ; Copy Ring1 outputs from VM memory (slots 64-67) to Ring1 buffer
    LD HL, VM_MEM + 128
    LD DE, RING1_BUF
    LD BC, 8
    LDIR

    RET

; ============================================================================
; apply_actions: Read Ring1 buffer and apply to world
; IX = NPC table entry
; ============================================================================
apply_actions:
    ; Read move direction
    LD HL, RING1_BUF
    LD A, (HL)             ; Ring1[0] = move dir
    ; Compute new x,y
    LD B, (IX+1)           ; current X
    LD C, (IX+2)           ; current Y
    CP 1 : JR Z, .move_n
    CP 2 : JR Z, .move_e
    CP 3 : JR Z, .move_s
    CP 4 : JR Z, .move_w
    JR .no_move
.move_n:
    DEC C
    JR .try_move
.move_e:
    INC B
    JR .try_move
.move_s:
    INC C
    JR .try_move
.move_w:
    DEC B
.try_move:
    ; Bounds check
    LD A, B
    CP WORLD_SIZE
    JR NC, .no_move
    LD A, C
    CP WORLD_SIZE
    JR NC, .no_move
    ; Check destination tile is empty (no occupant)
    PUSH BC
    CALL get_tile          ; A = tile at (B,C)
    POP BC
    AND $F0                ; occupant nibble
    JR NZ, .no_move
    ; Move: clear old tile, set new tile
    CALL clear_npc_tile
    LD (IX+1), B
    LD (IX+2), C
    CALL set_npc_tile
.no_move:

    ; Read action
    LD HL, RING1_BUF + 2   ; Ring1[1] = action
    LD A, (HL)
    CP 1 : JR Z, .act_eat
    JR .act_done

.act_eat:
    ; Try eating food at current or adjacent tiles
    LD B, (IX+1)
    LD C, (IX+2)
    CALL try_eat
    JR C, .act_done
    ; Try north
    LD B, (IX+1)
    LD C, (IX+2)
    DEC C
    CALL try_eat
    JR C, .act_done
    ; Try east
    LD B, (IX+1)
    LD C, (IX+2)
    INC B
    CALL try_eat
    JR C, .act_done
    ; Try south
    LD B, (IX+1)
    LD C, (IX+2)
    INC C
    CALL try_eat
    JR C, .act_done
    ; Try west
    LD B, (IX+1)
    LD C, (IX+2)
    DEC B
    CALL try_eat

.act_done:
    RET

; ============================================================================
; try_eat: Try to eat food at (B,C). Sets carry if eaten.
; IX = NPC table entry
; ============================================================================
try_eat:
    LD A, B
    CP WORLD_SIZE
    JR NC, .te_fail
    LD A, C
    CP WORLD_SIZE
    JR NC, .te_fail
    PUSH BC
    CALL get_tile          ; A = tile at (B,C)
    POP BC
    AND $0F                ; type nibble
    CP 2                   ; TileFood = 2
    JR NZ, .te_fail
    ; Clear food tile
    PUSH BC
    CALL tile_addr         ; HL = addr of tile (B,C)
    LD A, (HL)
    AND $F0                ; keep occupant, clear type to 0 (empty)
    LD (HL), A
    POP BC
    ; Boost energy (+30, cap 200)
    LD A, (IX+4)
    ADD A, 30
    CP 200
    JR C, .te_ecap
    LD A, 200
.te_ecap:
    LD (IX+4), A
    ; Boost health (+5, cap 100)
    LD A, (IX+3)
    ADD A, 5
    CP 100
    JR C, .te_hcap
    LD A, 100
.te_hcap:
    LD (IX+3), A
    ; Increment food eaten
    INC (IX+8)
    ; Reset hunger
    LD (IX+7), 0
    SCF                    ; set carry = success
    RET
.te_fail:
    OR A                   ; clear carry
    RET

; ============================================================================
; Tile helpers
; ============================================================================

; tile_addr: (B,C) → HL = address in WORLD_GRID
; B = X, C = Y
tile_addr:
    LD H, 0
    LD L, C
    ; L = Y, multiply by 32 (shift left 5)
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL             ; HL = Y * 32
    LD D, 0
    LD E, B
    ADD HL, DE             ; HL = Y*32 + X
    LD DE, WORLD_GRID
    ADD HL, DE
    RET

; get_tile: A = tile at (B,C)
get_tile:
    PUSH HL
    PUSH DE
    CALL tile_addr
    LD A, (HL)
    POP DE
    POP HL
    RET

; set_tile: Write A to tile at (B,C)
set_tile:
    PUSH HL
    PUSH DE
    CALL tile_addr
    LD (HL), A
    POP DE
    POP HL
    RET

; clear_npc_tile: Clear occupant at NPC's position
; IX = NPC
clear_npc_tile:
    PUSH BC
    LD B, (IX+1)
    LD C, (IX+2)
    CALL tile_addr
    LD A, (HL)
    AND $0F                ; keep type, clear occupant
    LD (HL), A
    POP BC
    RET

; set_npc_tile: Set occupant at NPC's position
; IX = NPC
set_npc_tile:
    PUSH BC
    LD B, (IX+1)
    LD C, (IX+2)
    CALL tile_addr
    LD A, (HL)
    AND $0F                ; keep type
    LD C, (IX+0)           ; ID
    SLA C
    SLA C
    SLA C
    SLA C                  ; shift ID to high nibble
    OR C
    LD (HL), A
    POP BC
    RET

; ============================================================================
; Food respawn
; ============================================================================
respawn_food:
    ; Simple LFSR random, try to spawn one food item
    CALL lfsr_next
    AND $1F                ; 0-31
    LD B, A
    CALL lfsr_next
    AND $1F
    LD C, A
    CALL get_tile
    OR A                   ; empty tile? (type=0, occupant=0)
    RET NZ
    LD A, 2                ; TileFood
    JP set_tile

; ============================================================================
; LFSR pseudo-random (16-bit Galois)
; ============================================================================
lfsr_next:
    LD HL, (lfsr_state)
    LD A, L
    RRCA
    JR NC, .ln_ns
    LD A, H
    XOR $B4
    LD H, A
    LD A, L
    XOR $00
    LD L, A
.ln_ns:
    SRL H
    RR L
    LD (lfsr_state), HL
    LD A, L
    RET

lfsr_state: DW $ACE1       ; seed

; ============================================================================
; Init NPCs: Create MAX_NPCS NPCs with tiny seed genomes
; ============================================================================
init_npcs:
    LD IX, NPC_TABLE
    LD A, 1
    LD (npc_id_ctr), A     ; NPC ID counter in memory (not register)
    LD HL, GENOME_BANK
    LD B, MAX_NPCS

.in_loop:
    PUSH BC
    PUSH HL

    ; Set NPC fields
    LD A, (npc_id_ctr)
    LD (IX+0), A           ; ID
    ; Random position
    CALL lfsr_next
    AND $1F
    LD (IX+1), A           ; X
    CALL lfsr_next
    AND $1F
    LD (IX+2), A           ; Y
    LD (IX+3), 100         ; health
    LD (IX+4), 100         ; energy
    LD (IX+5), 0           ; age lo
    LD (IX+6), 0           ; age hi
    LD (IX+7), 0           ; hunger
    LD (IX+8), 0           ; food eaten
    LD (IX+9), 0           ; fitness lo
    LD (IX+10), 0          ; fitness hi

    ; Genome: 16-byte simple genome
    LD (IX+11), 16         ; genome length
    POP HL
    LD (IX+12), L          ; genome ptr lo
    LD (IX+13), H          ; genome ptr hi

    ; Write seed genome: push dir → r1w 0 → push 1 → r1w 1 → halt + nop padding
    CALL lfsr_next
    AND $03                ; 0-3
    INC A                  ; 1-4 (valid directions)
    ADD A, $20             ; SmallNum opcode
    LD (HL), A             ; push dir
    INC HL
    LD (HL), $8C           ; OpRing1W
    INC HL
    LD (HL), 0             ; slot 0 (move)
    INC HL
    LD (HL), $21           ; push 1 (SmallNum 1)
    INC HL
    LD (HL), $8C           ; OpRing1W
    INC HL
    LD (HL), 1             ; slot 1 (action=eat)
    INC HL
    LD (HL), $F0           ; halt
    INC HL
    ; Pad to 16 bytes with halt (not nop, to be safe)
    LD A, $F0              ; halt
    PUSH BC
    LD B, 9
.pad_loop:
    LD (HL), A
    INC HL
    DJNZ .pad_loop
    POP BC

    ; Place NPC on tile
    CALL set_npc_tile

    ; Advance to next NPC
    LD DE, NPC_SIZE
    ADD IX, DE
    LD A, (npc_id_ctr)
    INC A
    LD (npc_id_ctr), A
    POP BC
    DEC B
    JP NZ, .in_loop
    RET

npc_id_ctr: DB 0

; ============================================================================
; Seed food: Place initial food items
; ============================================================================
seed_food:
    LD B, WORLD_SIZE       ; place ~32 food items
.sf_loop:
    PUSH BC
    CALL lfsr_next
    AND $1F
    LD B, A
    CALL lfsr_next
    AND $1F
    LD C, A
    CALL get_tile
    OR A
    JR NZ, .sf_skip
    LD A, 2                ; TileFood
    CALL set_tile
.sf_skip:
    POP BC
    DJNZ .sf_loop
    RET

; ============================================================================
; GA: Simple evolution (tournament-2, point mutation)
; ============================================================================
    INCLUDE "ga.asm"

; ============================================================================
; Print stats
; ============================================================================
print_stats:
    ; Print "T=" tick count
    LD A, 'T'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD DE, (tick_count)
    CALL pr_s16
    LD A, ' '
    OUT ($23), A

    ; Count alive NPCs
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
    LD C, 0                ; alive count
.ps_cnt:
    LD A, (IX+0)
    OR A
    JR Z, .ps_sk
    INC C
.ps_sk:
    LD DE, NPC_SIZE
    ADD IX, DE
    DJNZ .ps_cnt

    LD A, 'A'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD E, C
    LD D, 0
    CALL pr_s16

    LD A, 10               ; newline
    OUT ($23), A
    RET

; ============================================================================
; print_str: Print null-terminated string at HL
; ============================================================================
print_str:
    LD A, (HL)
    OR A
    RET Z
    OUT ($23), A
    INC HL
    JR print_str

; ============================================================================
; Data
; ============================================================================
tick_count: DW 0

str_start: DB "NPC Sandbox Z80", 10, 0
str_done:  DB "Done", 10, 0

sandbox_end:
