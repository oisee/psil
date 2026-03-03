# Warriors Become Healers: Emergent Phase Transitions in Evolved Civilizations

**Report 011** | 2026-03-03 | psil sandbox

---

## The Discovery

Seed 999 is the warrior seed. At 100k ticks it had 568,282 attacks and 7,907 kills — an order of magnitude more violence than any other seed under identical mechanics. We doubled the run to 200k ticks expecting escalation.

Instead, the warriors **self-corrected**.

| Metric | 100k ticks | 200k ticks | Change |
|--------|-----------|-----------|--------|
| Attacks | 568,282 | 40,225 | **-93%** |
| Kills | 7,907 | 252 | **-97%** |
| Heals | 1,710 | **264,192** | **+15,450%** |
| Trades | 882 | **2,042,358** | **+231,500%** |
| Best fitness | 510,669 | 491,269 | -4% |
| GenomeAvg | 123 | 98 | -20% |

The civilization that started as the most violent in our 4-seed sample became the most cooperative. 264k heals vs 40k attacks. 2 million trades. The warmonger genome was replaced by healer-trader genomes through natural selection.

## Mechanism: Why Warriors Self-Destruct

The phase transition has a clear mechanistic explanation:

### 1. Warriors Kill Their Own Fitness

Attack costs 10 energy and kills NPCs. Dead NPCs can't trade, can't cooperate, can't contribute to the population. In a finite population (capped at ~100 after initial crash), every kill reduces the pool of potential trading partners and kin.

Warrior genomes score high short-term fitness (they steal items on kill, +50 bonus) but create a negative-sum environment. The gold on dead NPCs vanishes. The items they carried are partially looted but the economy shrinks.

### 2. Healers Are Cheap Warriors

The heal opcode (`0x95`) is one byte different from attack (`0x94`). A single mutation flips a warrior genome into a healer genome. The reverse is also true — but healers create positive-sum environments (healed NPCs live longer, trade more, produce more), so healer populations grow while warrior populations shrink.

### 3. The GA Selects for Cooperation

The fitness function rewards longevity, food eaten, and gold accumulated:

```
fitness = age + foodEaten*10 + health + gold*20 + craftCount*30 + teachCount*15 - stress/5
```

Healers live longer (mutual healing prevents death), accumulate more food (stable population = more farmers), and trade more (healthy neighbors = trade partners). By tick 100k, warrior genomes have been outcompeted by healer-trader genomes in the breeding pool.

### 4. The Kin Selection Ratchet

The `Ring0Similarity` sensor lets genomes check genetic similarity before acting. Once a "check similarity → heal if kin, attack if foreign" pattern evolves, it spreads rapidly through the population via crossover. As the population becomes more genetically homogeneous (due to GA selection), the similarity threshold triggers healing more often than attacking. This is a self-reinforcing feedback loop — more healing → more survivors → more similar genomes → more healing.

## The Timeline

The sparkline data tells the story:

```
attacks   [0→40156]  ▁▁▁▁▁▂▃▆▇▇▇▇▇▇▇▇▇▇...▇▇▇▇▇▇█
attacks/t [467→43]   ▁▁▁▂▂▅█▃▁▁▁▁▁▁▁▁▁▁...▁▁▁▁▁▁▁
heals     [0→263265] ▁▁▁▂▂▃▄▄▅▆▆▇▇▇▇▇▇▇...▇▇▇▇▇▇█
```

Attack rate peaked around tick 15k (`attacks/t` spike to 467/interval) and then collapsed to background noise (43/interval). Heals ramped up in parallel and continued rising through the entire 200k run. The crossover happened around tick 20-30k.

Trades show a similar inflection:

```
trades    [4→2009214] ▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▁▂▂▂▂▂▂▂...▇▇▇▇▇▇▇▇▇█
trades/t  [890→29216] ▁▁▁▁▁▁▁▅▇▆▆▆▅▅▄▆▆▅▄▆█▆▆▇...▆▆▆▅▇▇▆▅▆▆
```

