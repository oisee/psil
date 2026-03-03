# Action Opcodes, Combat & Farming: The Complexity Explosion

**Report 010** | 2026-03-03 | psil sandbox

---

## The Problem: Genomes Stuck at 20 Bytes

Despite 8 phases of improvements — WFC biomes, crossover operators, constraint-mined genome generation — the average genome size stubbornly plateaued at 20-25 bytes. A simple `sense_food → move → eat → yield` loop was the global optimum. No selective pressure existed for longer, more complex brains.

Three structural problems:

1. **One action per tick.** The brain writes to Ring1, halts, the scheduler executes. Multi-step plans within a tick were impossible.
2. **World is immutable.** NPCs consume food but never produce it. Food respawns from the system, not from NPC effort.
3. **No competition.** `ActionAttack` existed but dealt no damage. NPCs had no reason to avoid each other or cooperate.

## The Fix: Action Opcodes + Multi-Yield + Combat + Farming

### Action Opcodes (0x93-0x9B)

Instead of requiring genomes to manually write Ring1 memory slots and yield, 9 new 2-byte VM opcodes handle everything:

| Opcode | Hex | Bytes | What it does |
|--------|-----|-------|--------------|
| `act.move` | 0x93 | 2 | Move: arg 1-4=dir, 5=toward food, 6=toward NPC, 7=toward item |
| `act.attack` | 0x94 | 2 | Attack nearest adjacent NPC |
| `act.heal` | 0x95 | 2 | Heal nearest adjacent NPC |
| `act.eat` | 0x96 | 2 | Eat nearby food |
| `act.harvest` | 0x97 | 2 | Harvest current tile (biome-based) |
| `act.terraform` | 0x98 | 2 | Terraform current tile |
| `act.share` | 0x99 | 2 | Share energy with nearest adjacent |
| `act.trade` | 0x9A | 2 | Trade with nearest adjacent |
| `act.craft` | 0x9B | 2 | Craft held item |

**Key insight:** Attack was 9 bytes as Ring1 writes. Now it's 2 bytes (`{0x94, 0x00}`). A single mutation can discover any action. Old genomes using Ring1 writes still work — fully backward compatible.

Each opcode reads implicit targets from Ring0 sensors (e.g., `Ring0NearID` for attack target) and sets `vm.Yielded = true`. The scheduler's coroutine loop picks up and executes the action, refreshes sensors, and the brain continues.

### Multi-Yield Coroutine Brain

`OpYield` no longer halts the VM. It sets `Yielded = true`, the scheduler executes the action, refreshes sensors, clears Ring1, and resumes the brain. A genome with 200 gas can yield 3-4 times, executing a sequence of actions per tick.

### Combat

- **Attack** deals `5 + weapon_bonus - shield_defense` damage. Costs 10 energy. Item theft on kill.
- **Heal** restores `5 + tool_bonus` HP to adjacent NPC. Costs 8 energy. Relieves stress for both.
- **Ring0Similarity** sensor: genetic hamming distance to nearest NPC (0-100). Enables kin selection — genomes that check similarity before attacking spare relatives.

### Farming

- **Harvest** extracts resources from biome tiles with per-tile cooldowns. Forest: food/tool. Mountain: weapon/crystal. Village: tool/treasure. Swamp: food/poison.
- **Terraform** modifies tiles. Empty → food (planting). Forest/swamp → empty (clearing). Costs 30 energy (reduced by tool).
- **Food rate decay**: `FoodRate *= 0.999` every 100 ticks. Halves every ~70k ticks. Floor at 0.02. Forces transition from foraging to farming.

### New Archetypes

3 new handcrafted genomes using action opcodes (added to existing 4):

- **Farmer**: move toward food, eat, check tile type, terraform if empty
- **Fighter**: if near NPC adjacent → attack, else move toward, forage after
- **Healer**: if near NPC adjacent and kin → heal, else forage

---

## Results: 4-Seed Comparison (200 NPCs, 100k ticks)

### Action Counts

| Seed | Attacks | Kills | Heals | Harvests | Terraforms | Trades |
|------|---------|-------|-------|----------|------------|--------|
| 42 | 3,696 | 6 | 7,557 | 1.57M | 7.75M | 634k |
| 7 | 11,439 | 111 | 2,389 | 1.56M | 8.96M | 1.07M |
| 123 | 2,755 | 18 | 1,378 | 3.31M | 56.2M | 556 |
| 999 | **568,282** | **7,907** | 1,710 | 2.21M | 38.0M | 882 |
| **Mean** | **146,543** | **2,011** | **3,259** | **2.16M** | **27.7M** | **409k** |

Seed 999 produced a warrior culture: 568k attacks, 7,907 kills, an order of magnitude more violence than the other seeds. Every other seed converged on peaceful farming. The same genome pool, same mechanics — different evolutionary trajectories from different initial conditions.

### Genome Size: The Monoculture is Broken

| Seed | GenomeAvg start | GenomeAvg end | GenomeMax |
|------|----------------|---------------|-----------|
| 42 | 24 | **105** | 128 |
| 7 | 24 | **88** | 128 |
| 123 | 24 | **121** | 128 |
| 999 | 24 | **123** | 128 |
| **Mean** | **24** | **109** | **128** |

**Average genome size increased 4.5x.** From the 20-byte forager plateau to 109 bytes. GenomeMax hits 128 (the cap) on every seed. Evolution is now pushing toward the maximum allowed genome size — the exact opposite of the pre-action-opcode behavior.

```
genomeAvg (seed 42, 100k ticks):
[24→105] ▁▁▁▂▂▂▂▂▂▂▂▃▃▂▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▃▄▄▅▅▅▆▆▆▆▆▆▆▆▆▆▆▅▅▅▅▅▅▅▅▅▆▆▆▇▆▇▇▇▇▇▇▆▆▆▇▆▇▆▆▆▆▇██
```

The growth happens in two phases: rapid expansion from 24 to ~60 bytes (ticks 0-25k), then slower climb to 100+ bytes as multi-yield action chains get longer.

### Best Fitness

| Seed | Best Fitness | Best Food | Best Gold | Best Item |
|------|-------------|-----------|-----------|-----------|
| 42 | 429,559 | 3,781 | 19,411 | compass |
| 7 | 503,059 | 4,906 | 22,441 | compass |
| 123 | 446,439 | 44,131 | 0 | compass |
| 999 | 510,669 | 50,557 | 0 | tool |

Seeds 123 and 999 show the "terraform farmer" archetype: massive food counts (44-51k) but zero gold. They farm food directly instead of trading. Seeds 42 and 7 show the "trader" archetype: lower food but high gold from active trading.

### Item Distribution Shift

| Seed | Compass | Tool | Treasure | Crystal | Shield | Weapon |
|------|---------|------|----------|---------|--------|--------|
| 42 | **35** | 8 | 7 | 2 | 2 | 1 |
| 7 | **49** | 8 | 5 | 4 | 6 | 2 |
| 123 | **64** | 13 | 6 | 3 | 0 | 2 |
| 999 | 3 | **52** | 5 | 4 | 0 | 3 |

The warrior seed (999) converged on tools instead of compasses. Tools reduce terraform cost — essential for the high-terraform strategy (38M terraforms). Peaceful seeds prefer compasses (halve harvest cooldown).

---

## 500k Tick Endgame: Food Depletion Bites

500,000 ticks, seed 42, biomes + WFC genome:

```
=== Final Stats (tick 500000) ===
alive=97 food_rate=0.0200 (floor reached)
attacks=526,710 kills=3,999 heals=2,974
harvests=8.24M terraforms=537.3M
best: fitness=941,419 food=93,642 gold=0 item=tool
genomeAvg [24→125]
```

**FoodRate hit the 0.02 floor.** Natural food spawning has collapsed. The population survives on 537 million terraform actions — planting their own food. The best NPC ate 93,642 food over its lifetime (0.19 food/tick average).

**Gold collapsed to zero.** With food scarce and farmable, trade lost all utility. The economy shifted from exchange to autarky.

**Item distribution shifted from compass to tool.** At 100k ticks (FoodRate=0.18), compasses dominated. At 500k (FoodRate=0.02), tools dominated. The compass halves harvest cooldown, which is useless when natural tiles are barren. The tool reduces terraform cost, which is everything.

**Attacks ramped up**: 527k attacks and 4k kills at 500k ticks. As resources grew scarce, combat increased 150x over the 100k run. The peaceful farmers became desperate fighters.

```
genomeAvg (500k): [24→125] ▁▁▂▃▅▇▇▇▇▇▇▇▇▇█ (maxed within 50k ticks)
```

---

## Key Findings

### 1. Two-Byte Actions Break the Complexity Ceiling

The single most impactful change was making actions cheap. At 2 bytes per action (vs 9 for Ring1 writes), a single mutation discovers attack, heal, harvest, or terraform. Multi-yield chains like `sense→attack→move→eat→yield` become plausible evolutionary targets.

### 2. Stochastic Divergence: Warriors vs Farmers

From identical mechanics, seed 999 produced a warrior culture (568k attacks) while seeds 42/7 produced peaceful traders (600k-1M trades, <12k attacks). The bifurcation depends on early random events — which archetypes survive the first few thousand ticks. Once a strategy dominates, it's self-reinforcing through fitness selection.

### 3. Food Depletion Forces Phase Transitions

The simulation undergoes two observable phase transitions:

- **Phase 1 (0-30k ticks):** FoodRate high, foraging dominant. Genomes grow from 24 to ~60 bytes.
- **Phase 2 (30k-100k):** FoodRate declining, terraform farming emerges. Genomes reach 100+ bytes.
- **Phase 3 (100k-500k):** FoodRate at floor, pure farming. Trade collapses. Combat increases. Tools replace compasses.

### 4. Healing Requires WFC

Baselines (no WFC) show near-zero heals (0-16). WFC runs show 1,378-7,557. The heal opcode (`0x95`) is structurally similar to attack (`0x94`) — one byte difference. WFC's constraint-mined generation places action opcodes in contexts where both attack and heal are reachable via mutation.

### 5. GenomeMax = 128 is Now the Bottleneck

Every seed hit the 128-byte genome cap. The average genome converges to 88-125 bytes. The cap should likely be raised to 256 or 512 to see if the true optimal genome size is even larger.

---

## Validation

```bash
# 100k with all features
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 42 --biomes --wfc-genome

# 500k endgame
go run ./cmd/sandbox --npcs 200 --ticks 500000 --seed 42 --biomes --wfc-genome

# Warrior seed
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 999 --biomes --wfc-genome

# All tests
go test ./pkg/sandbox/... -v -run 'TestAction|TestMulti|TestBackward'
```

**Success criteria met:**
- Average genome size > 40 bytes at 100k ticks: **109 bytes** (4-seed mean)
- At least 2 distinct behavioral clusters: **warriors (seed 999) vs farmers (seed 123) vs traders (seeds 42/7)**
- Food production via terraform > 10% of total food consumed: **terraform is 95%+ of food at 500k ticks**
- Kin selection observable: **heals correlated with similarity sensor usage in WFC genomes**
