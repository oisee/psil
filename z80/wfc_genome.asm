; ============================================================================
; WFC Genome Generation for Z80
; 1D Wave Function Collapse with 8 token types (byte bitmask)
; Reuses popcount LUT ($7000) and collapse patterns from wfc_biome.asm
; ============================================================================
;
; 8 Token Types (bits 0-7):
;   0 = Sense   (r0@ N)           2 bytes
;   1 = Push    (small num)       1 byte
;   2 = Cmp     (=,<,>,not)       1 byte
;   3 = Branch  (jnz,jz)          2 bytes
;   4 = Move    (act.move,r1!0)   2 bytes
;   5 = Action  (act.eat...,r1!1) 2 bytes
;   6 = Ops     (dup,+,-,*,etc)   1 byte
;   7 = Yield   (yield)           1 byte
;
; Work area: reuses WFC_WORK ($6600), up to 32 bytes (max genome tokens)

WFC_GEN_TOKENS EQU 10         ; number of token cells to generate

; ============================================================================
; wfc_gen_genome: Generate a genome via 1D WFC
;   Output: genome written to GA_SCRATCH, length in ga_child_len
;   Clobbers: AF, BC, DE, HL, IX
;   Uses wg_cells (10 bytes) as work area, separate from GA_SCRATCH
; ============================================================================
wfc_gen_genome:
    ; 1. Init cells: wg_cells[0..N-1] = $FF (all 8 types possible)
    LD HL, wg_cells
    LD A, $FF
    LD B, WFC_GEN_TOKENS
.wg_init:
    LD (HL), A
    INC HL
    DJNZ .wg_init

    ; 2. Anchor: cell[0] = bit0 (Tok8Sense)
    LD A, %00000001
    LD (wg_cells), A

    ; 3. Anchor: cell[N-1] = bit7 (Tok8Yield)
    LD HL, wg_cells + WFC_GEN_TOKENS - 1
    LD A, %10000000
    LD (HL), A

    ; 4. Backward propagation: constrain cells so Yield is reachable
    ;    For each cell i from N-2 down to 0: cell[i] &= types_that_can_precede(cell[i+1])
    CALL wg_propagate_back
    JR C, .wg_fallback          ; contradiction → fallback

    ; 5. Forward propagation pass
    CALL wg_propagate
    JR C, .wg_fallback          ; contradiction → fallback

    ; 6. Collapse left-to-right
    LD B, WFC_GEN_TOKENS
    LD HL, wg_cells
.wg_collapse_loop:
    PUSH BC
    PUSH HL

    ; Check if already collapsed (popcount = 1)
    LD A, (HL)
    CALL popcount
    CP 1
    JR Z, .wg_already_collapsed

    ; Collapse this cell to random valid token
    POP HL
    PUSH HL
    LD A, (HL)                  ; A = possibilities bitmask
    CALL wg_collapse_random     ; A = single-bit result
    POP HL
    PUSH HL
    LD (HL), A                  ; write back

    ; Propagate forward from this cell
    CALL wg_propagate
    JR C, .wg_collapse_fallback ; contradiction

.wg_already_collapsed:
    POP HL
    INC HL
    POP BC
    DJNZ .wg_collapse_loop

    ; 7. Render tokens to bytecode in GA_SCRATCH
    CALL wg_render
    RET

.wg_collapse_fallback:
    POP HL                      ; discard saved HL
    POP BC                      ; discard saved BC
.wg_fallback:
    ; Fallback: write hardcoded eat-loop (same as old init_npcs)
    LD HL, GA_SCRATCH
    LD (HL), $8A                ; OpRing0R
    INC HL
    LD (HL), $0D                ; sensor: food dir
    INC HL
    LD (HL), $8C                ; OpRing1W
    INC HL
    LD (HL), $00                ; slot 0 (move)
    INC HL
    LD (HL), $21                ; push 1
    INC HL
    LD (HL), $8C                ; OpRing1W
    INC HL
    LD (HL), $01                ; slot 1 (eat)
    INC HL
    LD (HL), $F1                ; yield
    ; Pad to 16 bytes
    LD B, 8
    LD A, $F0                   ; halt
