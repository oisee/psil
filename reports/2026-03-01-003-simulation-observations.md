# Simulation Observations: Phase 1a-1d (Modifiers, Crystal, Crafting, Stress)

**Date**: 2026-03-01
**Config**: 20 NPCs, 32x32 world, 50k ticks, evolve every 100, gas=200
**Seeds**: 42, 7, 1337, 999
**Code version**: post Phase 1d (modifiers, crystal/gas, forge crafting, stress/pricing)
**See also:** [Temporal Dynamics](2026-03-01-005-temporal-dynamics.md) for charts at 200-5,000 NPC scale

---

## 1. Summary Table

| Metric          | Seed 42 | Seed 7  | Seed 1337 | Seed 999 |
|-----------------|---------|---------|-----------|----------|
| Alive at 50k    | 10      | 10      | 10        | 10       |
| Total trades    | 129     | 112     | 223       | 257      |
| Crystal NPCs    | 7       | 7       | 6         | 8        |
| Crafted items   | 0       | 0       | 0         | 0        |
| Avg stress      | 3       | 10      | 1         | 0        |
| Best fitness    | 107,699 | 106,669 | 107,080   | 108,460  |
| Best food eaten | 5,780   | 5,687   | 5,698     | 5,836    |
| Best gas bonus  | 150     | 100     | 100       | 150      |
| End gold        | 0       | 0       | 0         | 0        |
| Item holders    | 8       | 8       | 8         | 8        |
| Food spawned    | 49,992  | 49,965  | 49,700    | 49,856   |

---

## 2. Emergent Phenomena

### 2.1 The "Trading Burst" — All Commerce in the First 200 Ticks

The most striking pattern across all seeds: **trade activity is explosive at the start, then drops to zero permanently.**

```
Seed 42:   tick 0→100: 85 trades  | tick 100→200: 44 trades  | tick 200+: 0
Seed 7:    tick 0→100: 95 trades  | tick 100→200: 17 trades  | tick 200+: 0
Seed 1337: tick 0→100: 189 trades | tick 100→200: 34 trades  | tick 200+: 0
Seed 999:  tick 0→100: 193 trades | tick 100→200: 64 trades  | tick 200+: 0
```

**Why**: Seeded trader genomes carry items and have hard-coded trade-seeking behavior. At tick 200, the first GA evolution round culls the weakest 50%. Traders who spent energy moving toward NPCs instead of eating get outcompeted by simpler forager genomes. By tick 300, every survivor has converged to a forager-like strategy. Trading requires two NPCs with compatible genomes to be adjacent — a coordination problem that random mutation never rediscovers.

**Implication**: Trade is a *seeded* behavior, not an *emergent* one. The GA cannot reinvent it because ActionTrade (opcode sequence `push 4, r1! 1, r0@ 12, r1! 2, yield`) requires 10 bytes of precise bytecode. Random mutation has negligible probability of producing this.

### 2.2 The Forager Monoculture — Convergent Evolution

Every simulation converges on the same genome:

```
Seed 42:   8a0d 8c2f 21 8a0d 8c00 21 8c01 f1 000000  (forager + noise)
Seed 7:    8a0d 8c00 22 8c01 f1 0000000000000000       (forager + noise)
Seed 1337: 8a0d 8c00 21 8c01 f1                        (pure forager)
Seed 999:  8a0d 8c00 21 8c01 f1                        (pure forager)
```

The core is always `8a0d 8c00 21 8c01 f1` — the forager genome:
- `r0@ 13` — read food direction from Ring0
- `r1! 0` — write to Ring1 slot 0 (movement direction)
- `push 1` — action = eat
- `r1! 1` — write to Ring1 slot 1 (action)
- `yield` — return control

