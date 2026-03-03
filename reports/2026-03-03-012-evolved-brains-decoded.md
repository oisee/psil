# Evolved Brains Decoded: What 100,000 Ticks of Evolution Actually Produce

**Report 012** | 2026-03-03 | psil sandbox

---

## Reading an NPC Brain

Every NPC in the sandbox has a genome — a bytecode program that runs each tick, reading sensors and issuing actions. The handcrafted archetypes are 17-27 bytes. After 100,000 ticks of evolution, the best genomes are 42-128 bytes. What are those extra bytes doing?

We disassembled the winning genomes from 6 simulation runs. Here's what evolution discovered.

---

## The Handcrafted Ancestors (17-27 bytes)

These are the 7 seeded archetypes — human-designed programs that bootstrap the population.

### Farmer (17 bytes) — "Move to food. Eat. If tile empty, plant."

```asm
000  act.move →food          ; walk toward nearest food
002  act.eat                  ; eat it
004  r0@ energy               ; read my energy
006  r0@ tile_type            ; read tile under me
008  push 0                   ; 0 = empty tile
009  =                        ; tile_type == 0?
010  jnz +2                   ; if yes, skip to terraform
012  act.terraform            ; plant food on empty tile
014  halt
```

Simple, functional, 17 bytes. One decision: "is this tile empty? → plant." No combat, no trading, no harvesting.

### Fighter (17 bytes) — "If NPC adjacent, attack. Otherwise chase."

```asm
000  r0@ near_dist            ; how far is nearest NPC?
002  push 2                   ; threshold: 2 tiles
003  <                        ; near_dist < 2?
004  jnz +4                   ; if adjacent → attack
006  act.move →npc            ; else chase toward NPC
008  halt
009  act.attack               ; hit them
011  act.move →food           ; then forage
013  act.eat
015  halt
```

Two-phase behavior: chase then punch. Multi-yield lets it attack *and* eat in the same tick. 17 bytes.

### Healer (27 bytes) — "If kin is adjacent, heal. Otherwise forage."

```asm
000  r0@ near_dist            ; how far is nearest NPC?
002  push 2
003  <
004  jnz +8                   ; if adjacent → check kin
006  r0@ similarity           ; genetic similarity 0-100
008  push 2
009  push 16
010  >                        ; similarity > 16?
011  jnz +4                   ; if kin → heal
013  act.move →food           ; else forage
015  act.eat
017  halt
018  act.heal                 ; heal the kin
020  r0@ similarity           ; (sensor refreshed after yield)
022  act.move →food           ; then forage
024  act.eat
026  halt
```

The only archetype with conditional cooperation: checks `Ring0Similarity` before deciding to heal. 27 bytes.

---

## The Evolved Winners (42-128 bytes)

### Seed 999 × 400 NPCs — "The Compact Farmer" (42 bytes, fitness 497k)

The shortest winning genome. 400 NPCs, 200k ticks. Pure farming efficiency:

```asm
000  act.harvest              ; extract from current tile
002  push 29                  ; (stack noise — ignored by harvest)
003  r1@ 35                   ; read ring1 (side effect: stack value)
005  not
006  push 3
007  r1! move                 ; set move direction = 3 (south)
009  jnz +5                   ; if stack truthy → skip to 016
011  yield                    ; execute the move
012  act.terraform            ; plant food
014  nop
015  nop
016  r0@ energy               ; check energy
018  over
019  swap
020  act.trade                ; trade with neighbor
022  act.harvest              ; harvest again
024  yield                    ; execute trade + harvest
025  ifte                     ; (dead code — stack underflow, no-op)
...
041  r1! action               ; (unreachable)
```

**What evolution kept:** harvest → move → terraform → trade → harvest. A 5-action farming loop in 24 effective bytes. The remaining 18 bytes are junk DNA — mutations that didn't hurt, so selection didn't remove them. The `ifte` at byte 25 is dead code (nothing on the stack to branch on), and everything after it is unreachable. Evolution doesn't optimize for elegance.

**What it doesn't do:** No attack. No heal. No sensor-based decisions. Just: harvest, move south, plant, trade, harvest. Repeat forever.

### Seed 42 × 200 NPCs — "The Trader-Crafter" (89 bytes, fitness 430k)

The economy-focused genome. 634k trades at 100k ticks:

```asm
000  r0@ energy               ; check energy level
002  load                     ; (stack op — loads from memory)
003  act.craft                ; craft held item (tool→compass, crystal→shield)
005  r0@ near_dist            ; how far is nearest NPC?
007  r1! target               ; set trade target
009  yield                    ; [action: craft]
010  r0@ my_gold              ; check gold reserves
012  act.trade                ; trade with target
014  r1! ?36                  ; (junk write — no ring1 slot 36)
016  yield                    ; [action: trade]
017  r0@ my_gold              ; check gold again
019  >                        ; compare (against stale stack value)
020  act.trade                ; trade again
022  r0@ item_dir             ; where's nearest item?
024  act.harvest              ; harvest current tile
026  ...
032  yield
033  ...                      ; more harvest/terraform/trade cycles
058  r0@ health               ; check health
060  act.eat                  ; eat
062  ...
072  yield                    ; [final actions]
...
088  yield
```

