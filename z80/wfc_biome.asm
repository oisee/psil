; ============================================================================
; WFC Biome Generation for Z80
; Runs at 16x12 half resolution, expands 2x2 to 32x24 biome grid
; ============================================================================
;
; 7 biome types (bits 0-6):
;   0 = Clearing  (.)    high food rate
;   1 = Forest    (T)    medium food, tool drops
;   2 = Mountain  (^)    low food, weapon/crystal
;   3 = Swamp     (~)    risky food
;   4 = Village   (H)    tool/treasure
;   5 = River     (=)    medium food
;   6 = Bridge    (#)    connector
;
; Each WFC cell = 1 byte bitmask of possible biomes ($7F = all possible)
; Work area: WFC_WORK ($6600), 16x12 = 192 bytes

WFC_W       EQU 16
WFC_H       EQU 12
WFC_CELLS   EQU WFC_W * WFC_H        ; 192
WFC_WORK    EQU $6600                 ; 192 bytes work grid
WFC_ALL     EQU $7F                   ; all 7 biomes possible

; Biome type constants
BIOME_CLEARING EQU 0
BIOME_FOREST   EQU 1
BIOME_MOUNTAIN EQU 2
BIOME_SWAMP    EQU 3
BIOME_VILLAGE  EQU 4
BIOME_RIVER    EQU 5
BIOME_BRIDGE   EQU 6

; ============================================================================
; generate_biomes: Run WFC to fill BIOME_GRID
; ============================================================================
generate_biomes:
    ; Initialize popcount LUT
    CALL init_popcount

    ; Initialize WFC work grid: all cells = $7F (all possible)
    LD HL, WFC_WORK
    LD A, WFC_ALL
    LD B, WFC_CELLS
.wfc_init:
    LD (HL), A
    INC HL
    DJNZ .wfc_init

    ; Place anchors to seed the collapse
    ; Center-ish: village
    CALL lfsr_next
    AND $07
    ADD A, 4                   ; X = 4-11
    LD C, A
    CALL lfsr_next
    AND $03
    ADD A, 4                   ; Y = 4-7
    LD D, A
    LD B, 1 << BIOME_VILLAGE   ; $10
    CALL wfc_set_cell

    ; Corner: clearing
    LD C, 0
    LD D, 0
    LD B, 1 << BIOME_CLEARING  ; $01
    CALL wfc_set_cell

    ; Propagate constraints from anchors
    CALL wfc_propagate

    ; Main WFC loop: find min-entropy cell, collapse, propagate
.wfc_loop:
    CALL wfc_find_min_entropy
    LD A, (wfc_min_entropy)
    CP 8                       ; sentinel = no uncollapsed cell found
    JR NC, .wfc_done
    CP 2
    JR C, .wfc_done            ; entropy 0 or 1 = done or contradiction

    ; Collapse: pick random biome from possibilities
    LD A, (wfc_min_x)
    LD C, A
    LD A, (wfc_min_y)
    LD D, A
    CALL wfc_collapse_cell

    ; Propagate
    CALL wfc_propagate
    JR .wfc_loop

.wfc_done:
    ; Expand 16x12 -> 32x24 into BIOME_GRID
    CALL wfc_expand

    ; Print biome grid
    CALL print_biome_grid

    LD A, 10
    OUT ($23), A               ; trailing newline
    RET

; ============================================================================
; wfc_set_cell: Set WFC cell at (C,D) to bitmask B
; C = X (0-15), D = Y (0-11)
; ============================================================================
wfc_set_cell:
    PUSH HL
    PUSH DE
    ; Offset = D * 16 + C
    LD A, D
    RLCA
    RLCA
    RLCA
    RLCA                       ; A = D * 16
    ADD A, C
    LD HL, WFC_WORK
    LD E, A
    LD D, 0
    ADD HL, DE
    LD (HL), B
    POP DE
    POP HL
    RET

; wfc_get_cell: A = WFC cell at (C,D). Preserves BC, DE.
wfc_get_cell:
    PUSH HL
    PUSH DE
    LD A, D
    RLCA
    RLCA
    RLCA
    RLCA
    ADD A, C
    LD HL, WFC_WORK
    LD E, A
    LD D, 0
    ADD HL, DE
    LD A, (HL)
    POP DE
    POP HL
    RET

; ============================================================================
; wfc_propagate: Propagate constraints across entire grid
; Repeats until no changes occur
; ============================================================================
wfc_propagate:
    XOR A
    LD (wfc_changed), A

    ; For each cell, constrain by neighbors
    LD D, 0                    ; Y
.wp_row:
    LD C, 0                    ; X
.wp_col:
    CALL wfc_get_cell
    LD (wfc_cur_poss), A
    ; Check if already collapsed (popcount = 1 or 0)
    CALL popcount
    CP 2
    JP C, .wp_next             ; 0 or 1 = skip

    ; Constrain by 4 neighbors
    LD B, WFC_ALL              ; accumulated allowed starts as all

    ; North (C, D-1)
    LD A, D
    OR A
    JR Z, .wp_east             ; at top edge, skip (B stays WFC_ALL)
    PUSH BC
    DEC D
    CALL wfc_get_cell
    INC D                      ; restore D
    CALL get_allowed
    POP BC
    AND B
    LD B, A

.wp_east:
    ; East (C+1, D)
    LD A, C
    CP WFC_W - 1
    JR NC, .wp_south           ; at right edge, skip
    PUSH BC
    INC C
    CALL wfc_get_cell
    DEC C                      ; restore C
    CALL get_allowed
    POP BC
    AND B
    LD B, A

.wp_south:
    ; South (C, D+1)
    LD A, D
    CP WFC_H - 1
    JR NC, .wp_west            ; at bottom edge, skip
    PUSH BC
    INC D
    CALL wfc_get_cell
    DEC D                      ; restore D
    CALL get_allowed
    POP BC
    AND B
    LD B, A

.wp_west:
    ; West (C-1, D)
    LD A, C
    OR A
    JR Z, .wp_apply            ; at left edge, skip
    PUSH BC
    DEC C
    CALL wfc_get_cell
    INC C                      ; restore C
    CALL get_allowed
    POP BC
    AND B
    LD B, A

.wp_apply:
    ; New possibilities = old AND accumulated allowed
    LD A, (wfc_cur_poss)
    AND B
    LD B, A
    ; If changed, update and flag
    LD A, (wfc_cur_poss)
    CP B
    JR Z, .wp_next
    ; Changed! Write back
    CALL wfc_set_cell
    LD A, 1
    LD (wfc_changed), A

.wp_next:
    INC C
    LD A, C
    CP WFC_W
    JP NZ, .wp_col
    INC D
    LD A, D
    CP WFC_H
    JP NZ, .wp_row

    ; Repeat if anything changed
    LD A, (wfc_changed)
    OR A
    JP NZ, wfc_propagate
    RET

wfc_changed:  DB 0
wfc_cur_poss: DB 0

; ============================================================================
; get_allowed: Given source cell possibilities in A,
;              return union of allowed neighbor biomes in A
; Clobbers: HL, E, B
; ============================================================================
get_allowed:
    LD HL, constraint_table
    LD E, A                    ; E = source possibilities
    XOR A                      ; accumulator = 0
    LD B, 7                    ; 7 biome bits
.ga_loop:
    RRC E                      ; bit into carry
    JR NC, .ga_skip
    OR (HL)                    ; OR in constraint for this biome
.ga_skip:
    INC HL
    DJNZ .ga_loop
    RET

; Constraint table: for each biome, which biomes can be adjacent
;         bits: 6543210 = Bridge,River,Village,Swamp,Mountain,Forest,Clearing
constraint_table:
    DB %01110010    ; Clearing(0): Forest, Village, River, Bridge
    DB %01101101    ; Forest(1):   Clearing, Mountain, Swamp, River, Bridge
    DB %00010010    ; Mountain(2): Forest, Village
    DB %00100010    ; Swamp(3):    Forest, River
    DB %01000101    ; Village(4):  Clearing, Mountain, Bridge
    DB %01001011    ; River(5):    Clearing, Forest, Swamp, Bridge
    DB %00110011    ; Bridge(6):   Clearing, Forest, Village, River

; ============================================================================
; wfc_find_min_entropy: Find uncollapsed cell with fewest possibilities
; Sets wfc_min_x, wfc_min_y, wfc_min_entropy
; If all cells collapsed, wfc_min_entropy stays at 8 (sentinel)
; ============================================================================
wfc_find_min_entropy:
    LD A, 8                    ; start with sentinel (> max 7)
    LD (wfc_min_entropy), A
    XOR A
    LD (wfc_min_x), A
    LD (wfc_min_y), A

    LD HL, WFC_WORK
    LD D, 0                    ; Y
.fme_row:
    LD C, 0                    ; X
.fme_col:
    LD A, (HL)
    PUSH HL
    CALL popcount              ; A = number of set bits
    POP HL
    CP 2
    JR C, .fme_skip            ; 0 or 1 = already collapsed
    LD B, A
    LD A, (wfc_min_entropy)
    CP B
    JR C, .fme_skip            ; current min < this entropy
    JR Z, .fme_skip            ; equal = skip (first wins)
    LD A, B
    LD (wfc_min_entropy), A
    LD A, C
    LD (wfc_min_x), A
    LD A, D
    LD (wfc_min_y), A
.fme_skip:
    INC HL
    INC C
    LD A, C
    CP WFC_W
    JR NZ, .fme_col
    INC D
    LD A, D
    CP WFC_H
    JR NZ, .fme_row
    RET

wfc_min_entropy: DB 0
wfc_min_x:      DB 0
wfc_min_y:      DB 0

; ============================================================================
; wfc_collapse_cell: Collapse cell (C,D) to a random valid biome
; C = X, D = Y (set by caller, preserved for wfc_set_cell)
; ============================================================================
wfc_collapse_cell:
    CALL wfc_get_cell          ; A = possibilities bitmask
    PUSH BC                    ; save C (X coord)
    PUSH DE                    ; save D (Y coord)

    PUSH AF                    ; save possibilities
    CALL popcount              ; A = count of set bits
    LD B, A                    ; B = count

    PUSH BC                    ; save count
    CALL lfsr_next             ; A = random byte
    POP BC                     ; B = count

    ; A mod B → target index
.cc_mod:
    CP B
    JR C, .cc_mod_done
    SUB B
    JR .cc_mod
.cc_mod_done:
    LD B, A                    ; B = target index (0-indexed)
    POP AF                     ; A = possibilities bitmask

    LD E, 1                    ; E = current single-bit mask
.cc_find:
    SRL A                      ; shift bit 0 into carry
    JR NC, .cc_skip            ; bit not set, advance mask
    ; Bit is set — is this our target?
    LD D, A                    ; save remaining shifted possibilities
    LD A, B
    OR A
    JR Z, .cc_found            ; target index == 0 → found!
    DEC B
    LD A, D                    ; restore shifted possibilities
.cc_skip:
    SLA E                      ; advance bit mask
    JR .cc_find                ; guaranteed to find (popcount >= 1)

.cc_found:
    ; E = single-bit mask for chosen biome
    ; Stack has: [saved DE (#2)] [saved BC (#1)] [retaddr]
    LD A, E                    ; A = result bitmask
    POP DE                     ; restore D = Y (matches PUSH DE #2)
    POP BC                     ; restore C = X (matches PUSH BC #1)
    LD B, A                    ; B = result bitmask
    CALL wfc_set_cell          ; set cell (C, D) = B
    RET

; ============================================================================
; wfc_expand: Expand 16x12 WFC grid -> 32x24 BIOME_GRID (2x2 per cell)
; ============================================================================
wfc_expand:
    LD HL, WFC_WORK
    LD D, 0                    ; src Y
.we_row:
    LD C, 0                    ; src X
.we_col:
    LD A, (HL)
    PUSH HL
    ; Convert bitmask to biome index (find lowest set bit)
    CALL bitmask_to_index      ; A = biome index 0-6
    ; Write to 4 cells in BIOME_GRID at (C*2, D*2)
    LD B, A                    ; B = biome value
    PUSH BC
    PUSH DE
    LD A, C
    ADD A, A                   ; X*2
    LD C, A
    LD A, D
    ADD A, A                   ; Y*2
    LD E, A
    ; Write (C, E)
    CALL biome_write
    ; Write (C+1, E)
    INC C
    CALL biome_write
    ; Write (C, E+1)
    DEC C
    INC E
    CALL biome_write
    ; Write (C+1, E+1)
    INC C
    CALL biome_write
    POP DE
    POP BC
    POP HL

    INC HL
    INC C
    LD A, C
    CP WFC_W
    JR NZ, .we_col
    INC D
    LD A, D
    CP WFC_H
    JR NZ, .we_row
    RET

; biome_write: Write biome B at grid position (C, E)
; C = X (0-31), E = Y (0-23)
biome_write:
    PUSH HL
    PUSH DE
    ; Offset = E * 32 + C
    LD H, 0
    LD L, E
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL                 ; HL = Y * 32
    LD D, 0
    LD A, C
    LD E, A
    ADD HL, DE
    LD DE, BIOME_GRID
    ADD HL, DE
    LD (HL), B
    POP DE
    POP HL
    RET

; bitmask_to_index: Convert bitmask in A to bit index of lowest set bit
; Returns A = index (0-6). Returns 0 if no bits set.
bitmask_to_index:
    OR A
    RET Z                      ; A=0 → return 0
    LD B, 0
.bti_loop:
    SRL A                      ; shift bit 0 into carry
    JR C, .bti_done            ; carry set = found lowest bit
    INC B
    JR .bti_loop
.bti_done:
    LD A, B
    RET

; ============================================================================
; popcount: Count set bits in A. Returns count in A.
; Uses LUT at POPCOUNT_LUT ($7000) if initialized, else manual.
; ============================================================================
popcount:
    LD B, A
    LD A, (popcount_ready)
    OR A
    LD A, B
    JR Z, .pc_manual
    ; LUT lookup
    PUSH HL
    PUSH DE
    LD HL, POPCOUNT_LUT
    LD E, A
    LD D, 0
    ADD HL, DE
    LD A, (HL)
    POP DE
    POP HL
    RET
.pc_manual:
    ; Manual popcount
    LD B, 0
    LD C, 8
.pc_loop:
    RRCA
    JR NC, .pc_noinc
    INC B
.pc_noinc:
    DEC C
    JR NZ, .pc_loop
    LD A, B
    RET

POPCOUNT_LUT EQU $7000
popcount_ready: DB 0

; init_popcount: Fill popcount LUT (256 bytes at $7000)
init_popcount:
    LD HL, POPCOUNT_LUT
    LD C, 0                    ; value 0-255
.ip_loop:
    LD A, C
    ; Count bits in A
    LD B, 0
    LD D, 8
.ip_bits:
    RRCA
    JR NC, .ip_noinc
    INC B
.ip_noinc:
    DEC D
    JR NZ, .ip_bits
    LD (HL), B
    INC HL
    INC C
    JR NZ, .ip_loop            ; loops 256 times (C wraps to 0)
    LD A, 1
    LD (popcount_ready), A
    RET

; ============================================================================
; print_biome_grid: Print 32x24 biome grid as characters
; ============================================================================
print_biome_grid:
    LD E, 0                    ; Y
.pbg_row:
    LD C, 0                    ; X
.pbg_col:
    ; Read biome at (C, E)
    PUSH DE
    PUSH BC
    LD H, 0
    LD L, E
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL
    ADD HL, HL                 ; HL = Y * 32
    LD D, 0
    LD E, C
    ADD HL, DE
    LD DE, BIOME_GRID
    ADD HL, DE
    LD A, (HL)
    POP BC
    POP DE
    ; Convert biome index to char
    PUSH HL
    LD HL, biome_chars
    LD D, 0
    PUSH DE
    LD E, A
    ADD HL, DE
    LD A, (HL)
    POP DE
    POP HL
    OUT ($23), A
    INC C
    LD A, C
    CP WORLD_SIZE_X
    JR NZ, .pbg_col
    LD A, 10                   ; newline
    OUT ($23), A
    INC E
    LD A, E
    CP WORLD_SIZE_Y
    JR NZ, .pbg_row
    RET

biome_chars: DB ".T^~H=#"    ; Clearing, Forest, Mountain, Swamp, Village, River, Bridge
