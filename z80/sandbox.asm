; ============================================================================
; NPC Sandbox — Z80 Scheduler + Tick Loop (128K Spectrum)
; ============================================================================
;
; Build:  sjasmplus z80/sandbox.asm --raw=z80/build/sandbox.bin
;
; Run:    mzx --run z80/build/sandbox.bin@8000 --console-io --frames DI:HALT
;
; 128K Memory layout:
;   Bank 5 (always $4000-$7FFF):
;     $5B00-$5DFF  Tile grid: 32×24 = 768 bytes (tile type, 1 byte/cell)
;     $5E00-$60FF  Occupancy grid: 768 bytes (NPC index, 0=empty)
;     $6100-$63FF  Biome grid: 768 bytes (reserved for Phase 3)
;     $6400-$643D  Ring0 buffer: 31 slots × 2 bytes = 62 bytes
;     $6440-$6447  Ring1 buffer: 4 slots × 2 bytes = 8 bytes
;     $6500-$65FF  NPC table: 16 NPCs × 16 bytes = 256 bytes
;     $6600-$6FFF  Scratch / GA temp
;     $7000-$70FF  Popcount LUT (reserved for Phase 3)
;
;   Bank 2 (always $8000-$BFFF):
;     $8000        JP sandbox_entry
;     $8003+       VM code (INCLUDE), then scheduler, GA, data
;     SP = $BFFE   Stack (top of Bank 2, never paged out)
;
;   Bank 0 (paged at $C000-$FFFF):
;     $C000-$FFFF  NPC genomes: up to 128 × 128 bytes
;
; NPC table entry (16 bytes):
;   +0   ID         1 byte  (0=dead, 1-255=alive)
;   +1   X          1 byte
;   +2   Y          1 byte
;   +3   Health     1 byte  (0-100)
;   +4   Energy     1 byte  (0-200)
;   +5   Age lo     1 byte
;   +6   Age hi     1 byte
;   +7   Hunger     1 byte
;   +8   FoodEaten  1 byte
;   +9   Fitness lo 1 byte
;   +10  Fitness hi 1 byte
;   +11  GenomeLen  1 byte
;   +12  GenomePtr lo 1 byte
;   +13  GenomePtr hi 1 byte
;   +14  Item       1 byte  (0=none, 2=tool, 3=weapon, 4=treasure,
;                             5=crystal, 6=shield, 7=compass)
;   +15  Flags      1 byte  (bits 0-1: lastDir)
; ============================================================================

    ORG $8000

; Jump to sandbox entry (VM routines follow, then sandbox code)
    JP sandbox_entry

; --- Constants ---
WORLD_SIZE_X EQU 32
WORLD_SIZE_Y EQU 24
WORLD_CELLS  EQU WORLD_SIZE_X * WORLD_SIZE_Y   ; 768
MAX_NPCS    EQU 16
NPC_SIZE    EQU 16              ; power of 2 for fast idx*16
GENOME_MAX  EQU 128             ; max genome length per NPC
GAS_LIMIT   EQU 50
FOOD_INIT   EQU 64
TICK_MAX    EQU 256
EVOLVE_EVERY EQU 16             ; must be power of 2 for AND mask

; Tile types
TILE_EMPTY  EQU 0
TILE_WALL   EQU 1
TILE_FOOD   EQU 2
TILE_ITEM   EQU 4               ; items are types 4-7

; Action IDs (match jump table order)
ACT_IDLE    EQU 0
ACT_EAT     EQU 1
ACT_ATTACK  EQU 2
ACT_SHARE   EQU 3
ACT_TRADE   EQU 4
ACT_CRAFT   EQU 5
ACT_TEACH   EQU 6
ACT_HEAL    EQU 7
ACT_HARVEST EQU 8
ACT_TERRAFORM EQU 9
NUM_ACTIONS EQU 10

; --- Memory layout (Bank 5) ---
TILE_GRID   EQU $5B00           ; 768 bytes: tile type per cell
OCC_GRID    EQU $5E00           ; 768 bytes: NPC index per cell (0=empty)
BIOME_GRID  EQU $6100           ; 768 bytes: biome type (Phase 3)
RING0_BUF   EQU $6400           ; 31 slots × 2 bytes = 62 bytes
RING1_BUF   EQU $6440           ; 4 slots × 2 bytes = 8 bytes
NPC_TABLE   EQU $6500           ; 16 × 16 = 256 bytes
GA_SCRATCH  EQU $6600           ; scratch area for GA crossover
TRADE_TABLE EQU $6700           ; 16 bytes: trade intent per NPC

; --- Memory layout (Bank 0, paged at $C000) ---
GENOME_BANK EQU $C000

; Include VM as library (skips entry point and ORG)
    DEFINE VM_LIB_MODE
    INCLUDE "micro_psil_vm.asm"

; ============================================================================
; Sandbox entry point
; ============================================================================

