; ============================================================================
; micro-PSIL Bytecode VM for Z80 (ZX Spectrum baremetal)
; ============================================================================
;
; Build:  sjasmplus z80/micro_psil_vm.asm --raw=z80/build/vm.bin
;
; Run (no quotations):
;   mzx --run z80/build/vm.bin@8000 \
;       --load z80/build/arithmetic.bin@9000 \
;       --console-io --frames DI:HALT
;
; Run (with quotations):
;   mzx --run z80/build/vm.bin@8000 \
;       --load z80/build/factorial.bin@9000 \
;       --load z80/build/factorial_quots.bin@9200 \
;       --console-io --frames DI:HALT
;
; I/O: Uses OUT ($23),A via --console-io (no ROM needed).
;
; Memory map:
;   $8000-$8FFF  VM code (this file)
;   $9000-$91FF  Main bytecode program
;   $9200-$97FF  Quotation binary blob
;   $B000-$B0FF  VM value stack (128 entries * 2 bytes)
;   $B100-$B17F  VM memory slots (64 * 2 bytes)
;   $B180-$B1BF  Quotation pointer table (32 * 2 bytes)
;
; ============================================================================

    ORG $8000

BYTECODE    EQU $9000
QUOT_BLOB   EQU $9200
VM_STACK    EQU $B000
VM_MEM      EQU $B100
QUOT_TBL    EQU $B180

; ── Entry ──
entry:
    DI
    LD SP, $FF00

    ; Init VM stack
    LD HL, VM_STACK
    LD (vm_sp), HL

    ; Clear memory slots
    LD HL, VM_MEM
    LD DE, VM_MEM + 1
    LD BC, 127
    LD (HL), 0
    LDIR

    ; Clear quotation table
    LD HL, QUOT_TBL
    LD DE, QUOT_TBL + 1
    LD BC, 63
    LD (HL), 0
    LDIR

    ; Parse quotations
    CALL parse_quots

    ; Set bytecode PC to start of program
    LD HL, BYTECODE
    LD (bc_pc), HL

    ; Run
    CALL vm_run

    DI
    HALT

; ============================================================================
; parse_quots
; ============================================================================
parse_quots:
    LD HL, QUOT_BLOB
    LD A, (HL)
    OR A
    RET Z
    CP $FF
    RET Z
    CP 33
    RET NC                  ; Max 32 quotations

    LD B, A
    LD (pq_n), A
    INC HL

    ; Body base = HL + n*2
    PUSH HL
    LD C, A
    ADD A, A
    LD C, A
    LD B, 0
    ADD HL, BC
    LD (pq_body), HL

    ; Build table
    POP HL
    LD A, (pq_n)
    LD B, A
    LD IX, QUOT_TBL

.pq_loop:
    LD E, (HL)
    INC HL
    LD D, (HL)
    INC HL
    PUSH HL

    LD HL, (pq_body)
    LD (IX+0), L
    LD (IX+1), H
    ADD HL, DE
    LD (pq_body), HL

    INC IX
    INC IX

    POP HL
    DJNZ .pq_loop
    RET

pq_n:    DB 0
pq_body: DW 0

; ============================================================================
; vm_run: Main fetch-decode-execute loop
; Bytecode pointer in (bc_pc). Returns on halt/end.
; ============================================================================
vm_run:
    ; Fetch opcode
    LD HL, (bc_pc)
    LD A, (HL)
    INC HL
    LD (bc_pc), HL

    ; Halt/end
    CP $F0
    RET Z
    CP $FF
    RET Z

    ; Dispatch
    CP $20
    JP C, do_cmd
    CP $40
    JR C, .num
    CP $60
    JR C, .sym
    CP $80
    JR C, .quot
    CP $C0
    JP C, do_2byte
    CP $E0
    JR C, .op3

    ; Skip variable-length and special
    JP vm_run

.num:
    SUB $20
    LD E, A
    LD D, 0
    CALL push_w
    JP vm_run

.sym:
    SUB $40
    LD E, A
    LD D, 0
    CALL push_w
    JP vm_run

.quot:
    SUB $60
    LD E, A
    LD D, $80              ; index | $8000
    CALL push_w
    JP vm_run

.op3:
    CP $C0
    JR NZ, .skip3
    ; push.w [hi][lo]
    LD HL, (bc_pc)
    LD D, (HL)
    INC HL
    LD E, (HL)
    INC HL
    LD (bc_pc), HL
    CALL push_w
    JP vm_run
