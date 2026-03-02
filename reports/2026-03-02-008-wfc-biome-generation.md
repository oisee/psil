# WFC Biome Generation: Rivers Create Specialists, Not Just Traders

**Report 008** | 2026-03-02 | psil sandbox

---

## The Problem: Flat Worlds Breed Forager Monoculture

Report 006 showed growth/exchange crossover produces 35% more trades than classic crossover. But both converge to the same endpoint: a population of generalist foragers who happen to bump into each other and trade. The world is a uniform random soup of tiles. There's no reason for an NPC to become a mountain-dwelling crystal seeker or a village crafter because mountains and villages don't exist.

The fitness landscape is flat. Evolution finds one local optimum (forage + wander) and stays there.

## The Fix: Wave Function Collapse Terrain

We ported the WFC engine from the emergent-adventure project into the sandbox. 7 biome types, bitwise constraint propagation, anchor-seeded generation with reachability verification:

```
. = Clearing   T = Forest   ^ = Mountain
~ = Swamp      H = Village  = = River   # = Bridge
```

A generated 32x32 map (seed 42):

```
==TT####..HH^^TT^^^^TT####....TT
==TT####..HH^^TT^^^^TT####....TT
........HH^^^^^^^^HH..TT####HH##
........HH^^^^^^^^HH..TT####HH##
HH##TT..HH^^HHHH^^HHHH..HH..##..
HH##TT..HH^^HHHH^^HHHH..HH..##..
##......HHHH####TT####TT##TT##TT
##......HHHH####TT####TT##TT##TT
```

Each biome has distinct resource profiles:
- **Clearings**: 60% food rate, no items. Forager paradise.
- **Mountains**: 10% food, spawn crystals and weapons. Crystal-seekers thrive.
- **Villages**: 30% food, all item types, forges. Trading and crafting hubs.
- **Rivers**: Impassable barriers. Force NPCs through bridge chokepoints.
- **Swamps**: 5% food, poison hazards. Punishing to traverse.

## Experimental Setup

All runs use growth crossover, evolve-every-100, seed-matched biomes vs flat comparisons. Flat worlds use the original uniform random tile generation.

## Results

### 100k Ticks, 200 NPCs (4 seeds: 42, 99, 7, 123)

| Metric | Biomes (avg) | Flat (avg) | Delta |
|--------|-------------|-----------|-------|
| Crafted items | **19.3** | **4.0** | **+383%** |
| Crystal NPCs | **2.0** | **0.0** | biomes-only |
| Compass holders | **15.3** | **3.3** | **+364%** |
| Shield holders | **4.0** | **1.5** | **+167%** |
| Trades | 22,152 | 28,782 | -23% |
| Teaches | 9,101 | 10,724 | -15% |
| Gold | 158 | 522 | -70% |
| Alive | 62 | 66 | -6% |
| Avg stress | 69 | 66 | +5% |
| Items on map | 39 | 134 | -71% |
| Tool holders | **3.3** | **27.8** | -88% |

### 100k Ticks, 500 NPCs (seed 42)

| Metric | Biomes | Flat | Delta |
|--------|--------|------|-------|
| Crafted | **31** | **3** | **+933%** |
| Crystal NPCs | 1 | 0 | -- |
| Trades | 46,437 | 85,118 | -45% |
| Teaches | 29,391 | 38,448 | -24% |
| Item diversity | 5 types | 4 types | +1 |
| Tool holders | 11 | 74 | -85% |

### 1M Ticks, 200 NPCs (seed 42) — The Long Run

| Metric | Biomes | Flat | Delta |
|--------|--------|------|-------|
| Trades | 220,328 | 288,286 | -24% |
| Teaches | 95,085 | 108,672 | -13% |
| Crafted | **22** | **3** | **+633%** |
| Crystal NPCs | **3** | **0** | biomes-only |
| Compass holders | 18 | 3 | +500% |
| Shield holders | 4 | 0 | -- |
| Best fitness | 1,199 | 569 | **+111%** |
| Gold | 143 | 45 | +218% |

## Interpretation

### What biomes create: Material specialists

The single clearest signal across every run and every scale: **biome worlds produce 4-10x more crafted items**. Flat worlds are dominated by tool holders (25-75 per run) because tools are the most common random spawn. Biome worlds produce balanced distributions of compass, shield, treasure, and crystal — because each biome spawns its specialty.

The crafting pipeline actually works in biome worlds:
1. Mountains spawn crystals and weapons
2. NPCs pick them up and carry them
3. NPCs encounter forges concentrated in villages/mountains
4. Crafting happens: tool->compass, weapon->shield
5. Crafted items grant better modifiers (compass: +2 forage vs tool: +1)

In flat worlds, forges are randomly scattered and item distribution is tool-heavy. The pipeline stalls at step 1.

### What biomes suppress: Raw trade volume

Biome worlds consistently produce **23-45% fewer trades** than flat worlds. This scales with population density — the effect is -23% at 200 NPCs and -45% at 500 NPCs.

The cause is river barriers. Rivers are 2x2 tiles wide (due to WFC expansion) and cut the map into disconnected regions linked only by bridges. NPCs that end up on the "wrong" side of a river have fewer trading partners.

This isn't necessarily bad. The *quality* of trades should be higher when NPCs carry diverse items. A compass-for-treasure trade creates more value than a tool-for-tool swap. But the current system counts all bilateral trades equally.

### What biomes enable: The crystal economy

Crystal NPCs (those with permanent +50 gas modifiers) appear **only** in biome worlds. Mountains reliably spawn crystals; in flat worlds, the 1-in-20 crystal spawn probability is too low and too diffused to matter.

At 1M ticks with biomes: 3 crystal NPCs alive at end. These NPCs get 50 more gas for their brain execution — a significant cognitive advantage. Over evolutionary time, crystal-seekers should become a distinct specialist lineage.

### The 1M run: Best fitness doubles

The most interesting single number: **best fitness 1,199 (biomes) vs 569 (flat)** at 1M ticks. The biome world's best NPC lived 699 ticks, ate 37 food, and held a compass. The flat world's best lived 299 ticks, ate 11 food, and held a tool.

Biome terrain selects for NPCs that can navigate to and exploit specific zones. The rugged fitness landscape rewards specialists who find their niche. Flat terrain is a dice roll — everyone wanders and whoever gets lucky with food placement wins briefly.

### Item distribution tells the full story

**Flat 200 NPCs @ 100k (seed 42)**:
```
tool=32  weapon=5  treasure=6  compass=2  shield=0
```
Tool monoculture. 71% of item holders carry tools.

**Biomes 200 NPCs @ 100k (seed 42)**:
```
compass=14  treasure=7  tool=6  shield=5  weapon=0
```
Diverse economy. Compass is dominant (crafted from tools), shields present (crafted from weapons), treasures balanced. The crafting pipeline is alive.

## Performance

| Config | Wall time | NPC-ticks/sec |
|--------|-----------|---------------|
| 200 NPCs, 100k ticks | 68s | 294k |
| 500 NPCs, 100k ticks | ~280s | 179k |
| 200 NPCs, 1M ticks | ~11min | 303k |

The VM brain execution (bytecode interpretation with gas limits) is the bottleneck. ~300k NPC-ticks/second on a single core.

## What's Missing

1. **Trade quality metric**: Count doesn't capture value. A diverse-item trade should score higher than a tool-for-tool swap. Need a "trade diversity index" or scarcity-weighted trade value.

2. **Thinner rivers**: The 2x2 tile expansion makes rivers very wide. At 500 NPCs, this fragments the population too aggressively. Options: 1x expansion, more bridges, or river tiles that slow but don't block.

3. **Behavioral specialization metric**: We measure *material* specialization (item diversity, crafting) but not *behavioral* specialization. Need to classify NPCs by dominant action pattern (forager, trader, crafter, explorer) and measure entropy of the distribution.

4. **Biome-aware genomes**: Ring0Biome sensor is wired (slot 26) but seed genomes don't use it yet. Evolution must discover biome-conditional behavior from scratch. Pre-seeding one archetype with biome-branching bytecode would accelerate specialization.

5. **Longer runs with larger populations**: 1M ticks at 200 NPCs is informative but the population is small. 500+ NPCs at 1M ticks would reveal whether the crafting economy scales or saturates.

## Conclusion

WFC biome generation transforms the sandbox from a uniform soup into structured geography. The headline: **crafting increases 4-10x, item diversity replaces tool monoculture, and crystal specialists emerge exclusively in biome worlds**. Trade volume drops 23-45% due to river fragmentation — a design tradeoff that may need tuning, but the trades that do occur involve more diverse goods.

The 1M-tick run shows the effect compounds over evolutionary time. Best fitness doubles. The rugged fitness landscape is doing what it was designed to do: rewarding specialists over generalists. The forager monoculture problem from report 006 is broken.

Next steps: trade quality metrics, thinner rivers, and biome-aware seed genomes.