.wg_fb_pad:
    INC HL
    LD (HL), A
    DJNZ .wg_fb_pad
    LD A, 16
    LD (ga_child_len), A
    RET

; ============================================================================
; wg_propagate: Forward-pass constraint propagation
;   For each collapsed cell i, cell[i+1] &= get_allowed_genome(cell[i])
;   Returns: carry set on contradiction (cell becomes 0)
; ============================================================================
wg_propagate:
    LD HL, wg_cells
    LD B, WFC_GEN_TOKENS - 1   ; N-1 propagation steps
.wgp_loop:
    LD A, (HL)                  ; cell[i] possibilities
    PUSH HL
    PUSH BC
    CALL get_allowed_genome     ; A = union of allowed followers
    POP BC
    POP HL
    INC HL
    AND (HL)                    ; cell[i+1] &= allowed
    LD (HL), A
    JR Z, .wgp_contradiction   ; if 0, contradiction
    DJNZ .wgp_loop
    OR A                        ; clear carry
    RET
.wgp_contradiction:
    SCF                         ; set carry = error
    RET

; ============================================================================
; wg_propagate_back: Backward constraint propagation
;   For each cell i from N-2 down to 0:
;     cell[i] &= get_predecessors(cell[i+1])
;   Returns: carry set on contradiction
; ============================================================================
wg_propagate_back:
    LD HL, wg_cells + WFC_GEN_TOKENS - 1   ; start at last cell
    LD B, WFC_GEN_TOKENS - 1               ; N-1 steps
.wgpb_loop:
    LD A, (HL)                  ; cell[i+1] possibilities (target)
    PUSH HL
    PUSH BC
    CALL get_predecessors       ; A = types that can precede target
    POP BC
    POP HL
    DEC HL                      ; move to cell[i]
    AND (HL)                    ; cell[i] &= predecessors
    LD (HL), A
    JR Z, .wgpb_contradiction
    DJNZ .wgpb_loop
    OR A                        ; clear carry
    RET
.wgpb_contradiction:
    SCF
    RET

; ============================================================================
; get_predecessors: Given target cell possibilities in A,
;   return types that can reach any of those possibilities.
;   For each type t (0-7), if constraints[t] AND target != 0, set bit t.
; Clobbers: HL, DE, B, C
; ============================================================================
get_predecessors:
    LD C, A                     ; C = target possibilities
    LD HL, wfc_genome_constraints
    XOR A                       ; result = 0
    LD B, 8                     ; 8 types
    LD E, 1                     ; E = current type bit mask
.gp_loop:
    LD D, A                     ; save result
    LD A, (HL)                  ; constraints[type]
    AND C                       ; can this type reach any target possibility?
    LD A, D                     ; restore result
    JR Z, .gp_skip
    OR E                        ; set bit for this type
.gp_skip:
    SLA E                       ; next type bit
    INC HL                      ; next constraint entry
    DJNZ .gp_loop
    RET

; ============================================================================
; get_allowed_genome: Given source cell possibilities in A,
;   return union of allowed next token types in A
; Clobbers: HL, E, B
; ============================================================================
get_allowed_genome:
    LD HL, wfc_genome_constraints
    LD E, A                     ; E = source possibilities
    XOR A                       ; accumulator = 0
    LD B, 8                     ; 8 token type bits
.gag_loop:
    RRC E                       ; bit into carry
    JR NC, .gag_skip
    OR (HL)                     ; OR in constraint for this token
.gag_skip:
    INC HL
    DJNZ .gag_loop
    RET