Trade volume exploded after the warrior→healer transition, reaching 29k trades per sample interval. The cooperative economy bootstrapped on the wreckage of the warrior culture.

## City-States Emerged

The 200k snapshot shows clear settlement clustering:

| Cluster | NPCs | Location | Total Gold | Items |
|---------|------|----------|-----------|-------|
| 1 | **26** | ~(7,54) | 317,533 | 23 |
| 2 | **15** | ~(41,54) | 208,214 | 15 |
| 3 | 7 | ~(36,28) | 85,101 | 5 |
| 4 | 6 | ~(53,37) | 64,500 | 3 |
| 5 | 4 | ~(54,52) | 46,510 | 3 |

Two large settlements (26 and 15 NPCs) account for half the population. These are the healer-trader city-states — dense clusters where mutual healing sustains longevity and proximity enables constant trading.

The richest individual (NPC 7878 at position 37,27 with gold=22,025 and fitness=491,269) sits near cluster 3 — a small settlement. This suggests that moderate-density settlements optimize fitness better than the mega-clusters, possibly due to less competition for nearby food.

## Item Distribution Shift

| Item | 100k | 200k | Interpretation |
|------|------|------|----------------|
| Tool | **52** | 39 | Still dominant (terraform) |
| Compass | 3 | 8 | Recovery (harvest has value again) |
| Shield | 0 | 5 | **New — defense against residual attackers** |
| Weapon | 3 | 5 | Stable (some warriors persist) |
| Treasure | 5 | 5 | Stable |

The emergence of shields at 200k is telling. With 264k heals and 40k attacks still occurring, defense items have selective value. The shields weren't present at 100k when pure offense dominated.

## Cross-Seed Comparison: Cooperation Is the Attractor

| Seed | 100k Attacks | 100k Trades | Strategy |
|------|-------------|-------------|----------|
| 42 | 3,696 | 634,016 | Trader-farmer |
| 7 | 11,439 | 1,066,687 | Trader-farmer |
| 123 | 2,755 | 556 | Pure farmer |
| 999 | 568,282 | 882 | **Warrior** |

| Seed | 200k Attacks | 200k Trades | Strategy |
|------|-------------|-------------|----------|
| 999 | 40,225 | 2,042,358 | **Healer-trader** |

Seed 999 is the only seed that started violent. By 200k ticks, it converged to the same cooperative strategy as the other seeds — but through a different path. The other seeds never went through a warrior phase; they found cooperation directly. Seed 999 had to learn it the hard way.

This suggests **cooperation is an evolutionary attractor** in this system. Multiple initial conditions converge to the same qualitative outcome: high-trade, high-heal, low-attack societies. The warrior path is a temporary local optimum that self-destructs under selection pressure.

## Why This Matters

This phase transition was not designed. No game designer wrote "warriors should become healers at tick 100k." The transition emerged from:

1. A fitness function that rewards longevity and accumulation
2. Action opcodes that make attack and heal equidistant in mutation space
3. A kin selection sensor that rewards genetic similarity
4. Finite populations where killing reduces total system wealth
5. A GA that selects for fitness every 100 ticks

The warrior→healer transition is an emergent property of these five ingredients interacting. It recapitulates a pattern seen in biological and cultural evolution: aggressive strategies dominate early but are replaced by cooperative strategies as populations become more connected and interdependent.

## Reproduction

```bash
# Warrior seed at 100k (peak violence)
go run ./cmd/sandbox --npcs 200 --ticks 100000 --seed 999 --biomes --wfc-genome

# Same seed at 200k (cooperation emerges)
go run ./cmd/sandbox --npcs 200 --ticks 200000 --seed 999 --biomes --wfc-genome

# Compare with peaceful seeds
go run ./cmd/sandbox --npcs 200 --ticks 200000 --seed 42 --biomes --wfc-genome
go run ./cmd/sandbox --npcs 200 --ticks 200000 --seed 7 --biomes --wfc-genome
```
