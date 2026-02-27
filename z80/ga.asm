; ============================================================================
; GA: Simplified Genetic Algorithm for Z80
; Tournament-2 selection, point mutation only
; ============================================================================

; evolve_step: Replace worst NPC with mutated offspring of best
evolve_step:
    ; Find best and worst living NPCs
    LD IX, NPC_TABLE
    LD B, MAX_NPCS

    ; Init best/worst
    LD HL, 0
    LD (ga_best_fit), HL
    LD (ga_best_ix), HL
    LD HL, $7FFF
    LD (ga_worst_fit), HL
    LD (ga_worst_ix), HL

    LD C, 0                ; found count

.ev_scan:
    PUSH BC
    LD A, (IX+0)
    OR A
    JR Z, .ev_sk

    INC C

    ; Fitness at IX+9,+10
    LD L, (IX+9)
    LD H, (IX+10)

    ; Compare with best
    LD DE, (ga_best_fit)
    OR A
    SBC HL, DE
    ADD HL, DE              ; restore HL
    JR C, .ev_not_best
    LD (ga_best_fit), HL
    PUSH IX
    POP HL
    LD (ga_best_ix), HL
    PUSH IX
    POP HL
    LD L, (IX+9)
    LD H, (IX+10)
.ev_not_best:

    ; Compare with worst
    LD DE, (ga_worst_fit)
    OR A
    SBC HL, DE
    ADD HL, DE
    JR NC, .ev_not_worst
    LD (ga_worst_fit), HL
    PUSH IX
    POP HL
    LD (ga_worst_ix), HL
.ev_not_worst:

.ev_sk:
    POP BC
    LD DE, NPC_SIZE
    ADD IX, DE
    DEC B
    JR NZ, .ev_scan

    ; Need at least 2 living NPCs
    LD A, C
    CP 2
    RET C

    ; Copy best's genome to worst's genome with mutation
    LD HL, (ga_best_ix)
    PUSH HL
    POP IX                 ; IX = best NPC

    LD L, (IX+12)
    LD H, (IX+13)          ; HL = best genome ptr
    LD A, (IX+11)          ; A = genome length
    LD (ga_glen), A

    LD DE, (ga_worst_ix)
    PUSH DE
    POP IY                 ; IY = worst NPC

    LD E, (IY+12)
    LD D, (IY+13)          ; DE = worst genome ptr

    ; Copy genome
    LD C, A
    LD B, 0
    PUSH DE
    LDIR
    POP DE

    ; Mutate: pick random position, replace with random opcode
    CALL lfsr_next
    LD C, A
    LD A, (ga_glen)
    LD B, A
    LD A, C
    ; A mod genome_length
.ev_mod:
    CP B
    JR C, .ev_mod_done
    SUB B
    JR .ev_mod
.ev_mod_done:
    ; A = position in genome
    LD C, A
    LD B, 0
    EX DE, HL              ; HL = genome ptr
    ADD HL, BC              ; HL = byte to mutate

    ; Random opcode (biased toward useful 1-byte ops)
    CALL lfsr_next
    AND $7F                ; 0x00-0x7F range
    LD (HL), A

    ; Reset worst NPC stats
    PUSH IY
    POP IX
    LD (IX+3), 100         ; health
    LD (IX+4), 100         ; energy
    LD (IX+5), 0           ; age lo
    LD (IX+6), 0           ; age hi
    LD (IX+7), 0           ; hunger
    LD (IX+8), 0           ; food eaten
    LD (IX+9), 0           ; fitness lo
    LD (IX+10), 0          ; fitness hi

    ; Copy genome length from best
    LD A, (ga_glen)
    LD (IX+11), A

    RET

; GA state
ga_best_fit:  DW 0
ga_best_ix:   DW 0
ga_worst_fit: DW 0
ga_worst_ix:  DW 0
ga_glen:      DB 0