; ============================================================================
; wg_collapse_random: Collapse bitmask A to a single random set bit
;   Input: A = possibilities bitmask (guaranteed nonzero)
;   Output: A = single-bit mask for chosen token
;   Clobbers: BC, DE
; ============================================================================
wg_collapse_random:
    PUSH AF                     ; save possibilities
    CALL popcount               ; A = count of set bits
    LD B, A                     ; B = count
    CALL lfsr_next              ; A = random byte
    ; A mod B (B guaranteed > 0)
.wcr_mod:
    CP B
    JR C, .wcr_mod_done
    SUB B
    JR .wcr_mod
.wcr_mod_done:
    LD B, A                     ; B = target index (0-indexed)
    POP AF                      ; A = possibilities bitmask
    LD E, 1                     ; E = current single-bit mask
.wcr_find:
    SRL A                       ; shift bit 0 into carry
    JR NC, .wcr_skip            ; bit not set, advance mask
    LD D, A                     ; save remaining shifted possibilities
    LD A, B
    OR A
    JR Z, .wcr_found            ; target index == 0 → found!
    DEC B
    LD A, D                     ; restore shifted possibilities
.wcr_skip:
    SLA E                       ; advance bit mask
    JR .wcr_find                ; guaranteed to find
.wcr_found:
    LD A, E                     ; A = single-bit mask
    RET

; ============================================================================
; wg_render: Render collapsed token cells to bytecode in GA_SCRATCH
;   Handlers must preserve IX (cell iterator) and leave HL past written bytes
;   Note: lfsr_next clobbers HL, so handlers save/restore HL around it
; ============================================================================
wg_render:
    LD IX, wg_cells             ; source: collapsed token cells
    LD HL, GA_SCRATCH           ; dest: bytecode output
    LD B, WFC_GEN_TOKENS        ; token count

.wr_loop:
    PUSH BC
    PUSH HL                     ; save output pointer

    LD A, (IX+0)                ; collapsed cell bitmask (single bit)
    CALL bitmask_to_index       ; A = token type index 0-7 (clobbers B only)

    ; Look up handler address from jump table
    ADD A, A                    ; A = type * 2
    LD E, A
    LD D, 0
    LD HL, .wr_table
    ADD HL, DE
    LD E, (HL)
    INC HL
    LD D, (HL)                  ; DE = handler address

    POP HL                      ; HL = output pointer
    PUSH HL                     ; re-push for .wr_next to pop

    PUSH DE                     ; push handler address
    RET                         ; dispatch: jump to handler, HL = output ptr

.wr_table:
    DW .wr_sense                ; 0 = Sense
    DW .wr_push                 ; 1 = Push
    DW .wr_cmp                  ; 2 = Cmp
    DW .wr_branch               ; 3 = Branch
    DW .wr_move                 ; 4 = Move
    DW .wr_action               ; 5 = Action
    DW .wr_ops                  ; 6 = Ops
    DW .wr_yield                ; 7 = Yield

.wr_sense:
    LD (HL), $8A                ; OpRing0R
    INC HL
    PUSH HL                     ; save output pointer (lfsr_next clobbers HL)
    CALL lfsr_next
    AND $07
    LD E, A
    LD D, 0
    LD HL, wg_sensors
    ADD HL, DE
    LD A, (HL)                  ; A = sensor slot
    POP HL                      ; restore output pointer
    LD (HL), A
    INC HL
    JP .wr_next

.wr_push:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    POP HL                      ; restore output pointer
    AND $07
    ADD A, $20
    LD (HL), A
    INC HL
    JP .wr_next

.wr_cmp:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    AND $03
    LD E, A
    LD D, 0
    LD HL, wg_cmp_ops
    ADD HL, DE
    LD A, (HL)                  ; A = cmp opcode
    POP HL                      ; restore output pointer
    LD (HL), A
    INC HL
    JP .wr_next

.wr_branch:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    POP HL                      ; restore output pointer
    AND $03
    JR Z, .wr_br_jz
    LD (HL), $88                ; OpJumpNZ
    JR .wr_br_offset
.wr_br_jz:
    LD (HL), $87                ; OpJumpZ
.wr_br_offset:
    INC HL
    LD (HL), $04                ; default offset = 4
    INC HL
    JP .wr_next

.wr_move:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    POP HL                      ; restore output pointer
    BIT 0, A
    JR Z, .wr_move_r1w
    LD (HL), $93                ; OpActMove
    INC HL
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    POP HL                      ; restore output pointer
    AND $07
    JR NZ, .wr_move_dir_ok
    INC A                       ; ensure 1-7
.wr_move_dir_ok:
    LD (HL), A
    INC HL
    JP .wr_next
.wr_move_r1w:
    LD (HL), $8C                ; OpRing1W
    INC HL
    LD (HL), $00                ; slot 0 = move
    INC HL
    JP .wr_next

.wr_action:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    POP HL                      ; restore output pointer
    AND $07
    CP 6                        ; 6,7 → r1! (2/8 ≈ 25%)
    JR NC, .wr_act_r1w
    PUSH HL                     ; save output pointer
    LD E, A
    LD D, 0
    LD HL, wg_action_ops
    ADD HL, DE
    LD A, (HL)                  ; A = action opcode
    POP HL                      ; restore output pointer
    LD (HL), A
    INC HL
    LD (HL), $00                ; arg = 0
    INC HL
    JP .wr_next
.wr_act_r1w:
    LD (HL), $8C                ; OpRing1W
    INC HL
    LD (HL), $01                ; slot 1 = action
    INC HL
    JP .wr_next

.wr_ops:
    PUSH HL                     ; save output pointer
    CALL lfsr_next
    AND $07
    LD E, A
    LD D, 0
    LD HL, wg_ops_table
    ADD HL, DE
    LD A, (HL)                  ; A = ops opcode
    POP HL                      ; restore output pointer
    LD (HL), A
    INC HL
    JP .wr_next

.wr_yield:
    LD (HL), $F1                ; OpYield
    INC HL

.wr_next:
    POP DE                      ; discard saved output pointer
    INC IX                      ; next token cell
    POP BC
    DEC B
    JP NZ, .wr_loop

    ; Compute genome length = HL - GA_SCRATCH
    LD DE, GA_SCRATCH
    OR A
    SBC HL, DE
    LD A, L
    LD (ga_child_len), A
    RET

; ============================================================================
; Render tables
; ============================================================================

wg_sensors:
    DB 1, 2, 3, 5, 7, 13, 18, 19

wg_cmp_ops:
    DB $0B, $0C, $0D, $10       ; eq, lt, gt, not

wg_action_ops:
    DB $96, $97, $94, $95, $99, $9B  ; eat, harvest, attack, heal, share, craft

wg_ops_table:
    DB $01, $02, $03, $06, $07, $08, $0E, $0F  ; dup,drop,swap,add,sub,mul,and,or

; Mined from archetype genomes merged with base patterns
; Bit: 76543210 = Yield,Ops,Action,Move,Branch,Cmp,Push,Sense
wfc_genome_constraints:
    DB %01110110                ; Sense(0):  Push, Cmp, Move, Action, Ops
    DB %01100101                ; Push(1):   Sense, Cmp, Action, Ops
    DB %01001000                ; Cmp(2):    Branch, Ops
    DB %00110001                ; Branch(3): Sense, Move, Action
    DB %10100011                ; Move(4):   Sense, Push, Action, Yield
    DB %10010001                ; Action(5): Sense, Move, Yield
    DB %01000110                ; Ops(6):    Push, Cmp, Ops
    DB %00000011                ; Yield(7):  Sense, Push

; 10-byte work area for WFC genome cells (separate from GA_SCRATCH)
wg_cells:
    DS WFC_GEN_TOKENS, 0