.skip3:
    ; Skip 2 argument bytes
    LD HL, (bc_pc)
    INC HL
    INC HL
    LD (bc_pc), HL
    JP vm_run

; ============================================================================
; do_cmd: 1-byte command dispatch (A = 0x00-0x1F)
; ============================================================================
do_cmd:
    LD HL, op_tbl
    ADD A, A
    LD C, A
    LD B, 0
    ADD HL, BC
    LD E, (HL)
    INC HL
    LD D, (HL)
    EX DE, HL

    ; Push return address then jump to handler
    LD DE, .cmd_ret
    PUSH DE
    JP (HL)

.cmd_ret:
    ; Check for quotation 'ret'
    LD A, (vm_retf)
    OR A
    RET NZ
    JP vm_run

; ============================================================================
; do_2byte: 2-byte opcode dispatch
; ============================================================================
do_2byte:
    LD C, A
    LD HL, (bc_pc)
    LD B, (HL)
    INC HL
    LD (bc_pc), HL

    LD A, C
    CP $80 : JR Z, .pb
    CP $89 : JR Z, .call
    CP $85 : JR Z, .jmp
    CP $86 : JR Z, .jmpb
    CP $87 : JR Z, .jz
    CP $88 : JR Z, .jnz
    CP $82 : JR Z, .qx
    CP $81 : JR Z, .sx
    CP $83 : JR Z, .loc
    CP $84 : JR Z, .sloc
    JP vm_run

.pb:    ; push.b
    LD E, B
    LD D, 0
    CALL push_w
    JP vm_run

.call:  ; call builtin
    LD A, B
    CALL do_builtin
    JP vm_run

.jmp:   ; jump forward
    LD HL, (bc_pc)
    LD C, B
    LD B, 0
    ADD HL, BC
    LD (bc_pc), HL
    JP vm_run

.jmpb:  ; jump backward
    LD HL, (bc_pc)
    LD C, B
    LD B, 0
    OR A
    SBC HL, BC
    LD (bc_pc), HL
    JP vm_run

.jz:    ; jump if zero
    CALL pop_w
    LD A, D
    OR E
    JP NZ, vm_run
    LD HL, (bc_pc)
    LD C, B
    LD B, 0
    ADD HL, BC
    LD (bc_pc), HL
    JP vm_run

.jnz:   ; jump if not zero
    CALL pop_w
    LD A, D
    OR E
    JP Z, vm_run
    LD HL, (bc_pc)
    LD C, B
    LD B, 0
    ADD HL, BC
    LD (bc_pc), HL
    JP vm_run

.qx:    ; extended quotation ref
    LD E, B
    LD D, $80
    CALL push_w
    JP vm_run

.sx:    ; extended symbol read
    LD A, B
    CALL mem_rd
    CALL push_w
    JP vm_run

.loc:   ; read local
    LD A, B
    ADD A, A
    LD HL, VM_MEM + 128
    LD C, A
    LD B, 0
    ADD HL, BC
    LD E, (HL)
    INC HL
    LD D, (HL)
    CALL push_w
    JP vm_run

.sloc:  ; set local
    PUSH BC
    CALL pop_w
    POP BC
    LD A, B
    ADD A, A
    LD HL, VM_MEM + 128
    LD C, A
    LD B, 0
    ADD HL, BC
    LD (HL), E
    INC HL
    LD (HL), D
    JP vm_run

; ============================================================================
; Op table (32 entries, 0x00-0x1F)
; ============================================================================
op_tbl:
    DW op_nop, op_dup, op_drop, op_swap       ; 00-03
    DW op_over, op_rot, op_add, op_sub         ; 04-07
    DW op_mul, op_div, op_mod, op_eq           ; 08-0B
    DW op_lt, op_gt, op_and, op_or             ; 0C-0F
    DW op_not, op_neg, op_exec, op_ifte        ; 10-13
    DW op_dip, op_loop, op_ret, op_load        ; 14-17
    DW op_store, op_print, op_inc, op_dec      ; 18-1B
    DW op_dup2, op_nop, op_depth, op_clear     ; 1C-1F

; ============================================================================
; Stack operations
; ============================================================================

; push_w: Push DE onto VM stack
push_w:
    LD HL, (vm_sp)
    LD (HL), E
    INC HL
    LD (HL), D
    INC HL
    LD (vm_sp), HL
    RET

; pop_w: Pop into DE
pop_w:
    LD HL, (vm_sp)
    DEC HL
    LD D, (HL)
    DEC HL
    LD E, (HL)
    LD (vm_sp), HL
    RET

; peek_w: Read top into DE (no pop)
peek_w:
    LD HL, (vm_sp)
    DEC HL
    LD D, (HL)
    DEC HL
    LD E, (HL)
    RET

; ============================================================================
; Operations (all RET)
; ============================================================================

op_nop: RET

op_dup:
    CALL peek_w
    JP push_w

op_drop:
    LD HL, (vm_sp)
    DEC HL
    DEC HL
    LD (vm_sp), HL
    RET

op_swap:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD (t2), DE
    LD DE, (t1)
    CALL push_w
    LD DE, (t2)
    JP push_w

op_over:
    LD HL, (vm_sp)
    DEC HL
    DEC HL
    DEC HL
    DEC HL
    LD E, (HL)
    INC HL
    LD D, (HL)
    JP push_w

op_rot:
    CALL pop_w
    LD (t1), DE             ; c
    CALL pop_w
    LD (t2), DE             ; b
    CALL pop_w
    LD (t3), DE             ; a
    LD DE, (t2)
    CALL push_w             ; b
    LD DE, (t1)
    CALL push_w             ; c
    LD DE, (t3)
    JP push_w               ; a

; ── Arithmetic ──

op_add:
    CALL pop_w              ; b
    LD (t1), DE
    CALL pop_w              ; a
    LD HL, (t1)
    ADD HL, DE
    EX DE, HL
    JP push_w

op_sub:
    CALL pop_w              ; b
    LD (t1), DE
    CALL pop_w              ; a → DE
    LD HL, (t1)             ; HL = b
    EX DE, HL               ; HL = a, DE = b
    OR A
    SBC HL, DE
    EX DE, HL
    JP push_w

op_mul:
    CALL pop_w              ; b
    LD (t1), DE
    CALL pop_w              ; a → DE
    LD BC, (t1)
    CALL mul16              ; HL = DE * BC
    EX DE, HL
    JP push_w

op_div:
    CALL pop_w              ; b
    LD A, D
    OR E
    JR NZ, .dv_ok
    CALL pop_w
    LD DE, 0
    JP push_w
.dv_ok:
    LD (t1), DE
    CALL pop_w              ; a
    LD BC, (t1)
    CALL div16s
    JP push_w

op_mod:
    CALL pop_w              ; b
    LD A, D
    OR E
    JR NZ, .md_ok
    CALL pop_w
    LD DE, 0
    JP push_w
.md_ok:
    LD (t1), DE
    CALL pop_w              ; a
    LD BC, (t1)
    CALL mod16s
    JP push_w

; ── Comparison ──

op_eq:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD HL, (t1)
    LD A, E
    CP L
    JR NZ, .eq_n
    LD A, D
    CP H
    JR NZ, .eq_n
    LD DE, 1
    JP push_w
.eq_n:
    LD DE, 0
    JP push_w

op_lt:
    CALL pop_w              ; b
    LD (t1), DE
    CALL pop_w              ; a
    LD HL, (t1)             ; HL = b
    CALL cmp_s              ; C if DE < HL
    JR C, .lt_y
    LD DE, 0
    JP push_w
.lt_y:
    LD DE, 1
    JP push_w

op_gt:
    CALL pop_w              ; b
    LD (t1), DE
    CALL pop_w              ; a
    LD HL, (t1)             ; HL = b
    EX DE, HL               ; DE=b, HL=a
    CALL cmp_s              ; C if DE < HL (b < a)
    JR C, .gt_y
    LD DE, 0
    JP push_w
.gt_y:
    LD DE, 1
    JP push_w

; ── Logic ──

op_and:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD HL, (t1)
    LD A, E : AND L : LD E, A
    LD A, D : AND H : LD D, A
    JP push_w

op_or:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD HL, (t1)
    LD A, E : OR L : LD E, A
    LD A, D : OR H : LD D, A
    JP push_w

op_not:
    CALL pop_w
    LD A, D
    OR E
    JR NZ, .nt_z
    LD DE, 1
    JP push_w
.nt_z:
    LD DE, 0
    JP push_w

op_neg:
    CALL pop_w
    CALL neg_de
    JP push_w

; ── Control flow ──

op_exec:
    CALL pop_w
    LD A, E
    AND $7F
    LD E, A
    LD D, 0
    JP exec_qt

op_ifte:
    CALL pop_w              ; else_q
    LD (t1), DE
    CALL pop_w              ; then_q
    LD (t2), DE
    CALL pop_w              ; cond
    LD A, D
    OR E
    JR Z, .if_f
    LD DE, (t2)
    JR .if_go
.if_f:
    LD DE, (t1)
.if_go:
    LD A, E
    AND $7F
    LD E, A
    LD D, 0
    JP exec_qt

op_dip:
    CALL pop_w              ; q
    LD (t1), DE
    CALL pop_w              ; x
    LD (t2), DE
    LD DE, (t1)
    LD A, E
    AND $7F
    LD E, A
    LD D, 0
    CALL exec_qt
    LD DE, (t2)
    JP push_w

op_loop:
    CALL pop_w              ; q
    LD A, E
    AND $7F
    LD (lp_q), A
    CALL pop_w              ; n
    LD (lp_n), DE
.lp_i:
    LD DE, (lp_n)
    LD A, D
    OR E
    RET Z
    DEC DE
    LD (lp_n), DE
    LD A, (lp_q)
    LD E, A
    LD D, 0
    CALL exec_qt
    JR .lp_i

lp_q: DB 0
lp_n: DW 0

op_ret:
    LD A, 1
    LD (vm_retf), A
    RET

; ── Memory ──

op_load:
    CALL pop_w
    LD A, E
    CALL mem_rd
    JP push_w

op_store:
    CALL pop_w              ; slot
    LD (t1), DE
    CALL pop_w              ; value
    LD A, (t1)
    JP mem_wr

; ── I/O ──

op_print:
    CALL pop_w
    JP pr_s16

op_inc:
    CALL pop_w
    INC DE
    JP push_w

op_dec:
    CALL pop_w
    DEC DE
    JP push_w

op_dup2:
    LD HL, (vm_sp)
    DEC HL : DEC HL : DEC HL : DEC HL
    LD E, (HL) : INC HL : LD D, (HL)
    CALL push_w
    LD HL, (vm_sp)
    DEC HL : DEC HL : DEC HL : DEC HL
    LD E, (HL) : INC HL : LD D, (HL)
    JP push_w

op_depth:
    LD HL, (vm_sp)
    LD DE, VM_STACK
    OR A
    SBC HL, DE
    SRL H
    RR L
    EX DE, HL
    JP push_w

op_clear:
    LD HL, VM_STACK
    LD (vm_sp), HL
    RET

; ============================================================================
; exec_qt: Execute quotation by index
; DE = index (0-31). Saves/restores bc_pc.
; ============================================================================
exec_qt:
    LD A, E
    ADD A, A
    LD HL, QUOT_TBL
    LD C, A
    LD B, 0
    ADD HL, BC
    LD E, (HL)
    INC HL
    LD D, (HL)

    ; Check valid
    LD A, D
    OR E
    RET Z

    ; Save current PC
    LD HL, (bc_pc)
    PUSH HL

    ; Set PC to quotation body
    LD (bc_pc), DE

    ; Clear ret flag
    XOR A
    LD (vm_retf), A

    ; Execute (vm_run returns on halt/end/ret)
    CALL vm_run

    ; Clear ret flag
    XOR A
    LD (vm_retf), A

    ; Restore PC
    POP HL
    LD (bc_pc), HL
    RET

; ============================================================================
; Memory operations
; ============================================================================

; mem_rd: A = slot → DE = value
mem_rd:
    ADD A, A
    LD HL, VM_MEM
    LD C, A
    LD B, 0
    ADD HL, BC
    LD E, (HL)
    INC HL
    LD D, (HL)
    RET

; mem_wr: A = slot, DE = value
mem_wr:
    ADD A, A
    LD HL, VM_MEM
    LD C, A
    LD B, 0
    ADD HL, BC
    LD (HL), E
    INC HL
    LD (HL), D
    RET

; ============================================================================
; Builtins
; ============================================================================
do_builtin:
    OR A
    JR Z, .bi_nl
    CP 1 : JR Z, .bi_sp
    CP 2 : JR Z, .bi_ch
    CP 3 : JR Z, .bi_abs
    CP 4 : JR Z, .bi_min
    CP 5 : JR Z, .bi_max
    RET
.bi_nl:
    LD A, 10
    OUT ($23), A
    RET
.bi_sp:
    LD A, ' '
    OUT ($23), A
    RET
.bi_ch:
    CALL pop_w
    LD A, E
    OUT ($23), A
    RET
.bi_abs:
    CALL pop_w
    BIT 7, D
    JP Z, push_w
    CALL neg_de
    JP push_w
.bi_min:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD HL, (t1)
    CALL cmp_s
    JP C, push_w
    EX DE, HL
    JP push_w
.bi_max:
    CALL pop_w
    LD (t1), DE
    CALL pop_w
    LD HL, (t1)
    CALL cmp_s
    JP NC, push_w
    EX DE, HL
    JP push_w

; ============================================================================
; pr_s16: Print signed 16-bit DE via OUT ($23)
; ============================================================================
pr_s16:
    BIT 7, D
    JR Z, .ps_p
    LD A, '-'
    OUT ($23), A
    CALL neg_de
.ps_p:
    LD (pr_v), DE
    XOR A
    LD (pr_lz), A

    LD BC, 10000 : CALL .ps_d
    LD BC, 1000  : CALL .ps_d
    LD BC, 100   : CALL .ps_d
    LD BC, 10    : CALL .ps_d

    ; Last digit
    LD HL, (pr_v)
    LD A, L
    ADD A, '0'
    OUT ($23), A
    RET

.ps_d:
    LD HL, (pr_v)
    LD A, '0' - 1
.ps_lp:
    INC A
    OR A                    ; Clear carry
    SBC HL, BC
    JR NC, .ps_lp
    ADD HL, BC
    LD (pr_v), HL

    CP '0'
    JR NZ, .ps_nz
    ; Leading zero?
    PUSH AF
    LD A, (pr_lz)
    OR A
    JR NZ, .ps_pr
    POP AF
    RET                     ; Skip leading zero
.ps_pr:
    POP AF
.ps_nz:
    PUSH AF
    LD A, 1
    LD (pr_lz), A
    POP AF
    OUT ($23), A
    RET

pr_v:  DW 0
pr_lz: DB 0

; ============================================================================
; Math routines
; ============================================================================

; mul16: HL = DE * BC (unsigned, low 16 bits)
mul16:
    LD HL, 0
    LD A, 16
.ml:
    ADD HL, HL
    RL E
    RL D
    JR NC, .ms
    ADD HL, BC
.ms:
    DEC A
    JR NZ, .ml
    RET

; div16u: DE / BC → DE = quotient, HL = remainder
div16u:
    LD HL, 0
    LD A, 16
.du:
    SLA E
    RL D
    RL L
    RL H
    OR A
    SBC HL, BC
    JR NC, .df
    ADD HL, BC
    JR .dn
.df:
    SET 0, E
.dn:
    DEC A
    JR NZ, .du
    RET

; div16s: signed DE / BC → DE = quotient
div16s:
    LD A, D
    XOR B
    PUSH AF
    BIT 7, D
    CALL NZ, neg_de
    BIT 7, B
    CALL NZ, neg_bc
    CALL div16u
    POP AF
    BIT 7, A
    RET Z
    JP neg_de

; mod16s: signed DE mod BC → DE = remainder
mod16s:
    LD A, D
    PUSH AF
    BIT 7, D
    CALL NZ, neg_de
    BIT 7, B
    CALL NZ, neg_bc
    CALL div16u
    EX DE, HL
    POP AF
    BIT 7, A
    RET Z
    JP neg_de

neg_de:
    LD A, E : CPL : LD E, A
    LD A, D : CPL : LD D, A
    INC DE
    RET

neg_bc:
    LD A, C : CPL : LD C, A
    LD A, B : CPL : LD B, A
    INC BC
    RET

; cmp_s: Signed compare DE vs HL. Carry if DE < HL.
cmp_s:
    LD A, D
    XOR H
    JP M, .cd
    LD A, D : CP H : RET NZ
    LD A, E : CP L
    RET
.cd:
    BIT 7, D
    JR Z, .cp
    SCF
    RET
.cp:
    OR A
    RET

; ============================================================================
; VM state
; ============================================================================
bc_pc:   DW BYTECODE        ; Bytecode program counter
vm_sp:   DW VM_STACK        ; VM value stack pointer
vm_retf: DB 0               ; Return flag

; Temporaries
t1: DW 0
t2: DW 0
t3: DW 0

vm_end:
