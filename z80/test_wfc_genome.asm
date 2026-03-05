; ============================================================================
; Test harness for WFC genome generation
; Generates genomes via wfc_gen_genome, prints cell states + hex output
; ============================================================================

    ORG $8000
    JP test_entry

    DEFINE VM_LIB_MODE
    INCLUDE "micro_psil_vm.asm"

; --- Constants ---
WORLD_SIZE_X EQU 32
WORLD_SIZE_Y EQU 24
MAX_NPCS    EQU 16
NPC_SIZE    EQU 16
GENOME_MAX  EQU 128
TILE_GRID   EQU $5B00
OCC_GRID    EQU $5E00
BIOME_GRID  EQU $6100
RING0_BUF   EQU $6400
RING1_BUF   EQU $6440
NPC_TABLE   EQU $6500
GA_SCRATCH  EQU $6600
; WFC_WORK etc defined in wfc_biome.asm

test_entry:
    LD SP, $BFFE
    DI

    CALL init_popcount

    ; Print header
    LD HL, str_header
    CALL print_str

    ; Generate and print 5 genomes
    LD C, 5
.test_loop:
    PUSH BC

    ; Print genome number
    LD A, 'G'
    OUT ($23), A
    LD A, 5
    SUB C
    ADD A, '1'
    OUT ($23), A
    LD A, ':'
    OUT ($23), A
    LD A, 10
    OUT ($23), A

    ; === Step 1: Call wfc_gen_genome ===
    CALL wfc_gen_genome

    ; === Step 2: Print collapsed cells ===
    LD A, ' '
    OUT ($23), A
    LD HL, str_cells
    CALL print_str
    LD HL, wg_cells
    LD B, WFC_GEN_TOKENS
.print_cells:
    LD A, (HL)
    CALL print_hex
    LD A, ' '
    OUT ($23), A
    INC HL
    DJNZ .print_cells
    LD A, 10
    OUT ($23), A

    ; === Step 3: Print genome bytes ===
    LD A, ' '
    OUT ($23), A
    LD HL, str_genome
    CALL print_str
    LD A, (ga_child_len)
    LD B, A
    OR A
    JR Z, .test_empty
    LD HL, GA_SCRATCH
.test_print:
    LD A, (HL)
    CALL print_hex
    LD A, ' '
    OUT ($23), A
    INC HL
    DJNZ .test_print

.test_empty:
    ; Print length
    LD A, '('
    OUT ($23), A
    LD A, (ga_child_len)
    CALL print_dec
    LD A, 'B'
    OUT ($23), A
    LD A, ')'
    OUT ($23), A
    LD A, 10
    OUT ($23), A
    LD A, 10
    OUT ($23), A

    POP BC
    DEC C
    JR NZ, .test_loop

    DI
    HALT

; ============================================================================
; Print helpers
; ============================================================================

print_hex:
    PUSH AF
    SRL A
    SRL A
    SRL A
    SRL A
    CALL .ph_nibble
    POP AF
    AND $0F
    CALL .ph_nibble
    RET
.ph_nibble:
    CP 10
    JR C, .ph_digit
    ADD A, 'A' - 10
    OUT ($23), A
    RET
.ph_digit:
    ADD A, '0'
    OUT ($23), A
    RET

print_dec:
    LD B, 0
    LD C, 0
.pd_hundreds:
    CP 100
    JR C, .pd_tens
    SUB 100
    INC B
    JR .pd_hundreds
.pd_tens:
    CP 10
    JR C, .pd_units
    SUB 10
    INC C
    JR .pd_tens
.pd_units:
    PUSH AF
    LD A, B
    OR A
    JR Z, .pd_skip_h
    ADD A, '0'
    OUT ($23), A
.pd_skip_h:
    LD A, B
    OR C
    JR Z, .pd_skip_t
    LD A, C
    ADD A, '0'
    OUT ($23), A
.pd_skip_t:
    POP AF
    ADD A, '0'
    OUT ($23), A
    RET

print_str:
    LD A, (HL)
    OR A
    RET Z
    OUT ($23), A
    INC HL
    JR print_str

; ============================================================================
; LFSR
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

lfsr_state: DW $ACE1

; ============================================================================
; GA state
; ============================================================================
ga_child_len: DB 0

; ============================================================================
; Includes
; ============================================================================
    INCLUDE "wfc_biome.asm"
    INCLUDE "wfc_genome.asm"

; ============================================================================
; Data
; ============================================================================
str_header: DB "=== WFC Genome Test ===", 10, 0
str_cells:  DB "cells: ", 0
str_genome: DB "bytes: ", 0