sandbox_entry:
    DI
    LD SP, $BFFE                ; top of Bank 2 (always mapped)

    ; Page Bank 0 at $C000 for genomes
    LD A, $10                   ; bank 0 + ROM select bit 4
    LD BC, $7FFD
    OUT (C), A

    ; Clear tile grid (768 bytes)
    LD HL, TILE_GRID
    LD DE, TILE_GRID + 1
    LD BC, WORLD_CELLS - 1
    LD (HL), TILE_EMPTY
    LDIR

    ; Clear occupancy grid (768 bytes)
    LD HL, OCC_GRID
    LD DE, OCC_GRID + 1
    LD BC, WORLD_CELLS - 1
    LD (HL), 0
    LDIR

    ; Clear NPC table (256 bytes)
    LD HL, NPC_TABLE
    LD DE, NPC_TABLE + 1
    LD BC, (MAX_NPCS * NPC_SIZE) - 1
    LD (HL), 0
    LDIR

    ; Clear trade table
    LD HL, TRADE_TABLE
    LD DE, TRADE_TABLE + 1
    LD BC, MAX_NPCS - 1
    LD (HL), 0
    LDIR

    ; Generate biome grid via WFC
    CALL generate_biomes

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

; ============================================================================
; Main tick loop
; ============================================================================
tick_loop:
    ; For each living NPC: sense, think, act, decay
    LD IX, NPC_TABLE
    LD A, MAX_NPCS
    LD (npc_loop_ctr), A

.npc_loop:
    ; Skip dead NPCs
    LD A, (IX+0)               ; ID
    OR A
    JP Z, .next_npc

    ; 1. Sense: fill Ring0 from 5×5 neighborhood
    CALL fill_ring0

    ; 2. Think: run NPC genome through VM
    CALL run_brain

    ; 3. Act: apply Ring1 outputs (9-action dispatch)
    CALL apply_actions

    ; 4. Decay: energy--, health if energy=0
    LD A, (IX+4)               ; energy
    OR A
    JR Z, .no_energy
    DEC A
    LD (IX+4), A
    JR .decay_done
.no_energy:
    LD A, (IX+3)               ; health
    SUB 5
    JR NC, .hp_ok
    XOR A                      ; dead
.hp_ok:
    LD (IX+3), A
    OR A
    JR NZ, .decay_done
    ; NPC died — clear occupancy, drop item
    CALL clear_npc_occ
    ; Drop item to tile if holding one
    LD A, (IX+14)
    OR A
    JR Z, .no_drop
    LD B, (IX+1)
    LD C, (IX+2)
    CALL tile_addr
    LD A, (IX+14)
    ADD A, 2                   ; item type offset (item 2→tile 4, etc.)
    LD (HL), A
    LD (IX+14), 0
.no_drop:
    LD (IX+0), 0               ; mark dead

.decay_done:
    ; Increment age
    LD L, (IX+5)
    LD H, (IX+6)
    INC HL
    LD (IX+5), L
    LD (IX+6), H

    ; Increment hunger (cap at 255)
    LD A, (IX+7)
    CP 255
    JR Z, .hunger_cap
    INC A
    LD (IX+7), A
.hunger_cap:

    ; Score fitness = age + food_eaten*10 + health + (item!=0)*50
    LD L, (IX+5)
    LD H, (IX+6)               ; HL = age
    LD A, (IX+8)               ; food eaten
    LD D, 0
    LD E, A
    ; food * 10 = food*8 + food*2
    SLA E : RL D               ; *2
    PUSH DE                    ; save *2
    SLA E : RL D               ; *4
    SLA E : RL D               ; *8
    POP BC
    EX DE, HL
    ADD HL, BC                 ; *8 + *2 = *10
    EX DE, HL                  ; DE = food*10
    ADD HL, DE                 ; HL = age + food*10
    LD A, (IX+3)               ; health
    LD E, A
    LD D, 0
    ADD HL, DE                 ; + health
    LD A, (IX+14)              ; item
    OR A
    JR Z, .no_item_bonus
    LD DE, 50
    ADD HL, DE                 ; + 50 if holding item
.no_item_bonus:
    LD (IX+9), L
    LD (IX+10), H

.next_npc:
    LD DE, NPC_SIZE
    ADD IX, DE
    LD A, (npc_loop_ctr)
    DEC A
    LD (npc_loop_ctr), A
    JP NZ, .npc_loop

    ; Respawn food (2 per tick)
    CALL respawn_food
    CALL respawn_food

    ; Increment tick
    LD HL, (tick_count)
    INC HL
    LD (tick_count), HL

    ; Check evolution + stats trigger (every EVOLVE_EVERY ticks)
    LD A, L
    AND EVOLVE_EVERY - 1       ; works if EVOLVE_EVERY is power of 2
    OR A
    JR NZ, .no_evolve
    CALL evolve_step
    CALL print_stats
.no_evolve:

    ; Check tick limit
    LD HL, (tick_count)
    LD DE, TICK_MAX
    OR A
    SBC HL, DE
    JP C, tick_loop

    ; Done
    LD HL, str_done
    CALL print_str

    DI
    HALT

npc_loop_ctr: DB 0

; ============================================================================
; fill_ring0: Fill Ring0 sensor buffer from 5×5 neighborhood scan
; IX = NPC table entry
; Populates 31 sensor slots (2 bytes each) at RING0_BUF
; ============================================================================
fill_ring0:
    ; First clear all 31 slots to 0
    LD HL, RING0_BUF
    LD DE, RING0_BUF + 1
    LD BC, 61                  ; 31*2 - 1
    LD (HL), 0
    LDIR

    ; Slot 0: self ID
    LD A, (IX+0)
    LD (RING0_BUF + 0), A
    ; Slot 1: health
    LD A, (IX+3)
    LD (RING0_BUF + 2), A
    ; Slot 2: energy
    LD A, (IX+4)
    LD (RING0_BUF + 4), A
    ; Slot 3: hunger
    LD A, (IX+7)
    LD (RING0_BUF + 6), A
    ; Slot 8: X
    LD A, (IX+1)
    LD (RING0_BUF + 16), A
    ; Slot 9: Y
    LD A, (IX+2)
    LD (RING0_BUF + 18), A
    ; Slot 10: day (tick mod 256)
    LD A, (tick_count)
    LD (RING0_BUF + 20), A
    ; Slot 15: my_item
    LD A, (IX+14)
    LD (RING0_BUF + 30), A
    ; Slot 20: rng
    CALL lfsr_next
    LD (RING0_BUF + 40), A

    ; Initialize search distances to 31 (far)
    LD A, 31
    LD (RING0_BUF + 8), A     ; Slot 4: fear (nearest NPC dist) → will be overwritten
    LD (RING0_BUF + 10), A    ; Slot 5: food_dist
    LD (RING0_BUF + 14), A    ; Slot 7: near_dist (nearest NPC)
    LD (RING0_BUF + 32), A    ; Slot 16: item_dist

    ; 5×5 neighborhood scan
    ; For each (dx,dy) in {-2..+2}×{-2..+2}, check tile + occupancy
    LD A, (IX+2)               ; center Y
    SUB 2
    LD (scan_y), A             ; start Y = centerY - 2

    LD C, 5                    ; dy counter
.scan_row:
    LD A, (IX+1)               ; center X
    SUB 2
    LD (scan_x), A             ; start X = centerX - 2

    LD B, 5                    ; dx counter
.scan_col:
    PUSH BC

    ; Bounds check
    LD A, (scan_x)
    CP WORLD_SIZE_X
    JP NC, .scan_skip          ; unsigned: <0 wraps to >=32
    LD A, (scan_y)
    CP WORLD_SIZE_Y
    JP NC, .scan_skip

    ; Compute Manhattan distance = |dx| + |dy|
    ; dx = scan_x - npc_x, dy = scan_y - npc_y
    LD A, (scan_x)
    SUB (IX+1)                 ; dx (signed)
    JP P, .dx_pos
    NEG
.dx_pos:
    LD D, A                    ; D = |dx|
    LD A, (scan_y)
    SUB (IX+2)                 ; dy (signed)
    JP P, .dy_pos
    NEG
.dy_pos:
    ADD A, D                   ; A = manhattan distance
    LD (scan_dist), A

    ; Skip self (distance 0)
    OR A
    JR Z, .scan_skip

    ; Compute direction: sign(dx),sign(dy) → 1=N,2=E,3=S,4=W
    CALL calc_scan_dir         ; A = direction 1-4

    ; Read tile type at (scan_x, scan_y)
    LD A, (scan_x)
    LD B, A
    LD A, (scan_y)
    LD C, A
    PUSH BC
    CALL tile_addr             ; HL = tile grid addr
    LD A, (HL)                 ; A = tile type
    POP BC
    LD (scan_tile), A

    ; Check occupancy
    PUSH HL
    CALL occ_addr              ; HL = occupancy grid addr
    LD A, (HL)                 ; A = NPC index (0=empty)
    POP HL
    LD (scan_occ), A

    ; --- Update nearest food ---
    LD A, (scan_tile)
    CP TILE_FOOD
    JR NZ, .not_food
    LD A, (scan_dist)
    LD HL, RING0_BUF + 10     ; slot 5: food_dist
    CP (HL)
    JR NC, .not_food           ; not closer
    LD (HL), A                 ; update food_dist
    CALL calc_scan_dir
    LD (RING0_BUF + 26), A    ; slot 13: food_dir
.not_food:

    ; --- Update nearest NPC ---
    LD A, (scan_occ)
    OR A
    JR Z, .not_npc
    LD A, (scan_dist)
    LD HL, RING0_BUF + 14     ; slot 7: near_dist
    CP (HL)
    JR NC, .not_npc            ; not closer
    LD (HL), A                 ; update near_dist
    LD (RING0_BUF + 8), A     ; slot 4: fear = same as near_dist
    LD A, (scan_occ)
    LD (RING0_BUF + 24), A    ; slot 12: near_id
    CALL calc_scan_dir
    LD (RING0_BUF + 36), A    ; slot 18: near_dir
.not_npc:

    ; --- Update nearest item ---
    LD A, (scan_tile)
    CP TILE_ITEM
    JR C, .not_item            ; tile type < 4 → not an item
    LD A, (scan_dist)
    LD HL, RING0_BUF + 32     ; slot 16: item_dist
    CP (HL)
    JR NC, .not_item
    LD (HL), A
    CALL calc_scan_dir
    LD (RING0_BUF + 38), A    ; slot 19: item_dir
.not_item:

.scan_skip:
    POP BC

    ; Advance X
    LD A, (scan_x)
    INC A
    LD (scan_x), A
    DEC B
    JP NZ, .scan_col

    ; Advance Y
    LD A, (scan_y)
    INC A
    LD (scan_y), A
    DEC C
    JP NZ, .scan_row

    ; Slot 27: current tile type
    LD B, (IX+1)
    LD C, (IX+2)
    CALL get_tile
    LD (RING0_BUF + 54), A    ; slot 27

    RET

; calc_scan_dir: Compute direction from center to (scan_x,scan_y)
; Returns A = 1(N), 2(E), 3(S), 4(W). Tie-break: vertical wins.
calc_scan_dir:
    LD A, (scan_y)
    SUB (IX+2)                 ; dy
    JR Z, .csd_horiz
    JP M, .csd_north           ; dy < 0 → north
    LD A, 3                    ; south
    RET