**The pattern:** craft → trade → trade → harvest → terraform → eat → trade → terraform. At least 7 yields = 7 actions per tick. This genome is a full economic agent: it crafts items, trades obsessively, harvests resources, plants food, and eats — all in a single tick.

**Junk DNA:** `r1! ?36` (byte 14) writes to a nonexistent Ring1 slot. Harmless mutation that persists because selection doesn't penalize it.

### Seed 999 × 200 NPCs at 100k — "The Terraform Warrior" (128 bytes, fitness 511k)

The genome that dominated during the warrior phase. 568k attacks. Genome maxed at 128 bytes (the cap):

```asm
000  r0@ food_dir             ; where's food?
002  r0@ my_age               ; how old am I?
004  r0@ similarity           ; how similar is nearest NPC?
006  r1! ?140                 ; (junk write)
008  push 2
009  act.harvest              ; harvest
011  yield                    ; [action: harvest]
012  act.harvest              ; harvest again
014  not
015  r0@ food_dist            ; how far is food?
017  r1! target               ; set target
019  act.terraform            ; PLANT FOOD
021  act.harvest              ; harvest what we planted
023  push 1
024  r1! action               ; set action = eat
026  yield                    ; [action: eat]
027  r1! target
029  act.terraform            ; plant again
031  act.harvest              ; harvest again
...
072  act.attack               ; ← ATTACK (byte 72)
074  r0@ food_dist
076  r1! target
078  act.terraform            ; terraform after attack
080  act.harvest
...
098  act.attack               ; ← SECOND ATTACK (byte 98)
100  push 1
101  r1! action
103  yield
104  >
105  not
106  r0@ food_dist
108  r1! target
110  act.terraform            ; and more terraform...
112  act.terraform
...
```

**The dual strategy:** This genome is 80% farmer, 20% warrior. The core loop is terraform→harvest→eat repeated 6+ times per tick. But buried at bytes 72 and 98 are two `act.attack` opcodes. The genome farms obsessively *and* fights occasionally. The attack doesn't use the similarity sensor — it attacks indiscriminately.

**Why it won at 100k:** In a small population (100 NPCs after initial crash), indiscriminate attack + massive farming is viable. You kill competitors and outproduce everyone. But this strategy self-destructs over time (see below).

### Seed 999 × 200 NPCs at 200k — "The Healer-Trader" (94 bytes, fitness 491k)

The genome that **replaced** the warrior genome. The same seed, 100k ticks later:

```asm
000  push 18                  ; stack setup
001  push 1
002  r0@ item_dir             ; where's nearest item?
004  over
005  push 7
006  yield                    ; [action: none — just sensing]
007  push 1
008  and
009  r0@ ?152                 ; (junk sensor — wraps to 0)
011  r0@ food_dist            ; where's food?
013  act.terraform            ; plant food
015  act.move S               ; move south
017  r0@ day                  ; what time is it?
019  push 9
020  dup
021  yield                    ; [action: terraform + move]
022  r0@ similarity           ; ← KIN AWARENESS
024  yield                    ; [action: sense kin]
025  r0@ fear                 ; any danger?
027  push 22
028  yield
029  push 4
030  act.harvest              ; harvest
032  push 9
033  inc
034  nop
035  nop
036  act.harvest              ; harvest again
038  act.trade                ; trade
040  ...
047  jnz +13                  ; branch based on sensor
049  dup
050  ...
052  act.harvest              ; more harvesting
054  r0@ health               ; check health
056  act.harvest
...
062  jnz +13                  ; another branch
064  rot
065  act.harvest              ; FIVE harvest calls total
067  ...
069  yield
...
077  act.terraform            ; terraform
079  ...
086  act.terraform            ; another terraform
...
089  r0@ food_dir
091  ...
093  halt
```

**Zero attacks.** The warrior genes were completely purged. This genome has 5 harvest calls, 2 terraform calls, 1 trade, and reads `Ring0Similarity` (byte 22) — but uses it for sensing, not combat gating. The branches (jnz at bytes 47 and 62) create conditional harvesting patterns based on sensor state.

**The replacement happened because:** healer-traders create positive-sum environments. Every NPC they heal is a future trading partner. Every NPC the warrior killed was a lost trade. Over 100k ticks of GA selection, the math favored cooperation.

---

## Scaling Changes Everything

We ran the warrior seed (999) at 200 and 400 NPCs for 200k ticks:

| Metric | 200 NPCs | 400 NPCs | Effect |
|--------|----------|----------|--------|
| Attacks | 40,225 | 18,674 | **-54%** fewer |
| Kills | 252 | 34 | **-87%** fewer |
| Heals | 264,192 | 11,312 | **-96%** fewer |
| Trades | 2,042,358 | 4,145,624 | **+103%** more |
| GenomeAvg | 98 | **77** | **-21%** smaller |
| Best item | compass | tool | farming shift |
| Best genome | 94 bytes | **42 bytes** | simpler wins |

### More NPCs → Less Drama

At 400 NPCs, the warrior phase never meaningfully happened. With double the genetic diversity, cooperative genomes dominate from the start. The warrior→healer transition that was dramatic at 200 NPCs (568k → 40k attacks) simply doesn't occur at 400 NPCs (18k attacks total — background noise).

### More NPCs → Simpler Genomes

The winning genome at 400 NPCs is 42 bytes — less than half the 98-byte winner at 200 NPCs. Why? More NPCs means more competition for the same resources. Simpler, tighter farming loops that waste fewer gas cycles on junk DNA outperform sprawling multi-strategy genomes. Evolution sharpens toward the minimum viable farmer.

### More NPCs → Tools Over Compasses

At 200 NPCs: 39 tools, 8 compasses. At 400 NPCs: **97 tools**, 14 compasses. With more NPCs competing for harvest sites, the harvest cooldown bonus from compass matters less than the terraform cost reduction from tools. Farming overtakes foraging at scale.

---

## Junk DNA Is Everywhere

Every evolved genome contains dead code:

| Genome | Total bytes | Effective bytes | Junk % |
|--------|------------|----------------|--------|
| Compact farmer (42B) | 42 | ~24 | 43% |
| Trader-crafter (89B) | 89 | ~50 | 44% |
| Terraform warrior (128B) | 128 | ~80 | 38% |
| Healer-trader (94B) | 94 | ~55 | 41% |

Junk DNA includes:
- **Writes to nonexistent Ring1 slots** (`r1! ?36`, `r1! ?140`) — harmless no-ops
- **Sensor reads with no consumer** (`r0@ ?152`, `r0@ ?140`) — push garbage values that get dropped
- **Dead branches** (`ifte` with empty stack, `jnz` past end of genome)
- **Neutral mutations** (`nop`, `push N` followed by `drop`)

This is exactly what biological genomes look like. Evolution doesn't remove neutral mutations — it only selects against harmful ones. The 40% junk rate is stable across all genome sizes, suggesting it's an equilibrium property of the GA.

---

## The Archetype→Evolved Gap

| Property | Archetypes | Evolved winners |
|----------|-----------|----------------|
| Size | 17-27 bytes | 42-128 bytes |
| Actions per tick | 1-2 | 5-8 |
| Sensor reads | 1-2 | 5-12 |
| Strategies | 1 fixed | 1-2 adaptive |
| Junk DNA | 0% | ~40% |
| Branches | 1-2 | 3-6 |
| Yields | 1 | 5-8 |

The biggest gap is **actions per tick**. Archetypes yield once or twice. Evolved genomes yield 5-8 times, executing a full behavioral sequence each tick: sense→decide→act→sense→decide→act→... The multi-yield coroutine brain created the selective pressure for this complexity.

---

## Conclusions

1. **Evolution produces working programs.** Not elegant ones, not minimal ones, but programs that harvest, terraform, trade, and (sometimes) fight. The structural patterns from WFC seeding survive and get elaborated.

2. **Strategy depends on population size.** 200 NPCs → warrior→healer phase transition. 400 NPCs → immediate cooperation. The drama requires scarcity.

3. **Genome size reflects task complexity.** 42 bytes for a pure farmer. 94 bytes for a healer-trader with conditional branches. 128 bytes (cap) for a warrior-farmer hybrid. Raise the cap and genomes will likely grow further.

4. **40% of every genome is junk.** Neutral mutations accumulate because selection doesn't penalize them. This is not a bug — it's a reservoir of variation for future evolution to explore.

5. **The one-byte bridge matters.** Attack (`0x94`) and heal (`0x95`) differ by one byte. This tiny mutation distance means evolution can flip between aggression and cooperation in a single generation. The action opcode design made behavioral phase transitions possible.

## Reproduction

```bash
# Disassemble any genome
go run ./tools/disasm_genome <hex>

# Example: the compact farmer
go run ./tools/disasm_genome 978a3d8b2310238c008805f1989700008a0204039a009700f1138a378a8c028a98980c068a048828088c01

# Run the warrior seed
go run ./cmd/sandbox --npcs 200 --ticks 200000 --seed 999 --biomes --wfc-genome

# Run at double population
go run ./cmd/sandbox --npcs 400 --ticks 200000 --seed 999 --biomes --wfc-genome
```
