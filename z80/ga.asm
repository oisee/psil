; ============================================================================
; GA: Tournament-3 Selection, Single-point Crossover, 3 Mutation Types
; ============================================================================

; evolve_step: Create offspring from two tournament-selected parents
; Replace lowest-fitness living NPC with child
evolve_step:
    ; Count living NPCs — need at least 3 for tournament
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
    LD C, 0
.ev_cnt:
    LD A, (IX+0)
    OR A
    JR Z, .ev_cnt_sk
    INC C
.ev_cnt_sk:
    LD DE, NPC_SIZE
    ADD IX, DE
    DJNZ .ev_cnt

    LD A, C
    OR A
    RET Z                      ; need at least 1 living

    ; Find a dead NPC first (preferred); fall back to worst living
    CALL find_dead_or_worst
    LD (ga_worst_ix), HL

    ; Tournament selection for parent A
    CALL tournament3
    LD (ga_parent_a), HL

    ; If only 1 alive, clone parent A as parent B
    LD A, C
    CP 2
    JR NC, .ev_two_parents
    LD HL, (ga_parent_a)
    LD (ga_parent_b), HL
    JR .ev_do_crossover
.ev_two_parents:
    ; Tournament selection for parent B
    CALL tournament3
    LD (ga_parent_b), HL
.ev_do_crossover:

    ; Single-point crossover → child genome in GA_SCRATCH
    CALL crossover

    ; Apply mutation to child in GA_SCRATCH
    CALL mutate

    ; Copy child genome from scratch to worst NPC's genome slot
    LD HL, (ga_worst_ix)
    PUSH HL
    POP IX                     ; IX = worst NPC

    ; Copy child genome
    LD L, (IX+12)
    LD H, (IX+13)              ; HL = dest genome ptr
    EX DE, HL                  ; DE = dest
    LD HL, GA_SCRATCH          ; HL = source
    LD A, (ga_child_len)
    LD C, A
    LD B, 0
    LDIR

    ; If NPC was dead, assign new ID
    LD A, (IX+0)
    OR A
    JR NZ, .ev_alive
    ; Assign new ID
    LD A, (npc_id_ctr)
    INC A
    LD (npc_id_ctr), A
    LD (IX+0), A
    ; Random position
    CALL lfsr_next
    AND WORLD_SIZE_X - 1
    LD (IX+1), A
    CALL lfsr_next
    CALL mod24
    LD (IX+2), A
.ev_alive:

    ; Reset stats
    LD (IX+3), 100             ; health
    LD (IX+4), 100             ; energy
    LD (IX+5), 0               ; age lo
    LD (IX+6), 0               ; age hi
    LD (IX+7), 0               ; hunger
    LD (IX+8), 0               ; food eaten
    LD (IX+9), 0               ; fitness lo
    LD (IX+10), 0              ; fitness hi
    LD A, (ga_child_len)
    LD (IX+11), A              ; genome length
    LD (IX+14), 0              ; item
    LD (IX+15), 0              ; flags

    ; Place on occupancy grid
    CALL set_npc_occ

    ; Resolve any pending trades
    CALL resolve_trades

    ; Clear trade table
    LD HL, TRADE_TABLE
    LD DE, TRADE_TABLE + 1
    LD BC, MAX_NPCS - 1
    LD (HL), 0
    LDIR

    RET

; ============================================================================
; find_dead_or_worst: Find dead NPC first; if none dead, find worst living
; Returns HL = pointer to NPC table entry
; ============================================================================
find_dead_or_worst:
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
.fdw_loop:
    LD A, (IX+0)
    OR A
    JR Z, .fdw_found_dead
    LD DE, NPC_SIZE
    ADD IX, DE
    DJNZ .fdw_loop
    ; No dead NPCs — find worst living
    JR find_worst
.fdw_found_dead:
    PUSH IX
    POP HL
    RET

; ============================================================================
; find_worst: Find living NPC with lowest fitness
; Returns HL = pointer to NPC table entry
; ============================================================================
find_worst:
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
    LD HL, 0                   ; best candidate ptr
    LD DE, $7FFF               ; worst fitness (init high)

.fw_loop:
    LD A, (IX+0)
    OR A
    JR Z, .fw_skip

    ; Compare fitness
    LD C, (IX+9)
    LD A, (IX+10)
    ; CA = fitness (A=hi, C=lo)
    CP D
    JR C, .fw_better           ; hi < D → definitely lower
    JR NZ, .fw_skip            ; hi > D → definitely higher
    LD A, C
    CP E
    JR NC, .fw_skip            ; lo >= E → not lower