This is the minimum viable genome: 8 bytes that move toward food and eat. Seeds 42 and 7 have trailing junk bytes (mutations that don't affect behavior because they execute after `yield`). The GA converges to this within ~300 ticks across all seeds.

**Implication**: In the current fitness landscape, *survival is the only strategy*. Fitness = `age + food*10 + health + gold*5 - stress/5`, so age dominates over time. A forager that reliably eats will always outcompete a trader who sometimes starves while seeking trade partners.

### 2.3 Crystal Hoarding — Passive Resource Accumulation

60-80% of survivors accumulate crystal gas modifiers by tick 50k:

```
Seed 42:  7/10 crystal NPCs, best gas_bonus=150
Seed 7:   7/10 crystal NPCs, best gas_bonus=100
Seed 1337: 6/10 crystal NPCs, best gas_bonus=100
Seed 999: 8/10 crystal NPCs, best gas_bonus=150
```

At 100k ticks (seed 42), crystal_npcs rises to 8/10 and best gas_bonus reaches 200.

**How**: Crystal tiles spawn rarely (1-in-20 item spawns). When a forager walks over a crystal, it's auto-consumed, granting a permanent +50 gas modifier (with diminishing returns: 50, 25, 12, 6...). Since foragers move toward food, they incidentally walk across crystals. The accumulation is entirely passive — no genome explicitly seeks crystals.

**Implication**: Crystal gas modifiers are a side effect of movement, not strategic behavior. The gas bonus (extra VM execution cycles) provides no survival advantage for the dominant forager genome because foragers yield within 8 instructions. The extra gas is wasted.

### 2.4 The Population Bottleneck — 20 → 10 in 300 Ticks

Every seed shows identical dynamics:

```
tick    alive (seed 42 / 7 / 1337 / 999)
0       20 / 20 / 20 / 20
100     20 / 20 / 20 / 20
200     16 / 12 / 12 / 15
300     13 / 10 / 10 / 10
500     10 / 10 / 10 / 10
1000+   10 / 10 / 10 / 10  (forever)
```

Population halves by tick 300, then locks at exactly 10. The carrying capacity is determined by food respawn rate: `w.FoodRate = 0.5` spawns ~16 food/tick on a 32×32 grid with `MaxFood = 64`. Ten NPCs consuming ~1 food/tick reach equilibrium.

**Implication**: The world has a hard carrying capacity. NPCs that can't find food fast enough starve during the initial scramble. Once the population matches the food supply, all survivors persist indefinitely.

### 2.5 Zero Crafting — The Forge Discovery Problem

Across 200k total NPC-ticks (4 seeds × 50k ticks), **zero crafting events occurred**.

Crafting requires: (1) holding an item, (2) being on a forge tile, (3) executing `push 5, r1! 1, yield` (ActionCraft). The world has 1-2 forge tiles on a 32×32 grid (0.1-0.2% of tiles). The probability of a forager randomly landing on a forge is ~1/1024 per step. Even then, the forager genome doesn't contain the craft opcode.

**Implication**: Crafting is inaccessible to evolution. It requires: spatial knowledge (finding the forge), item management (holding the right item), and a novel action (ActionCraft=5). No stepping stone path leads there from the forager genome.

### 2.6 Zero Steady-State Stress

Avg stress across seeds: 0-10 (on a 0-100 scale).

Stress sources require either combat (no attack genomes survive) or starvation (abundant food prevents it). The stress system is dormant because the conditions that trigger it never arise.

```
40 NPCs, 50% traders: avg_stress=21  (crowding causes food scarcity → starvation stress)
32×32 world, default:  avg_stress=0-10 (food abundance → no stress triggers)
```

With 40 NPCs on the same map, stress rises to 21 — food competition creates occasional starvation events.

### 2.7 Gold Amnesia — Economic Memory Loss

Gold accumulates during the trading burst (tick 0-200), reaching 500-1200 total across all NPCs. Then at tick 200-300, the first GA evolution round replaces the weakest NPCs. The replacement NPCs start with gold=0. Surviving NPCs retain gold briefly, but further evolution rounds (every 100 ticks) gradually replace all original NPCs. By tick 500, total gold is 0 across all seeds.

**Root cause**: The GA's `Evolve()` resets `Gold = 0` on replacement NPCs. Since fitness doesn't weight gold heavily enough (`gold*5` vs `age*1` which accumulates to ~50k), gold-rich NPCs don't have a selection advantage.

### 2.8 Item Saturation — 8/10 Equilibrium

By tick 500-1000, exactly 8 out of 10 survivors hold items. This persists for the remainder of the simulation. Items persist through death (they drop to the tile) and through evolution (the GA doesn't clear items). The 2 NPCs without items are typically recently spawned replacements that haven't walked over an item yet.

---

## 3. Structural Insights

### What the GA Actually Selects For

The fitness function `age + food*10 + health + gold*5 - stress/5` is dominated by age over long runs:

| Component | Best NPC (seed 42, 50k) | Contribution |
|-----------|------------------------|--------------|
| age       | 49,799                 | 46.2%        |
| food×10   | 57,800                 | 53.7%        |
| health    | 100                    | 0.09%        |
| gold×5    | 0                      | 0%           |
| -stress/5 | 0                      | 0%           |

Food eaten and age are perfectly correlated (eat once per tick → food ≈ age × ~0.12). The GA is selecting for *consistent foraging*, which is exactly what the forager genome provides.

### Why Complex Behaviors Don't Emerge

The current setup has three barriers to behavioral complexity:

1. **No stepping stones**: The jump from forager (8 bytes) to trader (24 bytes) is too large for mutation. There's no intermediate behavior that's fitter than foraging but simpler than trading.

2. **No environmental pressure**: With `FoodRate=0.5` and `MaxFood=64` on a 32×32 grid, food is abundant. There's no reason to trade, craft, or compete — eating is sufficient.

3. **Gold/stress don't matter**: Gold contributes <1% of fitness. Stress is nearly always 0. The fitness landscape is flat except for the "eat food" dimension.

---

## 4. Suggested Design Changes

Based on these observations, to get richer emergent behavior:

| Change | Expected Effect |
|--------|----------------|
| **Scarcer food** (FoodRate 0.1, MaxFood 16) | Force competition, make trading/crafting worthwhile |
| **Preserve gold through evolution** | Allow economic memory to persist, make gold a durable advantage |
| **Seed 1-2 crafter genomes** | Bootstrap the crafting behavior like we bootstrapped trading |
| **Increase forge count** (4-6 per world) | Make forge discovery more likely for random walkers |
| **Weight gold more in fitness** (gold×50) | Make trading a viable evolutionary strategy |
| **Add food scarcity events** (periodic droughts) | Create pressure for diverse survival strategies |
| **Smaller genome step** for ActionCraft | E.g., auto-craft if standing on forge with right item, no action needed |

---

## 5. Raw Data: Epoch Snapshots (Seed 42)

```
tick     alive  food  items  trades  gold  holders  avg_fit  best_fit
0        20     23    0      0       0     5        105      121
100      20     21    6      85      510   6        330      1,506
200      16     9     8      129     387   7        454      2,186
300      13     22    8      129     0     7        489      951
500      10     13    7      129     0     4        846      1,341
1,000    10     20    8      129     0     8        1,742    2,401
5,000    10     15    8      129     0     8        8,528    10,981
10,000   10     13    8      129     0     8        17,072   22,111
25,000   10     20    8      129     0     8        42,294   53,981
50,000   10     14    7      129     0     8        75,503   107,699
```
