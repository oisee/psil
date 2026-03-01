# Implementation Plan: Emergent NPC Societies

**Status:** In Progress (Phases 1-1d DONE, Phase 3 DONE, Phase 4 DONE, Phase 5 DONE)
**Report:** [Emergent NPC Societies](../reports/2026-03-01-001-emergent-npc-societies.md)
**Principle:** Go first (host machine), then Z80 port. Each phase is testable and demoable independently.

---

## Phase 1: Trade & Inventory (Go) — DONE

**Goal:** NPCs can hold items, trade bilaterally, and accumulate gold. Evolution discovers barter.

### 1.1 Extend NPC struct

**File:** `pkg/sandbox/npc.go`

```go
type NPC struct {
    // ... existing fields ...
    Gold      int    // currency (0-255)
    Item      byte   // held item type (0=none, 1=food, 2=tool, 3=weapon, 4=treasure)
    Charisma  int    // memetic transmission power (0-255)
    Faction   byte   // emergent group ID (computed, not evolved)
}
```

### 1.2 Add Ring0/Ring1 slots

**File:** `pkg/sandbox/npc.go`

```go
// Extended Ring0
const (
    Ring0MyGold      = 14
    Ring0MyItem      = 15
    Ring0NearItem    = 16
    Ring0NearTrust   = 17
    Ring0ExtCount    = 24
)

// Extended Ring1 actions
const (
    ActionTrade = 4
    ActionTell  = 5
    ActionAsk   = 6
    ActionLie   = 7
    ActionTeach = 8
)
```

### 1.3 Extend scheduler

**File:** `pkg/sandbox/scheduler.go`

- `sense()`: Fill slots 14-16 from NPC state and nearest neighbor
- `act()`: Add `ActionTrade` handler — bilateral consent check (both NPCs must output trade on same tick targeting each other), swap items

### 1.4 Add item spawning

**File:** `pkg/sandbox/world.go`

- New tile types: `TileTool=3`, `TileWeapon=4`, `TileTreasure=5`
- Items spawn like food but rarer
- Picking up = eating but for items (walk onto tile)

### 1.5 Seed genomes

**Files:** `testdata/sandbox/trader.mpsil`, `testdata/sandbox/hoarder.mpsil`

- Trader: if holding item and near NPC with different item, trade; else forage
- Hoarder: pick up everything, never trade, flee

### 1.6 Tests

**File:** `pkg/sandbox/sandbox_test.go`

- `TestTradeExchange`: Two NPCs with items, both output trade, items swap
- `TestTradeRequiresBilateral`: One-sided trade attempt does nothing
- `TestItemPickup`: NPC walks onto tool tile, item field updates

### 1.7 Verification

```sh
go test ./pkg/sandbox/... -v -run "Trade|Item"
go run ./cmd/sandbox --npcs 20 --ticks 5000 --seed 42 --verbose  # observe trade events
```

---

## Phase 1a: MaxGenome + Per-NPC RNG (Go) — DONE

**Goal:** Increase genome capacity and give each NPC a deterministic PRNG.

- `MaxGenome` bumped from 64 → 128
- `NPC.RngState [3]byte` + `Rand() byte` (tribonacci, returns 0-31)
- Ring0 slot 20 (`Ring0Rng`) populated each tick
- RNG seeded from `[NPC.ID, tick_lo, tick_hi]` at spawn

---

## Phase 1b: Modifier Foundation + Crystal/Gas (Go) — DONE

**Goal:** Flat, fixed-size effect system. Items, tiles, and buffs share one `Modifier` struct.

- `Modifier{Kind, Mag, Duration, Source}` — 5 bytes per mod, `[4]Modifier` per NPC
- 10 modifier kinds: Gas, Forage, Attack, Defense, Energy, Health, Stealth, Trade, Stress
- `ModSum(kind)`, `AddMod(m)`, `RemoveMod(source)` on NPC
- `ItemModifiers` table: Tool→Forage+1, Weapon→Attack+10, Treasure→Trade+3
- `TileCrystal` (7): rare spawn (1-in-20), consumed on pickup → permanent ModGas+50
- Gas bonus with diminishing returns (50, 25, 12, 6...), cap 500
- `applyModifiers()` / `decayModifiers()` in tick loop
- Ring0 slots: 21=Stress, 22=MyGas

---

## Phase 1c: Forge Tiles + Solo Crafting (Go) — DONE

**Goal:** Crafting via forge tiles. Brains decide when to craft.

- `TileForge` (8): 1-2 placed at world creation, persistent
- `ActionCraft` (5): on forge + held item with recipe → swap item + modifier
- Recipes: Tool→Compass (ModForage+2), Weapon→Shield (ModDefense+5)
- Ring0 slot 23 (`Ring0OnForge`): 1 if on forge, 0 otherwise
- Forge tiles preserved through NPC movement and death

---

## Phase 1d: Trade Pricing + Stress (Go) — DONE

**Goal:** Scarcity-based economy and stress mechanic.

- `MarketValue(item)`: `10 * totalItems / thisTypeCount` (rarer = more valuable)
- Trade gold transfer: base reward + value difference / 2
- Stress system (0-100): attack→+15 target, starvation→+5/tick, eating→-2, trading→-5, resting→-1
- Stress output override: if stress > 30, `(stress-30)%` chance of random action
- Defense modifier reduces attack damage
- Fitness penalty: `-stress/5`

---

## Phase 3 (NPC sandbox): Max Age, Hazards, Memetic Transmission (Go) — DONE

**Goal:** Break forager monoculture with forced turnover, environmental hazards, and horizontal genome transfer.

- **Max age (5000 ticks)** — NPCs die of old age, forcing GA turnover
- **Poison tiles** — 1-in-10 item spawns are poison (15 damage, consumed), decay after 200 ticks
- **Blights** — every 1024 ticks, ~50% food destroyed
- **Memetic transmission (ActionTeach=6)** — adjacent NPCs copy 4-byte instruction-aligned genome fragments
  - Fitness-based probability: `teacher.Fitness / (teacher.Fitness + student.Fitness + 1)`
  - Costs 10 energy, requires adjacency
- **New sensors** — Ring0MyAge (slot 24), Ring0Taught (slot 25)
- **Teacher genome seeded** — 5% of population
- **Fitness formula** — added `+TeachCount*15`
- **Aged-out replacement in GA** — NPCs at MaxAge replaced even if not bottom 25%

See [Simulation Observations](../reports/2026-03-01-003-simulation-observations.md).

---

## Phase 4 (NPC sandbox): Decouple Tiles, Scale to 10,000 NPCs (Go) — DONE

**Goal:** Remove the 15-NPC ceiling from 4-bit tile occupant encoding. Scale to civilization-scale simulations.

- **Tiles = pure terrain** — `Tile byte` all 8 bits for type, `MakeTile(typ)` 1-arg, `Occupant()` deleted
- **Separate OccGrid** — `[]uint16` parallel to Grid with `OccAt()` / `SetOcc()` / `ClearOcc()`
- **NPC.ID = uint16** — monotonic, no wrapping, 65535 max
- **O(1) NPC lookup** — `npcByID map[uint16]*NPC`
- **Cached tile counts** — `foodCount` / `itemCount` maintained by `SetTile()`
- **Bounded Manhattan ring search** — all `Nearest*` scan rings (radius 0→31) instead of full grid
- **Combined NPC sensor** — `NearestNPCFull()` returns dist+ID+dir in one scan
- **Auto-scale** — `AutoWorldSize(npcs) = max(32, sqrt(npcs)*4)`, ~6% density
- **Resource scaling** — `MaxFood = npcs*3`, `MaxItems = max(npcs/2, 4)`

Performance: 100 NPCs in 3.5s, 1000 in 41s, 10000 completes 1k ticks in 69s. See [Scaling Report](../reports/2026-03-01-004-scaling-10k-npcs.md).

---

## Phase 2: Knowledge Buffer & Information Sharing (Go)

**Goal:** NPCs remember world facts, share them via tell/ask, and act on received knowledge.

### 2.1 Knowledge data structure

**File:** `pkg/sandbox/knowledge.go` (new)

```go
type KnowledgeType byte
const (
    KnowFoodAt    KnowledgeType = 1
    KnowDangerAt  KnowledgeType = 2
    KnowTreasureAt KnowledgeType = 3
    KnowNPCIs     KnowledgeType = 4
    KnowSecret    KnowledgeType = 5
    KnowRumor     KnowledgeType = 6
)

type KnowledgeItem struct {
    Type       KnowledgeType
    Arg1, Arg2 byte
    Confidence byte // decays each tick, 0 = forgotten
}

type KnowledgeBuffer struct {
    Items [8]KnowledgeItem
    Count int
}
```

### 2.2 Automatic knowledge acquisition

**File:** `pkg/sandbox/scheduler.go`

During `sense()`, NPCs automatically learn about their surroundings:
- See food within 3 tiles → add `FOOD_AT(x,y)` with confidence 255
- See NPC attack → add `DANGER_AT(x,y)` with confidence 200
- Each tick: all confidence values decay by 1

### 2.3 Tell and Ask actions

**File:** `pkg/sandbox/scheduler.go`

- `ActionTell`: Copy knowledge item `param` from own buffer to nearest NPC's buffer. Target gets item with `confidence = min(own_confidence, 200)`.
- `ActionAsk`: Set a flag on target NPC. If target outputs `ActionTell` next tick, information transfers.
- `ActionLie`: Generate fake `FOOD_AT(random_x, random_y)` and send to nearest NPC as if real.

### 2.4 Brain access to knowledge

**File:** `pkg/sandbox/scheduler.go`

Ring0 slots 11-13, 18-20:
- `known_food_dist`: Scan own knowledge buffer for nearest `FOOD_AT` item, compute distance
- `known_food_dir`: Direction toward it (1-4)
- `last_told_type`, `last_told_by`, `secret_count`: From NPC state

### 2.5 Lie detection (automatic)

When an NPC moves to a `FOOD_AT` location from its knowledge buffer and finds no food:
- Decrease trust in the NPC that told it (`trust[self][teller] -= 20`)
- Remove the knowledge item

### 2.6 Seed genomes

**Files:** `testdata/sandbox/scout.mpsil`, `testdata/sandbox/liar.mpsil`

- Scout: explore, when finding food and near ally (trust > 0), tell
- Liar: when near NPC with high trust toward self, lie about food location

### 2.7 Tests

- `TestKnowledgeDecay`: Confidence decreases each tick, item removed at 0
- `TestTellTransfer`: NPC A tells B, B's buffer contains the item
- `TestLieDetection`: NPC follows lie, finds nothing, trust decreases
- `TestKnownFoodSensor`: Ring0[11] reflects distance to known (not visible) food

### 2.8 Verification

```sh
go test ./pkg/sandbox/... -v -run "Knowledge|Tell|Lie"
go run ./cmd/sandbox --npcs 20 --ticks 10000 --seed 42 --verbose  # observe gossip
```

---

## Phase 3: Trust Matrix & Alliances (Go)

**Goal:** NPCs form trust relationships that influence cooperation, trade, and information acceptance.

### 3.1 Trust matrix

**File:** `pkg/sandbox/trust.go` (new)

```go
type TrustMatrix struct {
    Values [][]int8  // [A][B] = A's trust toward B, range -128..127
    Size   int
}

func (tm *TrustMatrix) Adjust(from, to int, delta int8) { ... }
func (tm *TrustMatrix) Get(from, to int) int8 { ... }
```

### 3.2 Trust modification events

| Event | Delta |
|-------|-------|
| B shares food with A | +10 |
| B gives accurate information to A | +5 |
| B attacks A | -30 |
| B's information was a lie (detected) | -20 |
| B trades fairly with A | +8 |
| Natural decay toward 0 each 256 ticks | +/-1 |

### 3.3 Trust-gated actions

- `ActionTell`: Target only accepts if `trust[target][self] > -50`
- `ActionTrade`: Requires `trust[A][B] > 0` for both parties
- `ActionTeach`: Charisma check modulated by trust

### 3.4 Faction computation

Every 256 ticks, compute factions via simple connected components:
- Two NPCs are "connected" if mutual trust > 20
- Connected components become factions
- Faction ID written to NPC struct, exposed in Ring0[21]

### 3.5 Ring0 slots

- `Ring0[17] = near_npc_trust`: Trust value toward nearest NPC (shifted to 0-255)
- `Ring0[22] = near_npc_faction`: Faction of nearest NPC

### 3.6 Tests

- `TestTrustIncrease`: Share food → trust increases
- `TestTrustDecreaseOnLie`: Lie detected → trust drops
- `TestFactionFormation`: High mutual trust → same faction
- `TestTradeRequiresTrust`: Zero-trust pair cannot trade

### 3.7 Verification

```sh
go test ./pkg/sandbox/... -v -run "Trust|Faction"
go run ./cmd/sandbox --npcs 20 --ticks 20000 --seed 42 --verbose  # observe factions
```

---

## Phase 4: Memetic Transmission (Go)

**Goal:** NPCs can copy bytecode fragments between brains. Ideas spread culturally.

### 4.1 Teach action handler

**File:** `pkg/sandbox/scheduler.go`

```go
case ActionTeach:
    target := findNPC(ring1Target)
    if target == nil || manhattan(npc, target) > 1 { break }
    // Charisma contest
    prob := float64(npc.Charisma) / float64(npc.Charisma + target.Charisma + 1)
    if rng.Float64() > prob { break }
    // Copy 4-byte fragment
    srcOff := (ring1Param % (len(npc.Genome) / 4)) * 4
    dstOff := rng.Intn(len(target.Genome) - 3)
    copy(target.Genome[dstOff:dstOff+4], npc.Genome[srcOff:srcOff+4])
```

### 4.2 Charisma evolution

Charisma is part of the NPC state but NOT in the genome. Instead, it correlates with fitness:
- `charisma = fitness / 10` (capped at 255)
- Successful NPCs naturally become more influential
- Alternative: charisma as a genome-encoded trait (first 1-2 bytes encode charisma). This makes charisma itself evolvable.

### 4.3 Cultural similarity metric

For visualization and analysis, compute Hamming distance between genomes of nearby NPCs. Track average intra-faction vs inter-faction similarity over time. Convergence = culture formation.

### 4.4 Seed genomes

**File:** `testdata/sandbox/evangelist.mpsil`

- Evangelist: high charisma threshold, seeks nearby NPCs, teaches aggressively

### 4.5 Tests

- `TestTeachCopiesFragment`: Source NPC teaches target, 4 bytes transferred
- `TestTeachCharismaGating`: Low-charisma source fails against high-charisma target
- `TestCulturalConvergence`: After 1000 ticks with teaching, intra-cluster Hamming distance decreases

### 4.6 Verification

```sh
go test ./pkg/sandbox/... -v -run "Teach|Cultural"
go run ./cmd/sandbox --npcs 32 --ticks 50000 --seed 42 --verbose  # observe meme spread
```

---

## Phase 5: Z80 Port — Trade & Inventory

**Goal:** Bring Phase 1 to Z80 assembly.

### 5.1 Extend NPC table entry

**File:** `z80/sandbox.asm`

Grow `NPC_SIZE` from 14 to 20 bytes. Add fields at offsets +14 through +19.

### 5.2 Extend Ring0 fill

**File:** `z80/sandbox.asm`

Add slots 14-16 to `fill_ring0`. Each slot is 2 bytes at `RING0_BUF + slot*2`.

### 5.3 Extend apply_actions

**File:** `z80/sandbox.asm`

Add trade handler after eat handler:
```z80
    CP 4 : JR Z, .act_trade
    ; ...
.act_trade:
    ; Find target NPC, check bilateral consent, swap items
```

### 5.4 Item tiles

Add `TileTool=3` etc. to `respawn_food` (make it `respawn_items`).

### 5.5 Cross-validation

**File:** `testdata/sandbox/crossval_test.go`

Add `TestTraderGenomeCrossValidation`: Same genome, same Ring0 inputs → same Ring1 outputs on Go and Z80 VMs.

### 5.6 Verification

```sh
sjasmplus z80/sandbox.asm --raw=z80/build/sandbox.bin
mzx --run z80/build/sandbox.bin@8000 --console-io --frames DI:HALT
go test ./testdata/sandbox/... -v -run "Trader"
```

---

## Phase 6: Z80 Port — Knowledge & Trust

**Goal:** Bring Phases 2-3 to Z80.

### 6.1 Knowledge bank

**File:** `z80/sandbox.asm`

Allocate knowledge bank at `$D000` (or banked $C000 page 3). 16 NPCs × 32 bytes = 512 bytes.

### 6.2 Trust matrix

Allocate at `$D200`. 16 × 16 = 256 bytes (signed). On 128K, use page 3.

### 6.3 Extended Ring0/Ring1

Extend `fill_ring0` with knowledge-derived slots. Extend `apply_actions` with tell/ask/lie handlers.

### 6.4 Confidence decay

Add to tick loop: iterate knowledge bank, decrement all confidence bytes, zero out items at confidence=0.

### 6.5 Lie detection

In `apply_actions` for movement: if NPC arrived at a `FOOD_AT` location from knowledge and tile is empty, decrease trust in informer.

### 6.6 Cross-validation

Test that knowledge transfer and trust updates produce identical results on Go and Z80.

---

## Phase 7: Z80 Port — Memetic Transmission

**Goal:** Bring Phase 4 to Z80.

### 7.1 Teach handler

**File:** `z80/sandbox.asm`

~40 bytes of Z80 code:
- Read source genome fragment (4 bytes at param*4)
- Pick random destination offset (LFSR mod target genome length)
- LDIR 4 bytes
- Charisma comparison via subtraction

### 7.2 Extended stats output

Print faction counts and cultural similarity metrics in `print_stats`.

### 7.3 Full cross-validation

All new genomes (trader, scout, liar, evangelist) cross-validated between Go and Z80.

---

## Phase 8: Visualization & Demo

**Goal:** Make it visible and demoable.

### 8.1 Go CLI enhancements

- `--verbose` output includes: trade events, knowledge transfers, lie detections, faction changes
- `--stats` outputs CSV for graphing: tick, alive, factions, avg_trust, trade_count, lie_count
- `--dump-knowledge` dumps all NPC knowledge buffers at end

### 8.2 ZX Spectrum visualization (optional)

If targeting a playable demo:
- 32×32 world rendered as 8×8 pixel tiles (256×256 pixels, fits in Spectrum screen)
- NPC colors by faction
- Status bar shows selected NPC's knowledge/trust
- Requires ROM mode (`--console` instead of `--console-io`)

### 8.3 Web visualization (optional)

Go HTTP server serving real-time simulation state as JSON. Browser renders with Canvas. Low priority but high demo value.

---

## Dependency Graph

```
Phase 1 (Trade/Go) ✓
    │
Phase 1a (MaxGenome+RNG) ✓
    │
Phase 1b (Modifiers+Crystal) ✓
    │
Phase 1c (Forge+Crafting) ✓
    │
Phase 1d (Pricing+Stress) ✓
    │
Phase 3 (MaxAge+Hazards+Memetics/Go) ✓
    │
Phase 4 (Scale to 10k/Go) ✓ ────────┐
    │                                 │
Phase 2 (Knowledge/Go)               Phase 5 (Trade/Z80)
    │                                 │
Phase 3b (Trust/Go) ────────────────Phase 6 (Knowledge+Trust/Z80)
    │                                 │
    │                               Phase 7 (Memetics/Z80)
    │                                 │
    └────────── Phase 8 (Viz/Demo) ───┘
```

Phases 1-1d, 3, and 4 are complete. Phase 2 (Knowledge) is next on the Go side. Phases 5-7 can start after their Go counterpart is stable. Phase 8 can start after Phase 2.

## Estimated Effort

| Phase | Scope | Files touched |
|-------|-------|---------------|
| 1. Trade (Go) | NPC struct + 2 Ring slots + trade handler + 2 genomes + tests | 4 modified, 2 new |
| 2. Knowledge (Go) | Knowledge buffer + tell/ask/lie + sensors + decay + 2 genomes + tests | 3 modified, 2 new |
| 3. Trust (Go) | Trust matrix + gating + factions + tests | 2 modified, 1 new |
| 4. Memetics (Go) | Teach handler + charisma + cultural metrics + tests | 2 modified, 1 new |
| 5. Trade (Z80) | NPC table + Ring0 + apply_actions + cross-val | 1 modified, 1 test |
| 6. Knowledge+Trust (Z80) | Knowledge bank + trust matrix + extended handlers | 1 modified, 1 test |
| 7. Memetics (Z80) | Teach handler + stats | 1 modified, 1 test |
| 8. Viz/Demo | CLI flags + optional screen render | 2 modified |