.fw_better:
    LD E, (IX+9)
    LD D, (IX+10)              ; DE = new worst fitness
    PUSH IX
    POP HL                     ; HL = this NPC

.fw_skip:
    PUSH DE
    LD DE, NPC_SIZE
    ADD IX, DE
    POP DE
    DJNZ .fw_loop
    RET

; ============================================================================
; tournament3: Select best of 3 random living NPCs
; Returns HL = pointer to best NPC table entry
; ============================================================================
tournament3:
    ; Pick 3 random living NPCs, return fittest
    LD HL, 0
    LD (t3_best_fit), HL
    LD (t3_best_ptr), HL

    LD C, 3                    ; 3 candidates
.t3_pick:
    PUSH BC
    ; Pick random living NPC
    CALL pick_random_living     ; IX = random living NPC
    ; Compare with current best
    LD L, (IX+9)
    LD H, (IX+10)              ; HL = candidate fitness
    LD DE, (t3_best_fit)
    OR A
    SBC HL, DE
    ADD HL, DE                 ; restore HL
    JR C, .t3_not_better       ; HL < DE → not better
    LD (t3_best_fit), HL
    PUSH IX
    POP HL
    LD (t3_best_ptr), HL
.t3_not_better:
    POP BC
    DEC C
    JR NZ, .t3_pick

    LD HL, (t3_best_ptr)
    ; If no valid pick (shouldn't happen), return first NPC
    LD A, H
    OR L
    RET NZ
    LD HL, NPC_TABLE
    RET

t3_best_fit: DW 0
t3_best_ptr: DW 0

; ============================================================================
; pick_random_living: Return IX pointing to a random living NPC
; Tries up to 32 times, falls back to first living.
; ============================================================================
pick_random_living:
    LD B, 32                   ; max attempts
.prl_try:
    CALL lfsr_next
    AND MAX_NPCS - 1           ; 0..15 (works since MAX_NPCS is power of 2)
    ; Compute NPC_TABLE + A * 16
    RLCA
    RLCA
    RLCA
    RLCA                       ; A * 16 (shift left 4)
    LD E, A
    LD D, 0
    LD IX, NPC_TABLE
    ADD IX, DE
    LD A, (IX+0)
    OR A
    RET NZ                     ; found living NPC
    DJNZ .prl_try
    ; Fallback: scan for first living
    LD IX, NPC_TABLE
    LD B, MAX_NPCS
.prl_scan:
    LD A, (IX+0)
    OR A
    RET NZ
    LD DE, NPC_SIZE
    ADD IX, DE
    DJNZ .prl_scan
    ; All dead — return first entry anyway
    LD IX, NPC_TABLE
    RET

; ============================================================================
; crossover: Single-point crossover of parent A and B
; Result in GA_SCRATCH, length in ga_child_len
; ============================================================================
crossover:
    ; Load parent A
    LD HL, (ga_parent_a)
    PUSH HL
    POP IX
    LD A, (IX+11)              ; parent A genome length
    LD (ga_len_a), A
    LD L, (IX+12)
    LD H, (IX+13)
    LD (ga_ptr_a), HL

    ; Load parent B
    LD HL, (ga_parent_b)
    PUSH HL
    POP IX
    LD A, (IX+11)
    LD (ga_len_b), A
    LD L, (IX+12)
    LD H, (IX+13)
    LD (ga_ptr_b), HL

    ; Pick split point = random mod min(lenA, lenB)
    LD A, (ga_len_a)
    LD B, A
    LD A, (ga_len_b)
    CP B
    JR C, .co_min_ok           ; A < B → A is min
    LD A, B                    ; B is min
.co_min_ok:
    LD (ga_min_len), A
    ; Random split
    CALL lfsr_next
.co_mod:
    CP B
    JR C, .co_mod_done
    SUB B
    JR .co_mod
.co_mod_done:
    LD (ga_split), A           ; split point

    ; Copy parent_A[0..split] to scratch
    LD HL, (ga_ptr_a)
    LD DE, GA_SCRATCH
    LD A, (ga_split)
    OR A
    JR Z, .co_no_a             ; split=0 → skip A part
    LD C, A
    LD B, 0
    LDIR
.co_no_a:

    ; Copy parent_B[split..lenB] to scratch + split
    LD HL, (ga_ptr_b)
    LD A, (ga_split)
    LD C, A
    LD B, 0
    ADD HL, BC                 ; skip past split point in B
    ; Remaining bytes = lenB - split
    LD A, (ga_len_b)
    SUB C
    JR Z, .co_no_b             ; nothing to copy
    JR C, .co_no_b
    LD C, A
    LD B, 0
    LDIR
.co_no_b:

    ; Child length = split + (lenB - split) = lenB
    ; Actually, use lenB as child length (since we take tail from B)
    LD A, (ga_len_b)
    ; Cap at GENOME_MAX
    CP GENOME_MAX
    JR C, .co_len_ok
    LD A, GENOME_MAX
.co_len_ok:
    LD (ga_child_len), A

    RET

; ============================================================================
; mutate: Apply one of 3 mutation types to child in GA_SCRATCH
; ============================================================================
mutate:
    CALL lfsr_next
    AND $03                    ; 0-3
    CP 2
    JR C, .mut_point           ; 0,1 → point mutation (50%)
    CP 3
    JR Z, .mut_delete          ; 3 → delete (25%)
    ; 2 → insert (25%)
    JR .mut_insert

; --- Point mutation: replace random byte ---
.mut_point:
    LD A, (ga_child_len)
    OR A
    RET Z
    LD B, A
    CALL lfsr_next
.mp_mod:
    CP B
    JR C, .mp_ok
    SUB B
    JR .mp_mod
.mp_ok:
    ; A = position
    LD HL, GA_SCRATCH
    LD E, A
    LD D, 0
    ADD HL, DE
    ; Random opcode, biased toward 0x00-0x7F
    CALL lfsr_next
    AND $7F
    LD (HL), A
    RET

; --- Insert mutation: add 1 byte at random position ---
.mut_insert:
    LD A, (ga_child_len)
    CP GENOME_MAX
    RET NC                     ; already at max
    ; Pick position
    LD B, A
    CALL lfsr_next
.mi_mod:
    CP B
    JR C, .mi_ok
    SUB B
    JR .mi_mod
.mi_ok:
    LD C, A                    ; C = insert position
    ; Shift bytes right from position to end
    LD A, (ga_child_len)
    LD B, A
    SUB C                      ; bytes to shift = len - pos
    JR Z, .mi_no_shift
    ; Source = scratch + len - 1, dest = scratch + len
    LD HL, GA_SCRATCH
    LD E, B
    LD D, 0
    ADD HL, DE                 ; HL = scratch + len
    LD D, H
    LD E, L                    ; DE = scratch + len (dest)
    DEC HL                     ; HL = scratch + len - 1 (source)
    LD A, (ga_child_len)
    SUB C
    LD B, 0
    PUSH AF
    POP BC                     ; BC = count... actually let's be more careful
    LD A, (ga_child_len)
    SUB C
    LD B, A                    ; B = bytes to shift
.mi_shift:
    LD A, (HL)
    LD (DE), A
    DEC HL
    DEC DE
    DJNZ .mi_shift
.mi_no_shift:
    ; Write random byte at position
    LD HL, GA_SCRATCH
    LD E, C
    LD D, 0
    ADD HL, DE
    CALL lfsr_next
    AND $7F
    LD (HL), A
    ; Increment length
    LD A, (ga_child_len)
    INC A
    LD (ga_child_len), A
    RET

; --- Delete mutation: remove 1 byte at random position ---
.mut_delete:
    LD A, (ga_child_len)
    CP 4                       ; minimum genome size
    RET C
    ; Pick position
    LD B, A
    CALL lfsr_next
.md_mod:
    CP B
    JR C, .md_ok
    SUB B
    JR .md_mod
.md_ok:
    LD C, A                    ; C = delete position
    ; Shift bytes left from position+1 to end
    LD HL, GA_SCRATCH
    LD E, C
    LD D, 0
    ADD HL, DE                 ; HL = position to delete
    LD D, H
    LD E, L                    ; DE = dest
    INC HL                     ; HL = source (position + 1)
    LD A, (ga_child_len)
    DEC A
    SUB C                      ; bytes to shift = (len-1) - pos
    JR Z, .md_no_shift
    LD B, A
.md_shift:
    LD A, (HL)
    LD (DE), A
    INC HL
    INC DE
    DJNZ .md_shift
.md_no_shift:
    ; Decrement length
    LD A, (ga_child_len)
    DEC A
    LD (ga_child_len), A
    RET

; ============================================================================
; GA state variables
; ============================================================================
ga_worst_ix:  DW 0
ga_parent_a:  DW 0
ga_parent_b:  DW 0
ga_ptr_a:     DW 0
ga_ptr_b:     DW 0
ga_len_a:     DB 0
ga_len_b:     DB 0
ga_min_len:   DB 0
ga_split:     DB 0
ga_child_len: DB 0