.csd_north:
    LD A, 1                    ; north
    RET
.csd_horiz:
    LD A, (scan_x)
    SUB (IX+1)                 ; dx
    JP M, .csd_west
    LD A, 2                    ; east (or zero, shouldn't happen — self filtered)
    RET
.csd_west:
    LD A, 4
    RET

scan_x:    DB 0
scan_y:    DB 0
scan_dist: DB 0
scan_tile: DB 0
scan_occ:  DB 0

; ============================================================================
; run_brain: Load NPC genome and run VM
; IX = NPC table entry
; ============================================================================
run_brain:
    ; Suppress print during brain execution
    LD A, 1
    LD (vm_mute), A

    ; Set up VM state
    LD HL, VM_STACK
    LD (vm_sp), HL

    ; Copy Ring0 into VM memory slots 0-30 (31 slots × 2 bytes = 62 bytes)
    LD HL, RING0_BUF
    LD DE, VM_MEM
    LD BC, 62
    LDIR

    ; Clear Ring1 slots in VM memory (slots 64-67)
    LD HL, VM_MEM + 128        ; slot 64 = byte offset 128
    LD (HL), 0
    LD DE, VM_MEM + 129
    LD BC, 7
    LDIR

    ; Set gas (crystal item: +50 bonus)
    LD HL, GAS_LIMIT
    LD A, (IX+14)
    CP 5                       ; crystal?
    JR NZ, .gas_no_crystal
    LD DE, 50
    ADD HL, DE
.gas_no_crystal:
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
; Ring1[0] = move direction (1-4, or 5=→food, 6=→NPC, 7=→item)
; Ring1[1] = action ID (0-9)
; Ring1[2] = action arg
; Ring1[3] = reserved
; ============================================================================
apply_actions:
    ; --- Movement ---
    LD A, (RING1_BUF)          ; Ring1[0] = move dir
    LD B, (IX+1)               ; current X
    LD C, (IX+2)               ; current Y

    ; Resolve smart directions (5=→food, 6=→NPC, 7=→item)
    CP 5
    JR Z, .dir_food
    CP 6
    JR Z, .dir_npc
    CP 7
    JR Z, .dir_item
    JR .dir_resolved

.dir_food:
    LD A, (RING0_BUF + 26)    ; food_dir
    JR .dir_resolved
.dir_npc:
    LD A, (RING0_BUF + 36)    ; near_dir
    JR .dir_resolved
.dir_item:
    LD A, (RING0_BUF + 38)    ; item_dir

.dir_resolved:
    ; A = direction: 1=N, 2=E, 3=S, 4=W
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
    ; Store last direction in flags
    LD D, A
    LD A, (IX+15)
    AND $FC                    ; clear bits 0-1
    DEC D                     ; dir 1-4 → 0-3
    OR D
    LD (IX+15), A

    ; Bounds check
    LD A, B
    CP WORLD_SIZE_X
    JR NC, .no_move
    LD A, C
    CP WORLD_SIZE_Y
    JR NC, .no_move

    ; Check occupancy at destination
    PUSH BC
    CALL occ_at                ; A = occupant at (B,C)
    POP BC
    OR A
    JR NZ, .no_move            ; occupied

    ; Move: clear old occupancy, set new
    CALL clear_npc_occ
    LD (IX+1), B
    LD (IX+2), C
    CALL set_npc_occ

    ; Pick up item if on item tile
    LD A, (IX+14)              ; already holding?
    OR A
    JR NZ, .no_move            ; can't pick up if holding
    PUSH BC
    CALL tile_addr             ; HL = tile addr at new pos
    LD A, (HL)
    CP TILE_ITEM
    JR C, .no_pickup           ; not an item
    ; Pick up: store item, clear tile
    SUB 2                      ; tile type → item ID (tile 4→item 2, etc.)
    LD (IX+14), A
    LD (HL), TILE_EMPTY
.no_pickup:
    POP BC

.no_move:

    ; --- Action dispatch via jump table ---
    LD A, (RING1_BUF + 2)     ; Ring1[1] = action ID
    CP NUM_ACTIONS
    JR NC, act_done            ; invalid action → idle

    ; Jump table lookup
    ADD A, A                   ; ×2 for word table
    LD HL, action_table
    LD E, A
    LD D, 0
    ADD HL, DE
    LD E, (HL)
    INC HL
    LD D, (HL)
    EX DE, HL
    JP (HL)                    ; jump to action handler

act_done:
    RET

action_table:
    DW act_done                ; 0 = idle
    DW act_eat                 ; 1 = eat
    DW act_attack              ; 2 = attack
    DW act_share               ; 3 = share
    DW act_trade               ; 4 = trade
    DW act_craft               ; 5 = craft
    DW act_teach               ; 6 = teach
    DW act_heal                ; 7 = heal
    DW act_harvest             ; 8 = harvest
    DW act_terraform           ; 9 = terraform

; ============================================================================
; Action handlers
; IX = NPC table entry
; ============================================================================

; --- act_eat: eat food at current or adjacent cell ---
act_eat:
    LD B, (IX+1)
    LD C, (IX+2)
    CALL try_eat
    JP C, act_done
    ; Try N
    LD B, (IX+1) : LD C, (IX+2) : DEC C
    CALL try_eat
    JP C, act_done
    ; Try E
    LD B, (IX+1) : LD C, (IX+2) : INC B
    CALL try_eat
    JP C, act_done
    ; Try S
    LD B, (IX+1) : LD C, (IX+2) : INC C
    CALL try_eat
    JP C, act_done
    ; Try W
    LD B, (IX+1) : LD C, (IX+2) : DEC B
    CALL try_eat
    JP act_done

; --- act_attack: deal 5 damage to nearest adjacent NPC (weapon: +5) ---
act_attack:
    LD A, (IX+4)               ; energy check
    CP 10
    JP C, act_done
    ; Find adjacent NPC using near_dir
    LD A, (RING0_BUF + 14)    ; near_dist
    CP 2                       ; must be adjacent (dist <= 1)
    JP NC, act_done
    ; Get target NPC index from occupancy
    LD A, (RING0_BUF + 24)    ; near_id (slot 12)
    OR A
    JP Z, act_done
    ; Find target in NPC table
    CALL find_npc_by_id        ; IY = target NPC
    JP Z, act_done             ; not found
    ; Compute damage: 5 base + 5 if weapon
    LD B, 5                    ; base damage
    LD A, (IX+14)
    CP 3                       ; weapon?
    JR NZ, .atk_no_wpn
    LD B, 10                   ; 5 + 5 weapon bonus
.atk_no_wpn:
    ; Check target shield: reduce damage by 5
    LD A, (IY+14)
    CP 6                       ; shield?
    JR NZ, .atk_no_shld
    LD A, B
    SUB 5
    JR NC, .atk_shld_ok
    XOR A
.atk_shld_ok:
    LD B, A
.atk_no_shld:
    ; Deal damage
    LD A, (IY+3)               ; target health
    SUB B
    JR NC, .atk_hp_ok
    XOR A
.atk_hp_ok:
    LD (IY+3), A
    ; Track stat
    LD HL, (stat_attack)
    INC HL
    LD (stat_attack), HL
    ; Cost energy
    LD A, (IX+4)
    SUB 10
    LD (IX+4), A
    ; If target died, steal item
    LD A, (IY+3)
    OR A
    JP NZ, act_done
    ; Target died
    LD A, (IY+14)              ; target's item
    OR A
    JR Z, .atk_no_steal
    LD B, (IX+14)              ; do we already hold?
    LD A, B
    OR A
    JR NZ, .atk_no_steal
    LD A, (IY+14)
    LD (IX+14), A              ; steal item
    LD (IY+14), 0
.atk_no_steal:
    ; Clear target from occupancy
    PUSH IX
    PUSH IY
    POP IX
    CALL clear_npc_occ
    POP IX
    LD (IY+0), 0              ; mark dead
    JP act_done

; --- act_heal: heal adjacent NPC +5 HP, cost 8 energy ---
act_heal:
    LD A, (IX+4)
    CP 8
    JP C, act_done
    LD A, (RING0_BUF + 14)    ; near_dist
    CP 2
    JP NC, act_done
    LD A, (RING0_BUF + 24)    ; near_id
    OR A
    JP Z, act_done
    CALL find_npc_by_id
    JP Z, act_done
    LD A, (IY+3)
    ADD A, 5
    CP 100
    JR C, .heal_cap
    LD A, 100
.heal_cap:
    LD (IY+3), A
    LD A, (IX+4)
    SUB 8
    LD (IX+4), A
    JP act_done

; --- act_harvest: gain energy, biome-aware + item bonus ---
; Clearing: +30, Forest: +20, Mountain: +10, Swamp: +15 (50% chance -10HP),
; Village: +25, River: +30
act_harvest:
    LD A, (IX+4)
    CP 5
    JP C, act_done
    ; Look up biome at NPC position
    PUSH IX
    LD B, (IX+1)
    LD C, (IX+2)
    ; Read biome grid
    LD H, 0
    LD L, C
    ADD HL, HL : ADD HL, HL : ADD HL, HL : ADD HL, HL : ADD HL, HL
    LD D, 0
    LD E, B
    ADD HL, DE
    LD DE, BIOME_GRID
    ADD HL, DE
    LD A, (HL)
    POP IX
    ; Biome → base energy via table
    LD HL, harvest_energy_table
    LD E, A
    LD D, 0
    ADD HL, DE
    LD B, (HL)                 ; B = base harvest energy
    ; Swamp poison check (biome 3): 50% chance -10 HP
    LD A, E                    ; biome index
    CP 3                       ; swamp?
    JR NZ, .harv_no_swamp
    PUSH BC
    CALL lfsr_next
    POP BC
    BIT 0, A                  ; 50% chance
    JR Z, .harv_no_swamp
    LD A, (IX+3)
    SUB 10
    JR NC, .harv_swamp_ok
    XOR A
.harv_swamp_ok:
    LD (IX+3), A
.harv_no_swamp:
    ; Item bonus: tool(2)→+10, compass(7)→+20
    LD A, (IX+14)
    CP 2
    JR NZ, .harv_not_tool
    LD A, B
    ADD A, 10
    LD B, A
    JR .harv_apply
.harv_not_tool:
    CP 7
    JR NZ, .harv_apply
    LD A, B
    ADD A, 20
    LD B, A
.harv_apply:
    LD A, (IX+4)
    ADD A, B
    CP 200
    JR C, .harv_cap
    LD A, 200
.harv_cap:
    LD (IX+4), A
    ; Cost 5 energy
    LD A, (IX+4)
    SUB 5
    LD (IX+4), A
    ; Track stat
    LD HL, (stat_harvest)
    INC HL
    LD (stat_harvest), HL
    JP act_done

; Biome harvest energy: indexed by biome type 0-6
harvest_energy_table:
    DB 30                      ; 0 Clearing: +30
    DB 20                      ; 1 Forest: +20
    DB 10                      ; 2 Mountain: +10
    DB 15                      ; 3 Swamp: +15 (+ poison risk)
    DB 25                      ; 4 Village: +25
    DB 30                      ; 5 River: +30
    DB 15                      ; 6 Bridge: +15

; --- act_terraform: place food on empty tile, cost 30 energy ---
act_terraform:
    LD A, (IX+4)
    CP 30
    JP C, act_done
    LD B, (IX+1)
    LD C, (IX+2)
    CALL get_tile
    OR A
    JP NZ, act_done  ; not empty
    LD A, TILE_FOOD
    LD B, (IX+1)
    LD C, (IX+2)
    CALL set_tile
    LD A, (IX+4)
    SUB 30
    LD (IX+4), A
    JP act_done

; --- act_share: transfer 10 energy to adjacent NPC ---
act_share:
    LD A, (IX+4)
    CP 10
    JP C, act_done
    LD A, (RING0_BUF + 14)
    CP 2
    JP NC, act_done
    LD A, (RING0_BUF + 24)
    OR A
    JP Z, act_done
    CALL find_npc_by_id
    JP Z, act_done
    LD A, (IY+4)
    ADD A, 10
    CP 200
    JR C, .share_cap
    LD A, 200
.share_cap:
    LD (IY+4), A
    LD A, (IX+4)
    SUB 10
    LD (IX+4), A
    JP act_done

; --- act_trade: record trade intent (bilateral resolution end-of-tick) ---
act_trade:
    ; Simple version: just swap items with adjacent NPC if both want to trade
    LD A, (RING0_BUF + 14)
    CP 2
    JP NC, act_done
    LD A, (RING0_BUF + 24)
    OR A
    JP Z, act_done
    ; Record intent: store target ID
    LD HL, TRADE_TABLE
    LD A, (IX+0)               ; my ID
    DEC A                      ; 0-based index
    LD E, A
    LD D, 0
    ADD HL, DE
    LD A, (RING0_BUF + 24)    ; near_id
    LD (HL), A
    JP act_done

; --- act_craft: tool→compass, weapon→shield ---
act_craft:
    LD A, (IX+14)
    CP 2                       ; tool?
    JR Z, .craft_compass
    CP 3                       ; weapon?
    JR Z, .craft_shield
    JP act_done
.craft_compass:
    LD (IX+14), 7              ; → compass
    JP act_done
.craft_shield:
    LD (IX+14), 6              ; → shield
    JP act_done

; --- act_teach: copy 4-byte genome fragment to adjacent NPC ---
act_teach:
    LD A, (IX+4)
    CP 15
    JP C, act_done
    LD A, (RING0_BUF + 14)
    CP 2
    JP NC, act_done
    LD A, (RING0_BUF + 24)
    OR A
    JP Z, act_done
    CALL find_npc_by_id
    JP Z, act_done
    ; Copy 4 bytes from our genome to theirs at random offset
    CALL lfsr_next
    LD B, A
    LD A, (IX+11)              ; our genome len
    CP 5
    JP C, act_done             ; need len >= 5 to teach (avoid mod-0)
    SUB 4
    LD C, A
.teach_mod:
    LD A, B
    CP C
    JR C, .teach_ok
    SUB C
    LD B, A
    JR .teach_mod
.teach_ok:
    ; Source = our genome + offset
    LD L, (IX+12)
    LD H, (IX+13)
    LD D, 0
    LD E, B
    ADD HL, DE
    ; Dest = their genome + offset
    LD E, (IY+12)
    LD D, (IY+13)
    PUSH HL
    EX DE, HL
    LD D, 0
    LD E, B
    ADD HL, DE
    EX DE, HL                  ; DE = dest
    POP HL                     ; HL = source
    LD BC, 4
    LDIR
    ; Cost energy
    LD A, (IX+4)
    SUB 15
    LD (IX+4), A
    JP act_done

; ============================================================================
; find_npc_by_id: Find NPC with given ID
; A = target ID. Returns IY = NPC entry, Z flag clear if found.
; Z flag set if NOT found.
; ============================================================================
find_npc_by_id:
    LD (find_target), A
    LD IY, NPC_TABLE
    LD B, MAX_NPCS
.find_loop:
    LD A, (IY+0)
    LD C, A
    LD A, (find_target)
    CP C
    JR NZ, .find_next
    ; Found — clear Z flag
    OR A                       ; A is nonzero (it's the ID), so Z=0
    RET
.find_next:
    LD DE, NPC_SIZE
    ADD IY, DE
    DJNZ .find_loop
    ; Not found — set Z
    XOR A                      ; Z=1
    RET

find_target: DB 0

; ============================================================================
; try_eat: Try to eat food at (B,C). Sets carry if eaten.
; IX = NPC table entry
; ============================================================================
try_eat:
    LD A, B
    CP WORLD_SIZE_X
    JR NC, .te_fail
    LD A, C
    CP WORLD_SIZE_Y
    JR NC, .te_fail
    PUSH BC
    CALL tile_addr
    LD A, (HL)
    POP BC
    CP TILE_FOOD
    JR NZ, .te_fail
    ; Clear food tile
    PUSH BC
    CALL tile_addr
    LD (HL), TILE_EMPTY
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
    ; Track action count
    LD HL, (stat_eat)
    INC HL
    LD (stat_eat), HL
    SCF                        ; carry = success
    RET
.te_fail:
    OR A                       ; clear carry
    RET

; ============================================================================
; Tile + Occupancy helpers
; ============================================================================

; tile_addr: (B,C) → HL = address in TILE_GRID
; B = X, C = Y
tile_addr:
    LD H, 0
    LD L, C
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL                 ; HL = Y * 32
    LD D, 0
    LD E, B
    ADD HL, DE                 ; HL = Y*32 + X
    LD DE, TILE_GRID
    ADD HL, DE
    RET

; occ_addr: (B,C) → HL = address in OCC_GRID
occ_addr:
    LD H, 0
    LD L, C
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    LD D, 0
    LD E, B
    ADD HL, DE
    LD DE, OCC_GRID
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

; occ_at: A = occupant at (B,C) from OCC_GRID
occ_at:
    PUSH HL
    PUSH DE
    CALL occ_addr
    LD A, (HL)
    POP DE
    POP HL
    RET

; clear_npc_occ: Clear occupancy at NPC's position
; IX = NPC
clear_npc_occ:
    PUSH BC
    LD B, (IX+1)
    LD C, (IX+2)
    CALL occ_addr
    LD (HL), 0
    POP BC
    RET

; set_npc_occ: Set occupancy at NPC's position to NPC ID
; IX = NPC
set_npc_occ:
    PUSH BC
    LD B, (IX+1)
    LD C, (IX+2)
    CALL occ_addr
    LD A, (IX+0)
    LD (HL), A
    POP BC
    RET

; ============================================================================
; Food respawn + Trade resolution
; ============================================================================
respawn_food:
    ; Try to spawn one food item, biome-weighted
    CALL lfsr_next
    AND WORLD_SIZE_X - 1       ; 0-31
    LD B, A
    CALL lfsr_next
    CALL mod24
    LD C, A
    CALL get_tile
    OR A                       ; empty?
    RET NZ
    CALL occ_at
    OR A
    RET NZ
    ; Check biome food rate: LFSR roll vs threshold
    PUSH BC
    ; Read biome at (B,C)
    LD H, 0
    LD L, C
    ADD HL, HL : ADD HL, HL : ADD HL, HL : ADD HL, HL : ADD HL, HL
    LD D, 0
    LD E, B
    ADD HL, DE
    LD DE, BIOME_GRID
    ADD HL, DE
    LD A, (HL)                 ; biome type
    LD HL, food_rate_table
    LD E, A
    LD D, 0
    ADD HL, DE
    LD B, (HL)                 ; B = threshold (0-255)
    PUSH BC
    CALL lfsr_next             ; A = random 0-255
    POP BC
    CP B                       ; spawn if A < threshold
    POP BC
    RET NC                     ; A >= threshold → no food
    LD A, TILE_FOOD
    JP set_tile

; Food spawn rate by biome (probability out of 255)
food_rate_table:
    DB 153                     ; 0 Clearing: 60%
    DB 77                      ; 1 Forest: 30%
    DB 25                      ; 2 Mountain: 10%
    DB 51                      ; 3 Swamp: 20%
    DB 38                      ; 4 Village: 15%
    DB 128                     ; 5 River: 50%
    DB 51                      ; 6 Bridge: 20%

; resolve_trades: Check bilateral trade matches, swap items
resolve_trades:
    LD B, MAX_NPCS
    LD IX, NPC_TABLE
.rt_loop:
    PUSH BC
    LD A, (IX+0)
    OR A
    JR Z, .rt_next
    ; Check our trade intent
    LD A, (IX+0)
    DEC A
    LD HL, TRADE_TABLE
    LD E, A
    LD D, 0
    ADD HL, DE
    LD A, (HL)
    OR A
    JR Z, .rt_next             ; no intent
    ; A = target ID. Check if target wants to trade with us.
    LD C, A                    ; C = target ID
    PUSH HL
    CALL find_npc_by_id
    POP HL
    JR Z, .rt_next             ; target not found
    ; Check target's trade intent
    LD A, (IY+0)
    DEC A
    LD DE, TRADE_TABLE
    ADD A, E
    LD E, A
    LD A, 0
    ADC A, D
    LD D, A
    EX DE, HL
    LD A, (HL)                 ; target's intent
    CP (IX+0)                  ; does target want us?
    JR NZ, .rt_next
    ; Bilateral match! Swap items.
    LD A, (IX+14)
    LD E, A
    LD A, (IY+14)
    LD (IX+14), A
    LD (IY+14), E
    ; Clear both intents
    EX DE, HL
    LD (HL), 0                 ; clear target intent
    POP BC                     ; recover loop counter temporarily
    PUSH BC
    LD A, (IX+0)
    DEC A
    LD HL, TRADE_TABLE
    LD E, A
    LD D, 0
    ADD HL, DE
    LD (HL), 0                 ; clear our intent
.rt_next:
    POP BC
    LD DE, NPC_SIZE
    ADD IX, DE
    DEC B
    JR NZ, .rt_loop
    RET

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

lfsr_state: DW $ACE1          ; seed

; mod24: A = A mod 24 (unsigned). Destroys no other regs.
mod24:
.m24_loop:
    CP WORLD_SIZE_Y
    RET C                      ; A < 24 → done
    SUB WORLD_SIZE_Y
    JR .m24_loop

; ============================================================================
; Init NPCs
; ============================================================================
init_npcs:
    LD IX, NPC_TABLE
    LD A, 1
    LD (npc_id_ctr), A
    LD HL, GENOME_BANK
    LD B, MAX_NPCS

.in_loop:
    PUSH BC
    PUSH HL

    ; Set NPC fields
    LD A, (npc_id_ctr)
    LD (IX+0), A               ; ID
    CALL lfsr_next
    AND WORLD_SIZE_X - 1
    LD (IX+1), A               ; X
    CALL lfsr_next
    ; Y = A mod 24: use repeated subtraction
    CALL mod24
    LD (IX+2), A               ; Y
    LD (IX+3), 100             ; health
    LD (IX+4), 100             ; energy
    LD (IX+5), 0               ; age lo
    LD (IX+6), 0               ; age hi
    LD (IX+7), 0               ; hunger
    LD (IX+8), 0               ; food eaten
    LD (IX+9), 0               ; fitness lo
    LD (IX+10), 0              ; fitness hi
    LD (IX+14), 0              ; item
    LD (IX+15), 0              ; flags

    ; Genome: WFC-generated seed
    POP HL
    LD (IX+12), L              ; genome ptr lo
    LD (IX+13), H              ; genome ptr hi
    PUSH HL                    ; save genome ptr for copy

    ; Generate WFC genome → GA_SCRATCH, length in ga_child_len
    PUSH IX
    CALL wfc_gen_genome
    POP IX

    ; Copy from GA_SCRATCH to genome slot
    LD A, (ga_child_len)
    LD (IX+11), A              ; genome length
    POP HL                     ; HL = genome ptr (dest)
    EX DE, HL                  ; DE = dest
    LD HL, GA_SCRATCH          ; HL = source
    LD C, A
    LD B, 0
    OR A
    JR Z, .in_skip_copy
    LDIR
.in_skip_copy:

    ; Compute next genome slot (128 bytes per NPC)
    LD L, (IX+12)
    LD H, (IX+13)
    LD DE, GENOME_MAX          ; 128
    ADD HL, DE                 ; HL = next genome slot

    ; Place NPC on occupancy grid
    CALL set_npc_occ

    ; Advance NPC table
    LD DE, NPC_SIZE
    ADD IX, DE
    LD A, (npc_id_ctr)
    INC A
    LD (npc_id_ctr), A
    POP BC                     ; loop counter (pushed at top of .in_loop)
    DEC B
    JP NZ, .in_loop
    RET

npc_id_ctr: DB 0

; ============================================================================
; Seed food
; ============================================================================
seed_food:
    LD B, FOOD_INIT
.sf_loop:
    PUSH BC
    CALL lfsr_next
    AND WORLD_SIZE_X - 1
    LD B, A
    CALL lfsr_next
    CALL mod24
    LD C, A
    CALL get_tile
    OR A
    JR NZ, .sf_skip
    CALL occ_at
    OR A
    JR NZ, .sf_skip
    LD A, TILE_FOOD
    CALL set_tile
.sf_skip:
    POP BC
    DJNZ .sf_loop
    RET

; ============================================================================
; GA: Tournament-3, crossover, 3 mutation types
; ============================================================================
    INCLUDE "ga.asm"

; ============================================================================
; WFC Biome Generation
; ============================================================================
    INCLUDE "wfc_biome.asm"

; ============================================================================
; WFC Genome Generation
; ============================================================================
    INCLUDE "wfc_genome.asm"

; ============================================================================
; Print stats
; ============================================================================
print_stats:
    ; "T=nnn A=nnn F=nnn E=nnn K=nnn H=nnn\n"
    LD A, 'T'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD DE, (tick_count)
    CALL pr_s16
    LD A, ' '
    OUT ($23), A

    ; Count alive + find best fitness
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
    LD C, 0                    ; alive count
    LD HL, 0                   ; best fitness
.ps_cnt:
    LD A, (IX+0)
    OR A
    JR Z, .ps_sk
    INC C
    ; Check fitness
    LD E, (IX+9)
    LD D, (IX+10)
    PUSH HL
    OR A
    SBC HL, DE
    POP HL
    JR NC, .ps_sk              ; HL >= DE, not better
    LD L, (IX+9)
    LD H, (IX+10)              ; new best
.ps_sk:
    LD DE, NPC_SIZE
    ADD IX, DE
    DJNZ .ps_cnt

    ; Print alive
    LD A, 'A'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    PUSH HL
    LD E, C
    LD D, 0
    CALL pr_s16
    LD A, ' '
    OUT ($23), A
    POP HL

    ; Print best fitness
    LD A, 'F'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    EX DE, HL
    CALL pr_s16
    LD A, ' '
    OUT ($23), A

    ; Print eat count
    LD A, 'E'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD DE, (stat_eat)
    CALL pr_s16
    LD A, ' '
    OUT ($23), A

    ; Print attack count
    LD A, 'K'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD DE, (stat_attack)
    CALL pr_s16
    LD A, ' '
    OUT ($23), A

    ; Print harvest count
    LD A, 'H'
    OUT ($23), A
    LD A, '='
    OUT ($23), A
    LD DE, (stat_harvest)
    CALL pr_s16

    LD A, 10
    OUT ($23), A

    ; Reset stats
    LD HL, 0
    LD (stat_eat), HL
    LD (stat_attack), HL
    LD (stat_harvest), HL
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
tick_count:   DW 0
stat_eat:     DW 0
stat_attack:  DW 0
stat_harvest: DW 0

str_start: DB "NPC Sandbox Z80 v3", 10, 0
str_done:  DB "Done", 10, 0

sandbox_end:
