# ADR-004: Multi-Yield Coroutine Brain, Terraform, and Combat

**Date:** 2026-03-03
**Status:** Accepted
**Related:** [WFC Genome Generation](../../reports/2026-03-03-009-wfc-genome-generation.md), [ADR-001](001-bytecode-encoding.md)

## Context

Evolution converges to ~20-byte forager genomes regardless of max genome size (32-512). The brain-size sweep (Report 009) shows `genomeAvg` stays 20-25 bytes because there's no selective pressure for complexity. A simple `sense_food → move → eat → yield` loop is the global optimum.

Three structural problems:

1. **One action per tick.** Brain writes to Ring1, halts, scheduler executes. No multi-step plans within a tick.
2. **World is immutable.** NPCs consume but never produce. Food respawns from the system, not from NPC effort.
3. **No competition.** `ActionAttack` exists but deals no damage. NPCs have no reason to avoid each other.

## Decision

### 1. Yield-as-Syscall (coroutine brain)

**Current:** `yield` and `halt` both set `vm.Halted = true`. Brain runs once, scheduler acts once.

**New:** Add `vm.Yielded bool` field. `OpYield` sets `Yielded = true` (but NOT `Halted`). `OpHalt` sets `Halted = true` as before. `VM.Run()` stops on either flag.

Scheduler loop becomes:

```go
func (s *Scheduler) think(npc *NPC) {
    vm := s.vm
    // ... existing setup, load genome ...
    for {
        vm.Run()
        if vm.Halted || vm.Gas <= 0 {
            break
        }
        if vm.Yielded {
            s.act(npc)           // execute Ring1 commands
            s.sense(npc)         // refresh sensors with new world state
            // clear Ring1 for next action
            vm.MemWrite(64+Ring1Move, 0)
            vm.MemWrite(64+Ring1Action, 0)
            vm.MemWrite(64+Ring1Target, 0)
            vm.Yielded = false   // resume
        }
    }
    // Final act for anything written before halt
    s.act(npc)
}
```

Gas limits total computation across all yields. A brain with 200 gas might yield 3-4 times and execute 3-4 sequential actions per tick.

**Impact on genomes:** Multi-yield creates demand for larger, more structured genomes. Sense→decide→act→yield→sense→decide→act→yield sequences are longer than single-action loops. The WFC constraint mining will discover yield-as-separator patterns.

**Backward compatible:** Existing genomes that `halt` after one action work identically. Genomes that `yield` just get their action executed and the brain continues — previously this was equivalent to halt, now it's a resume point.

### 2. New Actions

Add to existing action constants:

```go
ActionHeal      = 7  // heal adjacent target NPC, costs energy
ActionAttack    = 2  // (existing, currently no-op) → now deals damage
ActionHarvest   = 8  // extract resource from current tile (tile stays, cooldown)
ActionTerraform = 9  // modify current tile (permanent change)
```

#### Attack (fix existing no-op)

- Deals `5 + weapon_bonus` HP damage to target NPC (must be adjacent).
- Costs 10 energy.
- If target dies, attacker takes their item.
- Weapon item: +5 damage. Shield item: -5 incoming damage.

#### Heal

- Heals target NPC `5 + tool_bonus` HP (must be adjacent).
- Costs 15 energy.
- Tool item: +3 heal bonus.

Attack and heal are symmetric — `push 2, r1! 1` vs `push 7, r1! 1`. One bit-flip in the push constant switches between them. Easy for mutation to discover both from one.

#### Harvest

- Extracts a resource from the tile the NPC is standing on.
- Tile stays but goes on cooldown (cannot be harvested again for N ticks).
- Result depends on biome:

| Biome | Primary yield | Rare yield (20%) | Cooldown |
|-------|--------------|-------------------|----------|
| Clearing | food | — | 5 ticks |
| Forest | food | tool | 15 ticks |
| River | food | — | 10 ticks |
| Mountain | weapon | crystal | 40 ticks |
| Village | tool | treasure | 30 ticks |
| Swamp | food | poison (damage!) | 20 ticks |

- Costs 10 energy.
- Compass item: halves cooldown (double harvest rate).

#### Terraform

- Modifies the tile the NPC is standing on.
- Costs 30 energy (reduced by tool: -10 with tool).

| Current tile | Result |
|-------------|--------|
| Empty | → Food tile (planting) |
| Forest | → Empty (clearing) |
| Swamp | → Empty (draining) |
| Empty + adjacent river + has tool | → Bridge |

### 3. Food Depletion

`World.FoodRate` decays over time:

```go
if w.Tick%100 == 0 {
    w.FoodRate *= 0.999  // halves every ~70k ticks
}
```

Initial FoodRate = 0.5, after 100k ticks ≈ 0.045. Forces transition from foraging to farming (terraform + harvest).

Minimum floor: `FoodRate >= 0.02` — some natural food always spawns, but not enough to sustain population.

### 4. New Sensors

```go
Ring0TileType  = 27  // tile type under NPC (for harvest/terraform decisions)
Ring0Similarity = 28  // genetic similarity to nearest NPC (0-100, hamming-based)
Ring0TileAhead = 29  // tile type in move direction (for navigation)
Ring0Cooldown  = 30  // ticks remaining on current tile's cooldown
```

`Ring0Similarity` enables kin selection to emerge: genomes that check similarity before attacking will preferentially spare relatives, creating proto-cooperation without hardcoding it.

### 5. Tile Cooldown Storage

Add per-tile cooldown byte. Current tile is `uint16` (4-bit type + 12-bit occupant). Cooldown stored in a parallel `[]byte` grid:

```go
type World struct {
    // ... existing ...
    Cooldowns []byte  // per-tile harvest cooldown (0 = available)
}
```

Decremented by 1 each tick (for non-zero tiles). Harvest fails if cooldown > 0.

### 6. New Archetypes

Three new handcrafted genomes for WFC constraint mining and population seeding:

**Farmer:**
```
r0@ food_count → push 5 → < → jnz plant
  r0@ food_dir → r1! 0 → push 1 → r1! 1 → yield    // forage
  halt
plant:
  push 9 → r1! 1 → yield                              // terraform (plant food)
  halt
```

**Fighter:**
```
r0@ near_dist → push 2 → < → jnz attack
  r0@ near_dir → r1! 0 → yield                        // move toward target
  halt
attack:
  push 2 → r1! 1 → r0@ near_id → r1! 2 → yield       // attack nearest
  r0@ food_dir → r1! 0 → push 1 → r1! 1 → yield      // then forage
  halt
```

**Healer:**
```
r0@ near_dist → push 2 → < → jnz check_health
  r0@ food_dir → r1! 0 → push 1 → r1! 1 → yield      // forage
  halt
check_health:
  r0@ similarity → push 50 → > → jnz heal             // heal kin only
  r0@ food_dir → r1! 0 → yield
  halt
heal:
  push 7 → r1! 1 → r0@ near_id → r1! 2 → yield       // heal
  halt
```

All three use multi-yield (2-3 actions per tick), demonstrating the pattern for WFC mining.

### 7. Action Opcodes

Instead of requiring genomes to manually write Ring1 slots and yield, actions are 2-byte VM opcodes:

```go
OpActMove      = 0x93 // [arg] 1-4=dir, 5=toward food, 6=toward NPC, 7=toward item
OpActAttack    = 0x94 // [0] attack nearest adjacent
OpActHeal      = 0x95 // [0] heal nearest adjacent
OpActEat       = 0x96 // [0] eat nearby food
OpActHarvest   = 0x97 // [0] harvest current tile
OpActTerraform = 0x98 // [0] terraform current tile
OpActShare     = 0x99 // [0] share energy
OpActTrade     = 0x9A // [0] trade with nearest
OpActCraft     = 0x9B // [0] craft held item
```

Each opcode writes the appropriate Ring1 values (reading targets from Ring0 sensors) and sets `vm.Yielded = true`. The scheduler's existing coroutine loop picks up and executes the action.

**Key benefit:** Attack is 2 bytes (`{0x94, 0x00}`) instead of ~9 bytes of Ring1 writes. A single mutation can discover any action. Backward compatible — old genomes using Ring1 writes still work.

## Implementation Order

1. **VM: Yielded flag** — `Yielded bool`, `OpYield` sets `Yielded` not `Halted`. ✅
2. **Scheduler: coroutine loop** — yield loop in `think()`. ✅
3. **Tile cooldowns** — `Cooldowns []byte` in World, decrement per tick. ✅
4. **New sensors** — TileType, Similarity, TileAhead, Cooldown. ✅
5. **ActionAttack fix** — damage + item steal. ✅
6. **Action opcodes** — 9 new 2-byte opcodes (0x93-0x9B) in VM. ✅
7. **ActionHeal** — heal adjacent target. ✅
8. **ActionHarvest** — biome-based extraction with cooldown. ✅
9. **ActionTerraform** — tile modification. ✅
10. **WFC update** — action opcodes in token classifier + renderer. ✅
11. **Tests** — 8 action opcode tests, all 84 tests pass. ✅
12. **FoodRate decay** — `FoodRate *= 0.999` every 100 ticks, floor at 0.02. ✅
13. **New archetypes** — farmer, fighter, healer using action opcodes. ✅

## Consequences

**Positive:**
- Multi-yield creates demand for longer genomes (hypothesis: optimal size shifts from 20 to 60+ bytes)
- Terraform + depletion creates producer/consumer dynamic (farmers emerge)
- Attack + heal + similarity creates cooperation/competition axis (kin groups emerge)
- Existing items gain combat/economic meaning (weapon=attack, shield=defense, tool=terraform, compass=harvest)

**Negative:**
- Multi-yield increases computation per NPC per tick (~3x gas consumption on average)
- Combat creates population volatility (extinction risk if too aggressive)
- Terraform can be exploited (plant food → eat → plant → never need to move)
- Need to tune energy costs carefully to prevent degenerate strategies

**Risks:**
- Attack may dominate if damage is too cheap — mitigate with energy cost + shield
- Farming may be too hard to discover (delayed reward) — seed with farmer archetype
- Multi-yield may not change genome size if gas is too low — ensure gas >= 200

## Validation

```bash
# Brain size sweep with new mechanics (expect genome size increase)
go test ./pkg/sandbox/... -v -run TestEvalBrainSizeSweep

# Multi-yield behavior test
go test ./pkg/sandbox/... -v -run TestMultiYield

# Full civilization run
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes --wfc-genome

# Compare with/without new mechanics (A/B)
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes --wfc-genome --ab
```

**Success criteria:**
- Average genome size > 40 bytes at 100k ticks (currently 20)
- At least 2 distinct behavioral clusters (forager, farmer, fighter)
- Food production via terraform > 10% of total food consumed
- Kin selection observable: attack rate toward similar genomes < dissimilar
