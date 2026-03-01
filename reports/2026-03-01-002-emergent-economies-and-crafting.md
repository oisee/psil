# Emergent Economies, Crafting, Mental Health, and World-Object Evolution

**Status:** Design brainstorm (critical review)
**Depends on:** [Emergent NPC Societies](2026-03-01-001-emergent-npc-societies.md), Phase 1 implementation (trade & inventory)
**Validated by:** 50,000-tick simulation showing stable population, emergent trade rediscovery, spatial clustering

---

## 1. What We Learned from the 50k-Tick Simulation

Before proposing anything new, what actually happened:

- **Population stability**: 20 NPCs survived indefinitely once goal-based navigation replaced random walks
- **Trade emergence**: The GA independently re-evolved trade behavior 3 times after the seeded traders were culled (ticks ~8800, ~43800, ~46400). Total: 301 trades across 50,000 ticks
- **Spatial clustering**: 8-12 groups formed, with trading hubs of 4-5 NPCs concentrating gold and items
- **Item saturation**: 12-15 of 20 NPCs held items at any given time. Items saturated at 8 on the map (the cap)
- **Fitness was boring**: Linearly growing with age. The best NPC lived 49,799 ticks and had fitness 83,309, dominated entirely by longevity. Gold contributed almost nothing (170 total across the whole population)

**The honest assessment:** We have a stable ecosystem with detectable emergent trade, but it's not a *civilization*. NPCs forage, occasionally trade, and mostly just survive. The economy is shallow. There's no specialization, no division of labor, no information asymmetry, no reason to trade beyond "I bumped into you and we both have items."

---

## 2. Navigation: Goals, Coordinates, and Aliases

### 2.1 Current State (Direction Sensors)

Ring0 provides `FoodDir`, `NearDir`, `ItemDir` — the direction (N/E/S/W) toward the nearest X. The brain writes the direction to Ring1[move]. This works but is limiting: the brain can only pursue the *nearest* instance of a category.

### 2.2 Proposal: Goal-Slot Navigation

Replace Ring1[move] with Ring1[goal]. The brain writes a goal ID, the framework resolves it to movement:

```
Ring1[goal]:
  0 = idle
  1 = toward nearest food
  2 = toward nearest item
  3 = toward nearest NPC
  4 = flee from nearest NPC
  5 = toward home marker
  6 = toward quest site
  7 = toward NPC specified by Ring1[target]
```

A 4-byte brain can now express goal-seeking behavior:
```asm
2         ; push 2 (goal = nearest item)
r1! 0     ; set goal
yield
```

### 2.3 Proposal: Named Locations (Aliases)

Locations on the map get IDs. NPCs "learn" locations through exploration. Ring0 exposes known locations:

```
Ring0[known_loc_count]  = how many locations I know
Ring0[nearest_known]    = distance to nearest known location
Ring0[nearest_known_id] = ID of nearest known location
```

### 2.4 Critical Review

**Praise:** Goal-slot navigation is elegant. It collapses pathfinding into a single byte of brain output, which is exactly right for 30-byte genomes. The framework handles the hard part.

**Criticism:** This is *too* helpful. If the brain just writes "go to food" and the framework does all the work, there's no evolutionary pressure to develop interesting navigation strategies. The brain becomes a trivial priority selector: "am I hungry? goal=food. do I have an item? goal=NPC." That's a 10-byte genome. What do the other 54 bytes evolve into? Probably junk.

Direction sensors force the brain to do *some* spatial reasoning. That's good. The brain has to decide "food is North but danger is also North — do I go North or flee South?" Goal-slot eliminates this tension.

**Named locations** are worse. They add complexity to the framework for marginal benefit. In a 32x32 world, "explore until you find a thing" takes at most 64 ticks of random walking. Location memory solves a problem that doesn't really exist at this scale.

**Improvement:** Keep direction sensors as the primary navigation mechanism. Add goal-slot as a *secondary* system only for goals that can't be expressed as directions (e.g., "go home" requires remembering a coordinate). The brain can write either a direction (1-4) or a goal (5+) to the same Ring1 slot. This preserves evolutionary pressure while enabling new capabilities.

Named locations should wait for Phase 6+ when the world is large enough (64x64 or bigger) to make exploration non-trivial, and when the knowledge buffer exists to store them.

---

## 3. Brain Capacity: 64, 128, or 256 Bytes?

### 3.1 Current Limits

- **MinGenome = 16 bytes**, MaxGenome = 64 bytes
- The trader genome (our most complex seed) is 25 bytes
- The warrior genome (3-way branching) is 35 bytes
- Ring0R/Ring1W are 2-byte instructions. A sense-decide-act cycle costs ~8 bytes minimum
- With 64 bytes, you get roughly 3-4 conditional branches and 5-6 distinct behaviors

### 3.2 What More Capacity Enables

At **64 bytes** (current): simple if/else chains. "If hungry eat, elif enemy flee, elif item trade, else wander." One level of priority.

At **128 bytes**: nested conditionals, state machines via `load`/`store`. "If I have a tool AND my trust of nearby NPC > 0 AND they have food, trade. Elif I know a quest site AND I have the key, go there." Two levels of priority, contextual decisions.

At **256 bytes**: multi-phase plans, subroutine-like patterns (using quotations), rudimentary counting. "Spend 10 ticks foraging, then seek a trade partner, then go to the quest site." Temporal planning emerges.

### 3.3 The GA Problem

Larger genomes are harder to evolve. The search space for a 256-byte genome is astronomically larger than for 64 bytes. With 6 mutation operators and tournament-3 selection, our GA struggles to evolve even the 2-instruction trade sequence (`push 4, r1! 1`). Doubling genome size would make this worse.

### 3.4 Critical Review

**Praise:** 128 bytes is the sweet spot. It's enough for 2-level decision trees with 6-8 distinct behaviors, small enough for the GA to explore, and fits nicely in Z80 memory (128 bytes x 32 NPCs = 4KB, one 128K bank page).

**Criticism of 256:** We can't even fill 64 bytes with useful code — the best evolved genomes are mostly junk with a small kernel of functional bytecode surrounded by dead code. 256 bytes would be 80%+ junk. The GA doesn't have enough selection pressure to fill that space, and crossover would more often *destroy* working code by inserting junk into it.

**Criticism of staying at 64:** The current max is tight for the behaviors we want. A brain that forages + trades + does quests + has conditional trust logic needs at least 80-100 bytes. 64 is constraining.

**Improvement:** Raise MaxGenome to 128. BUT also change the GA to be smarter about larger genomes:

1. **Functional length tracking**: Track where the first `yield`/`halt` occurs. Mutations and crossover should concentrate in the functional prefix, not the junk suffix.
2. **Bloat penalty**: Subtract a small fitness penalty for genome length beyond the first yield. This creates pressure toward compact, efficient brains.
3. **Modular crossover**: Instead of single-point crossover, identify "blocks" (sequences between yields/halts/jumps) and swap entire blocks. This preserves functional units.

---

## 4. Artifacts That Modify NPC Properties

This is the most interesting proposal. Items aren't just tradeable tokens — they change what the NPC *can do*.

### 4.1 Gas-Modifying Artifacts

The VM's gas limit (currently 200) determines how much the brain can think per tick. Artifacts that modify gas create a direct link between the economy and cognitive capacity.

| Artifact | Effect | How Obtained |
|----------|--------|-------------|
| **Scroll of Wisdom** | +50 gas permanently | Rare spawn (1 per 5000 ticks) |
| **Crystal Focus** | +100 gas this tick only | Consumed on use, quest site reward |
| **Mind Leech** | Steal 25 gas from adjacent NPC for this tick | Crafted (weapon + treasure) |
| **Teaching Stone** | Loan 50 gas to target NPC; get it back next tick | Crafted (tool + tool) |

**Implementation:** The scheduler already sets `vm.Gas = s.Gas` each tick. Change to:

```go
gas := s.Gas + npc.GasBonus  // permanent artifact bonus
if npc.HasCrystalFocus {
    gas += 100
    npc.ConsumeItem(CrystalFocus)
}
```

### 4.2 Stat-Modifying Equipment

| Item Type | Slot | Effect |
|-----------|------|--------|
| **Food Pack** | held | +15 energy on eat (vs +30 raw food) — portable but less efficient |
| **Tool** | held | Auto-eat checks 8 tiles instead of 5 (larger foraging radius) |
| **Weapon** | held | Attack deals 20 damage instead of 10 |
| **Shield** | worn | Incoming damage halved |
| **Cloak** | worn | NearestNPC distance reads as +5 (harder to find) |
| **Crown** | worn | +3 charisma (memetic transmission bonus, Phase 4) |
| **Compass** | held | NearestFood/NearestItem range doubles |
| **Treasure** | stored | +2 gold per trade (profit multiplier) |

### 4.3 Rare Ores and Crafting Materials

Raw materials spawn rarely and must be combined:

| Material | Spawn Rate | Properties |
|----------|-----------|------------|
| **Iron Ore** | 1 per 500 ticks | Base crafting material |
| **Crystal Shard** | 1 per 2000 ticks | Magical crafting material |
| **Ancient Rune** | 1 per 5000 ticks | Legendary, enables gas artifacts |

### 4.4 Critical Review

**Praise:** Gas modification is genuinely novel. In most games, cognitive capacity is fixed. Here, an NPC can *acquire better thinking* through economic activity. A wealthy NPC that acquires a Scroll of Wisdom can run a more complex genome, enabling strategies that were previously gas-limited. This creates a positive feedback loop: better thinking -> better trading -> more wealth -> even better thinking.

The Teaching Stone (gas loan) is beautiful. It creates a new form of trade: "I'll lend you my thinking power in exchange for your item." This is *intellectual labor as a service*.

**Criticism:** This is over-designed. We have 6 item types, 3 materials, and crafting recipes before we've even verified that NPCs can reliably *pick up* items and *use* them strategically. The 50k-tick simulation showed that item holding is mostly incidental — NPCs walk onto items, not seek them.

The stat modifications add complexity to every part of the scheduler (combat resolution, foraging, sensing). Each modifier requires special-case code. With 8+ modifier types, the scheduler becomes a spaghetti of conditional bonuses.

**The deeper problem:** We're designing a game, not an evolutionary system. A game designer assigns value to items. An evolutionary system lets value *emerge*. If we hardcode "weapon = 2x damage," we're choosing the optimal strategy for the NPCs. They don't need to discover anything. They just need to find the weapon tile and pick it up.

**Improvement:** Start with exactly two modifiers:

1. **Gas bonus** (+50 permanent): because it creates genuinely emergent complexity (longer brains become viable)
2. **Foraging bonus** (wider auto-eat radius): because it directly improves survival and creates clear fitness advantage

Everything else should emerge from these two. An NPC with a gas bonus can run a more complex brain that *outcompetes* others. What strategies does that brain evolve? We don't know. That's the point.

---

## 5. Crafting

### 5.1 The Minimum Viable Crafting System

Crafting requires: two NPCs adjacent, both perform `ActionCraft`, both holding items. The items are consumed, a new item is created and given to the initiator.

```
Ring1[action] = 9 (ActionCraft)
Ring1[target] = adjacent NPC ID
```

Recipe resolution:

```go
func craftResult(itemA, itemB byte) byte {
    key := uint16(itemA)<<8 | uint16(itemB)
    switch key {
    case ItemTool<<8 | ItemTool:     return ItemCompass
    case ItemTool<<8 | ItemWeapon:   return ItemShield
    case ItemWeapon<<8 | ItemTreasure: return ItemCrown
    case ItemTool<<8 | ItemCrystal:  return ItemScrollOfWisdom  // gas +50
    default: return ItemNone  // crafting failed, items lost
    }
}
```

### 5.2 Why Crafting Requires Cooperation

Key design choice: crafting requires TWO NPCs. One NPC alone cannot craft. This forces social behavior — you need a partner willing to sacrifice their item.

The partner gets nothing unless the initiator trades back. This creates a trust problem (Phase 3 territory): will the crafter share the result? NPCs that cooperate and reciprocate build trust and get better crafting partners. NPCs that take the crafted item and flee lose trust and future opportunities.

### 5.3 Discoverable Recipes

Don't tell the NPCs what the recipes are. Let them discover through trial and error. The GA will favor genomes that craft successfully — but only if they stumble into valid recipes first.

To make discovery feasible: have a small number of recipes (4-6), and make the "craft failed" outcome return one of the input items instead of destroying both. This reduces the penalty for experimentation.

### 5.4 Critical Review

**Praise:** Two-NPC crafting is the best idea here. It creates a genuine reason for social behavior that isn't just "swap items." It requires trust, proximity, and coordination. The trust problem (will you share?) is exactly the kind of dilemma that evolutionary dynamics can explore.

**Criticism:** Crafting is extremely unlikely to evolve. The brain needs to output:
1. ActionCraft (value 9) — a specific constant
2. A valid target ID — requires reading Ring0[NearID]
3. Both NPCs must do this simultaneously — requires bilateral intent, just like trade
4. Both must hold compatible items — requires item awareness

Trade already barely evolves (301 trades in 50k ticks, most from seeded genomes). Crafting is *harder* than trade (more preconditions). It won't happen by mutation alone.

**The recipe problem:** If there are 5 item types, there are 25 possible pairs. Only 4-6 produce results. The NPC has no way to know which pairs work. The GA has to discover valid recipes by accidentally producing crafting behavior AND holding the right items AND being next to a partner with the right items AND both crafting simultaneously. The probability is vanishingly small.

**Improvement:** Phase crafting in two stages:

**Stage 1 (Solo Crafting):** An NPC standing on a special "forge" tile with an item can craft alone. The forge + item determines the result. This is discoverable: the NPC just needs to walk onto the forge while holding an item. The GA can discover this easily — it's the same as "walk onto food + eat."

**Stage 2 (Cooperative Crafting):** Once solo crafting is established and NPCs understand "forge tiles are valuable," introduce two-NPC recipes that produce *better* items than solo recipes. Now there's a reason to cooperate: the cooperative result is worth more.

Ring0 sensor for this:
```
Ring0[on_forge]      = 1 if standing on forge tile, 0 otherwise
Ring0[forge_result]  = what item I'd get if I craft now (0 = nothing)
```

The brain just needs:
```asm
r0@ forge_result   ; would crafting produce something?
0                  ; push 0
>                  ; result > 0?
jnz do_craft       ; yes → craft
; else continue foraging
```

This is 8 bytes. Evolvable.

---

## 6. World-Object Evolution and Rare Spawns

### 6.1 Object Rarity Tiers

| Tier | Spawn Rate | Example | Effect |
|------|-----------|---------|--------|
| Common | 1 per 20 ticks | Food, Tool, Weapon | Basic survival and trade |
| Uncommon | 1 per 200 ticks | Treasure, Shield | Economic and defensive value |
| Rare | 1 per 2000 ticks | Crystal Shard, Compass | Crafting material, enhanced sensing |
| Legendary | 1 per 10000 ticks | Ancient Rune, Scroll of Wisdom | Gas modification, permanent upgrades |

### 6.2 Procedural Artifacts

Instead of hardcoded item types, generate artifacts procedurally:

```go
type Artifact struct {
    Base     byte // tool, weapon, armor, charm
    Modifier byte // fire, ice, mind, speed, sight
    Quality  byte // 0-7 (affects magnitude)
}
```

This gives 4 x 5 x 8 = 160 distinct artifacts from 3 bytes. An artifact's effect is determined by its components:

- Base=weapon + Modifier=fire + Quality=5 = "Flame Sword" (+15 attack damage)
- Base=charm + Modifier=mind + Quality=7 = "Mind Crystal" (+70 gas permanently)
- Base=tool + Modifier=speed + Quality=3 = "Swift Pick" (+3 foraging radius)
- Base=armor + Modifier=sight + Quality=2 = "Seer's Cloak" (+2 sensing range)

### 6.3 Critical Review

**Praise:** Procedural artifacts create variety without hardcoding. The NPC doesn't need to understand 160 item types. Ring0 just reports the *effect*: "my item gives +50 gas" or "my item gives +3 foraging." The brain decides based on effects, not item names.

Rarity tiers create economic stratification. NPCs that find legendary items become powerful. Other NPCs must trade to compete. This is the seed of social hierarchy.

**Criticism:** 160 artifact types in a population of 20 NPCs is absurd. Most artifacts will never be seen. The GA can't learn anything about items it never encounters. With 20 NPCs and a rare item spawning every 2000 ticks, an NPC might encounter a rare item once in its entire 50,000-tick lifetime.

Procedural generation sounds elegant but it pushes complexity into the sensing system. How does the brain decide if "Flame Sword" is better than "Swift Pick"? It needs Ring0 slots for the item's attack bonus, foraging bonus, gas bonus, sensing bonus... that's 4+ Ring0 slots consumed by item effects alone.

**The real problem:** We're adding variety for the sake of variety. The 50k-tick simulation showed that NPCs don't even *deliberately seek* items yet. They pick them up incidentally while foraging. Adding 160 item types doesn't change this dynamic. It just makes the display output more colorful.

**Improvement:** Start with 3 rarity tiers, 6-8 total item types, all with clearly distinct mechanical effects:

| Item | Tier | Mechanical Effect |
|------|------|-------------------|
| Food Pack | common | Restores 30 energy (same as food tile) |
| Tool | common | +1 auto-eat radius (check 8 tiles) |
| Weapon | common | 2x attack damage |
| Shield | uncommon | Half incoming damage |
| Treasure | uncommon | +3 gold per trade |
| Crystal | rare | +50 gas permanently |
| Forge Key | rare | Enables crafting at forge tile |

Seven items. Each has exactly one clear effect. The brain needs only Ring0[my_item_effect] to make decisions. No procedural generation, no modifier stacking, no quality levels. If this works — if NPCs evolve to seek crystals for gas bonuses and trade weapons for shields — *then* add complexity.

---

## 7. Gas as an Evolvable Resource

This deserves its own section because it's the most architecturally significant proposal.

### 7.1 The Idea

Gas (CPU cycles per tick) determines how much of its genome an NPC can execute. Currently fixed at 200. If gas becomes variable:

- An NPC with 200 gas can execute ~25 instructions (enough for simple if/else)
- An NPC with 400 gas can execute ~50 instructions (enough for nested conditionals + state)
- An NPC with 100 gas can barely sense + output one action

This creates a **cognitive economy**: thinking is a scarce resource. Items that modify gas change what brains are viable.

### 7.2 Interactions

**Gas + Genome Size:** A 128-byte genome with only 200 gas will hit gas exhaustion before executing most of its code. That NPC *needs* a gas-boosting artifact to unlock its full potential. The artifact doesn't just help — it *enables* strategies that were impossible before.

**Gas + Trade:** An NPC can lend gas (Teaching Stone). This means: "Run my complex trading algorithm using your extra gas." The lender sacrifices thinking power temporarily; the borrower gains capability. This is *intellectual labor for hire*.

**Gas + Crafting:** If the Crystal item (rare) gives +50 gas permanently, then the Crystal is the most valuable item in the economy. NPCs should evolve to seek Crystals above all else. The fitness advantage of +50 gas compounds over an NPC's lifetime: better decisions every single tick.

### 7.3 Critical Review

**Praise:** This is the single most interesting mechanic because it creates a *feedback loop between economy and cognition*. No other game does this. In WoW, a wealthy character and a poor character have the same cognitive capacity (the player). Here, wealth literally makes you smarter.

The evolutionary implications are profound. A population of gas-poor NPCs evolves simple brains. When Crystal items appear, the NPC that acquires one can run a more complex brain. If that brain is better at acquiring more Crystals (or trading for them), you get runaway cognitive development in a subpopulation. This is analogous to the "expensive tissue hypothesis" in human evolution — better food enabled bigger brains, which enabled better food acquisition.

**Criticism:** The feedback loop could also be destructive. If gas-rich NPCs dominate so thoroughly that gas-poor NPCs can't compete, the population loses diversity. The GA might converge on a single "rich + complex brain" strategy, killing off alternative strategies. Monocultures are fragile — a single mutation that disrupts the dominant strategy could crash the population.

Also, gas modification requires careful balancing. If +50 gas is too cheap, everyone gets it and it's meaningless. If it's too expensive, nobody gets it and it's meaningless. The sweet spot is narrow.

**Improvement:** Make gas bonuses diminishing-returns. First Crystal gives +50. Second gives +25. Third gives +12. This prevents runaway accumulation while preserving the incentive to acquire the first one.

Also: gas *costs* for some actions. Trade costs 10 gas. Crafting costs 20 gas. Attack costs 5 gas. This means a gas-poor NPC literally can't afford complex actions. It must forage (cheap) until it can afford to trade (expensive). Gas becomes the universal currency of capability.

---

## 8. Peer-to-Peer Trade with Gain and Loss

### 8.1 Current Trade: Pure Swap

NPC A gives item X, NPC B gives item Y. No gold changes hands. Both gain and lose equally. There's no economic motivation — the swap only helps if you specifically want the other item.

### 8.2 Proposal: Price-Based Trade

Add Ring1[offer_gold] to trade. Each NPC offers an item + gold amount. The framework resolves:

```go
func resolveTrade(a, b *NPC) {
    // Both must hold items and offer trade
    if !bilateral { return }

    valueA := marketValue(a.Item)  // scarcity-based
    valueB := marketValue(b.Item)

    // Swap items
    a.Item, b.Item = b.Item, a.Item

    // Gold transfer based on value difference
    diff := valueA - valueB
    a.Gold += diff / 2
    b.Gold -= diff / 2
}
```

### 8.3 Scarcity-Driven Market Value

```go
func marketValue(item byte, world *World) int {
    total := world.ItemCount()
    thisType := world.ItemCountByType(item)
    if thisType == 0 { return 100 }  // priceless
    return 10 * total / thisType     // inversely proportional to supply
}
```

If there are 8 items total and only 1 Crystal, Crystal's value = 80. If there are 4 Tools, Tool's value = 20. Trading a Crystal for a Tool nets +30 gold.

### 8.4 Critical Review

**Praise:** Scarcity-based pricing creates emergent market dynamics with zero designer input. Rare items are automatically valuable. Gluts crash prices. This is actual supply-and-demand economics from a 5-line function.

**Criticism:** The brain can't participate in pricing. The framework computes market value and adjusts gold automatically. The NPC doesn't "decide" to sell high or buy low — it just trades and the framework handles the rest. This isn't emergent economics; it's automated economics. The emergence is in *what* NPCs trade, not *how* they trade.

Real economic emergence would require NPCs to *set* prices. But that requires the brain to output a bid/ask price, which requires understanding value, which requires memory and comparison, which requires >64 bytes of genome and >200 gas. We're not there yet.

**Improvement:** For now, let the framework handle pricing. It's better to have functional scarcity-based prices than no prices at all. When genome size increases to 128 and gas becomes variable, revisit NPC-set pricing. The brain could output Ring1[min_price] = "minimum gold I'll accept for my item." The framework only executes trades where both parties' prices are satisfied.

---

## 9. Quest Sites and Emergent Quests

### 9.1 Forge Tiles (Solo Crafting)

Special tiles that transform items. An NPC walks onto a forge holding an item, outputs `ActionCraft`, and receives a different item.

```
Forge + Tool    = Compass (wider sensing)
Forge + Weapon  = Shield (defense)
Forge + Crystal = Scroll of Wisdom (gas +50)
```

### 9.2 Dungeon Tiles (Key-Gate)

Tiles that require a specific item to activate:

```
Dungeon + Forge Key = 50 gold + random rare item
```

### 9.3 Shrine Tiles (Temporary Buff)

Tiles that grant temporary bonuses:

```
Shrine = +100 gas for next 10 ticks
```

### 9.4 Critical Review

**Praise:** Forge tiles are the right entry point. Solo, discoverable, mechanically simple. The NPC just needs to walk onto a tile and craft. The GA can discover this in a few thousand ticks.

Dungeon tiles create the first genuine "quest pattern": acquire key, travel to dungeon, activate. This is a 3-step plan that requires temporal reasoning or at least conditional branching. Evolving this behavior would be a significant milestone.

**Criticism:** We're adding 3 new tile types to an already crowded tile system. With 4 bits for tile type, we have 16 total slots. Currently used: Empty(0), Wall(1), Food(2), Water(3), Tool(4), Weapon(5), Treasure(6). Adding Forge(7), Dungeon(8), Shrine(9) uses 10 of 16. That's manageable but tight.

The bigger concern: quest behavior requires multi-step planning. The brain needs to: (1) sense it has a key, (2) remember that a dungeon exists, (3) navigate to it. Steps 1 and 3 are feasible with Ring0 sensors. Step 2 requires memory — the NPC must know the dungeon exists even when it's not the nearest thing. This is Phase 2 territory (knowledge buffer).

**Improvement:** Implement forge tiles in Phase 1.5. They require no memory, no knowledge buffer, no multi-step planning. Just "walk onto tile + craft." Dungeon and shrine tiles wait for Phase 2 when the knowledge buffer enables spatial memory.

---

## 10. Recommended Implementation Order

Based on the critical review, prioritizing by *impact on emergent behavior* vs *implementation complexity*:

### Phase 1.5: Gas Artifacts + Forge Tiles
- Raise MaxGenome to 128
- Add Crystal item type (rare, +50 gas permanently, diminishing returns)
- Add Forge tile type
- Solo crafting: forge + item = upgraded item
- Ring0[on_forge], Ring0[forge_result], Ring0[my_gas]
- **Why first:** Gas modification creates the richest feedback loop with minimal code changes

### Phase 1.7: Scarcity-Based Trade Pricing
- Add `marketValue()` based on item supply
- Gold changes hands on trade proportional to value difference
- Ring0[my_item_value] sensor
- **Why second:** Creates genuine economic motivation for trade

### Phase 2.0: Knowledge Buffer (per existing genplan)
- NPCs remember 8 world facts
- Enables multi-step plans (remember dungeon location)
- Prerequisite for dungeon/shrine/quest mechanics

### Phase 2.5: Dungeon Tiles + Key-Gate Quests
- Dungeon tiles require key item to activate
- Reward: gold + rare item
- First genuine "quest" pattern

### Phase 2.5b: Stress System
- Stress accumulator per NPC (0-100)
- Stress-based output override (probabilistic random action at high stress)
- Stress events: damage, starvation, failed trade, adjacent death
- Stress decay: eating, trading, resting in safety
- Ring0[my_stress] sensor
- **Why here:** No new actions needed, just scheduler logic. Creates behavioral variety without genome changes.

### Phase 3.0: Trust + Cooperative Crafting
- Trust matrix (per existing genplan)
- Two-NPC crafting at forge tiles
- Trust-gated recipes (better results with trusted partners)

### Phase 3.5: Healing + Corruption Detection
- OrigGenome snapshot per NPC (or hash for Z80)
- ActionHeal (stress relief, -30 stress)
- ActionReset (restore genome to birth state)
- Ring0[my_coherence], Ring0[my_corrupted], Ring0[near_stress]
- Healer fitness reward (+15 fitness, +10 energy per successful heal)
- Therapist seed genome
- **Why after trust:** Healing is a social action; trust determines who heals whom

---

## 11. What This Report Got Wrong (Self-Critique)

1. **Scope creep.** This report proposes 15+ new mechanics. The simulation can barely evolve 2-instruction trade. We should be embarrassed about proposing procedural artifacts with quality levels when NPCs don't yet deliberately seek items.

2. **Designer bias.** Half of these proposals are "what would make a fun game" not "what would create interesting evolution." Games need content; evolution needs constraints and gradients. The best changes are those that create *selection pressure*, not *content*.

3. **Z80 amnesia.** None of this considers the Z80 port (Phase 5-7). Adding 160 artifact types, crafting recipes, forge tiles, and market pricing to a 3KB Z80 sandbox is delusional. Every mechanic must fit in ~50 bytes of Z80 code or it doesn't ship.

4. **The GA bottleneck.** We keep proposing behaviors for NPCs to evolve, but the GA can barely evolve `push 4, r1! 1` (trade action). The real work isn't adding more things to evolve — it's making the GA better at evolving the things we already have. GA improvements (functional length tracking, bloat penalty, modular crossover) would have 10x more impact than any new mechanic.

5. **Crafting skepticism.** Cooperative crafting sounds great on paper. In practice, it requires bilateral intent + compatible items + adjacency + trust. Trade already requires bilateral intent + adjacency and barely works. Adding more preconditions makes it exponentially less likely to evolve.

---

## 12. The One Thing That Matters Most

If we could only implement one thing from this entire report, it should be:

**Variable gas from Crystal items.**

Because:
- It's 10 lines of scheduler code
- It creates a positive feedback loop (more gas -> better brain -> more gas acquisition)
- It gives the GA something to select for beyond longevity
- It makes MaxGenome=128 meaningful (currently, 64 bytes at 200 gas is already enough)
- It creates the first *genuine* reason for NPCs to seek items (not just incidental pickup)
- It's the only mechanic where the NPC's *cognitive architecture* changes based on world interaction

Everything else — quests, crafting, fashion, named locations — is content. Variable gas is *architecture*.

---

## 13. Mental Health, Brainwashing, and Therapist NPCs

### 13.1 What Is "Mental Health" in a Bytecode Brain?

The NPC's genome IS its mind. When mutation corrupts a working genome, that's brain damage. When crossover grafts a foreign code block into a functional brain, that's a personality disorder. When a hostile NPC overwrites another's memory (Phase 4 memetic transmission), that's brainwashing.

So "mental health" already exists — we just don't track it.

**Proposed metric: Genome Coherence.** Measure how "healthy" a brain is:

```go
type MentalState struct {
    Coherence   int  // 0-100: how functional the genome is
    Stress      int  // accumulates from damage, starvation, failed actions
    Corrupted   bool // genome was externally modified (brainwash/memetic)
    OrigGenome  []byte // snapshot of genome at birth / last "therapy"
}
```

**Coherence** is computed heuristically:
- Does the genome reach `yield` or `halt` before gas exhaustion? (+30)
- Does it write to Ring1 (produce output)? (+30)
- Does it read Ring0 (sense the world)? (+20)
- Is the functional prefix (before first yield) > 50% of total length? (+20)
- Was the genome externally modified since birth? (-40)

An NPC with coherence < 30 is "mentally ill" — its brain runs but produces garbage output.

### 13.2 Stress as a Degradation Mechanic

**Stress** accumulates from adverse events and degrades decision-making:

| Event | Stress Change |
|-------|---------------|
| Take damage (attack) | +15 |
| Starvation (energy < 50) | +5 per tick |
| Failed trade (unilateral) | +3 |
| Failed craft | +5 |
| Adjacent NPC death | +10 |
| Successful eat | -2 |
| Successful trade | -5 |
| At rest (energy > 150, no enemies nearby) | -1 per tick |

When stress exceeds a threshold, bad things happen:

| Stress Level | Effect |
|-------------|--------|
| 0-30 | Healthy — no effect |
| 31-60 | Anxious — 10% chance of ignoring brain output, doing random action instead |
| 61-80 | Unstable — 25% chance of random action; auto-eat threshold raised (eats compulsively) |
| 81-100 | Breakdown — 50% random action; gas halved (can barely think); wanders aimlessly |

This is elegant because stress affects the NPC *through the existing scheduler*. The brain still runs normally — stress just overrides its output probabilistically. A stressed NPC looks "confused" to observers: it has a working brain but erratic behavior.

### 13.3 Brainwashing (Memetic Corruption)

Phase 4 introduces memetic transmission: NPCs copy bytecode fragments to each other. This is how culture spreads. But it's also how brainwashing works.

**Hostile memetics:** An NPC with a "persuasion" genome writes a code fragment into an adjacent NPC's genome. The target NPC now runs foreign code — it might trade away its items, walk into danger, or attack its allies.

**Detection:** The `OrigGenome` snapshot lets us detect corruption. If `genome != OrigGenome`, the NPC has been modified. Ring0 can report this:

```
Ring0[corrupted] = 1 if genome differs from birth snapshot
Ring0[stress]    = current stress level (0-100)
```

The brain can (if sophisticated enough) detect its own corruption and seek help.

### 13.4 Therapist NPCs (The Healer Role)

A new action: `ActionHeal = 10`. An NPC performing ActionHeal on an adjacent target:

**Option A: Full Reset.** Restore the target's genome to `OrigGenome`. This is the "priest" model: absolution, return to factory settings. Pros: simple, guaranteed to fix corruption. Cons: also erases beneficial mutations acquired since birth. The NPC loses everything it learned.

**Option B: Stress Relief.** Reduce target's stress by 30. Don't touch the genome. This is the "therapist" model: the brain is unchanged but the stress overlay is reduced, so brain output is respected again. Pros: preserves learned behavior. Cons: doesn't fix actual genome corruption.

**Option C: Selective Repair.** Compare target's genome to its `OrigGenome`. Revert only bytes that differ AND are in non-functional regions (after the first `yield`). This is the "psychiatrist" model: fix the damage without losing the growth. Pros: best of both worlds. Cons: complex to implement, and "non-functional region" is a heuristic that can be wrong.

**Recommendation:** Implement Option B first (stress relief), with Option A available as a separate action (`ActionReset = 11`). Let evolution decide which is more useful. A "therapist genome" might look like:

```asm
; Therapist: find stressed NPC, move toward them, heal
r0@ stress_near    ; nearest NPC's stress level
30                 ; push threshold
>                  ; nearby NPC stressed?
jnz do_heal        ; yes → heal them
; else forage normally
r0@ 13             ; food direction
r1! 0              ; move toward food
1                  ; eat
r1! 1              ; action
yield

; heal block
r0@ 18             ; nearest NPC direction
r1! 0              ; move toward them
10                 ; ActionHeal
r1! 1              ; action
r0@ 12             ; nearest NPC ID
r1! 2              ; target
yield
```

This is ~30 bytes — well within the 128-byte genome. It creates a new NPC archetype: the healer who derives fitness not from personal survival but from keeping others alive.

### 13.5 Fitness for Healers

The problem: healers spend time healing instead of eating. They starve unless healing grants fitness.

**Proposal:** Each successful heal grants the healer +15 fitness and +10 energy (a "social reward"). The healed NPC could also transfer 2 gold to the healer (payment for services).

This creates a viable economic niche: healers sustain themselves by providing a service. If the population is healthy (low stress), healers are useless and get outcompeted. If the population is stressed (lots of combat or starvation), healers thrive. This is self-regulating.

### 13.6 Emergent Social Dynamics

The mental health system creates several interesting dynamics:

**1. PTSD-like patterns.** An NPC that survives combat accumulates stress. Even after the danger passes, it behaves erratically (stress > 60). It needs a healer or a long period of safety to recover. This is analogous to trauma — the body is fine but the mind is degraded.

**2. Cult-like brainwashing.** A charismatic NPC with memetic transmission could brainwash a cluster of NPCs into serving it — trading it items, following it around, attacking its enemies. The brainwashed NPCs have corrupted genomes and elevated stress. A therapist NPC could "deprogram" them by resetting their genomes.

**3. The healer's dilemma.** If healing costs gas (say 20 gas per heal), then only gas-rich NPCs can afford to be healers. This connects back to Crystal items: an NPC that finds a Crystal (+50 gas) gains enough cognitive budget to be both a survivalist AND a healer. Wealth enables altruism.

**4. Anti-social evolution.** The GA might evolve "anxiety exploiters" — NPCs that deliberately stress others (attacking, stealing, brainwashing) and then offer healing for a price. Protection rackets. The victim is stressed → seeks healing → pays the healer → the healer is the same NPC that caused the stress. This is a parasitic strategy that the GA could plausibly discover.

**5. Herd immunity.** If enough NPCs in a cluster are healers, the cluster becomes resistant to brainwashing — any corrupted NPC gets quickly reset. Clusters without healers are vulnerable. This creates selective pressure for clusters to "recruit" or evolve healers, analogous to how social groups maintain norms.

### 13.7 Ring0 Sensors for Mental Health

```
Ring0[my_stress]       = 20   ; own stress level (0-100)
Ring0[my_coherence]    = 21   ; own genome coherence (0-100)
Ring0[my_corrupted]    = 22   ; 1 if genome modified since birth
Ring0[near_stress]     = 23   ; nearest NPC's stress level
```

These fit into the existing Ring0 extended range (slots 20-23, within Ring0ExtCount=24).

### 13.8 Critical Review

**Praise:** This is the most *socially* interesting mechanic proposed so far. Trade is economic. Crafting is material. Mental health is *relational*. It creates roles (healer, aggressor, victim) and dynamics (trauma, recovery, exploitation) that don't exist in any other mechanic. The therapist NPC is the first archetype whose value is entirely social — it produces nothing, gathers nothing, but makes the population function better.

The stress-as-output-override is a clean design. It doesn't add a new VM instruction or Ring0 slot for "confusion." It just probabilistically corrupts the scheduler's reading of Ring1. The brain doesn't know it's being overridden. This is how stress actually works: you know what you should do, but your body does something else.

**Criticism:** We're now proposing 4 new Ring0 slots, 2 new actions (ActionHeal, ActionReset), a stress accumulator per NPC, a genome snapshot per NPC, and a coherence heuristic. This is significant complexity for a system that might never emerge evolutionarily.

Healing has the same bilateral-intent problem as trade: the healer must target a specific NPC, be adjacent, and output ActionHeal. We already know bilateral actions are hard to evolve. Healing is slightly easier (unilateral — only the healer needs to act) but still requires sensing stress, navigating to the target, and outputting the right action + target.

**The deeper concern:** Are we designing a system where mental health *matters*, or one where it's just a debuff that NPCs occasionally clear? If stress just adds noise to output and healers remove that noise, it's a mechanical tax on the population, not a meaningful dynamic. For mental health to *matter*, the population must be better off WITH healers than without, and the difference must be large enough for the GA to select for healing behavior.

**The OrigGenome problem:** Storing a birth snapshot of every NPC's genome doubles memory usage per NPC. For 20 NPCs with 128-byte genomes, that's 2.5KB extra — fine on the host, but problematic for Z80 (where we have 4KB total for all NPC genomes). Possible compromise: store only a hash of the original genome (2 bytes) for corruption detection, and have the "reset" action restore a *species template* instead of the individual's birth genome.

**Improvement:** Implement in two stages:

**Stage 1 (Phase 2.5): Stress only.** Add the stress accumulator and the output-override mechanic. No healing action, no genome snapshots. Just: stressed NPCs behave erratically, and stress decays over time in safe conditions. This alone creates interesting dynamics — clusters near food are "healthy," frontier NPCs are stressed.

**Stage 2 (Phase 3.5): Healing + corruption detection.** Add ActionHeal (stress relief), genome snapshots, corruption detection. This requires the trust system (Phase 3) to be meaningful — otherwise healer NPCs have no way to decide who to heal.

---

## 14. The One Thing That Matters Most (Revised)

If we could only implement **two** things from this entire report:

1. **Variable gas from Crystal items** — because it creates a feedback loop between economy and cognition (Section 12's argument still holds).

2. **Stress accumulation with output override** — because it's 15 lines of scheduler code, requires no new actions or Ring0 slots initially, and creates the first mechanic where an NPC's *internal state* degrades its behavior independently of its genome. It's the seed from which all social dynamics (healing, brainwashing, trust) can grow.

Together, these two create a world where NPCs vary in both *cognitive capacity* (gas) and *emotional stability* (stress). One is acquired through items, the other through experience. One makes you smarter, the other makes you erratic. The interplay between them — a smart but stressed NPC, a dumb but calm NPC, a rich NPC that can afford therapy — is where emergent social structure lives.

---

## 15. The Brighter Side: Cooperation, Celebration, and Flourishing

Section 13 leaned heavy on trauma, exploitation, and parasitic strategies. That's one evolutionary attractor — but probably not the dominant one. Cooperative strategies consistently outperform parasitic ones in iterated games (Axelrod's tournaments). Here's what the *same mechanics* look like when things go right:

**Mutual aid clusters.** A group of 4-5 NPCs near a food source develops low stress naturally (safe, well-fed). Low-stress NPCs produce reliable brain output → better trading → gold accumulates → they can afford gas-boosting Crystals → smarter brains → even better cooperation. The cluster becomes a pocket of prosperity. This is the *default* attractor, not the exception.

**The village healer.** A healer NPC in a stable cluster barely needs to heal (everyone's calm). It forages alongside others, occasionally topping up a stressed newcomer. It's not a sacrifice role — it's a part-time job. The healer thrives because the cluster thrives.

**Teaching and mentoring.** Gas-loaning (Teaching Stone) isn't exploitation — it's a mentor giving a junior NPC temporary cognitive power to execute a complex trade or crafting sequence. The mentor gets the stone back next tick. The junior gets experience (fitness) it couldn't have earned alone. Both benefit.

**Celebration as stress relief.** If we add a tile type or action for "gathering" (multiple NPCs in one spot, no conflict), it could reduce stress for all participants. Festivals. Markets. Town squares. NPCs cluster not because they're trading but because proximity itself is calming. This is the emergent equivalent of community.

**The optimistic prediction:** Given equal starting conditions, cooperation emerges faster than exploitation because it's simpler. "Move toward NPCs + trade" is a 20-byte genome. "Stress others + offer healing for payment" requires sensing stress, causing damage, tracking targets, and conditional healing — that's 60+ bytes. The GA will find cooperation long before it finds parasitism. Darkness requires sophistication. Kindness is the default.

---

---

## 16. Tit-for-Tat with Forgiveness in Bytecode

### 16.1 Axelrod's Result

In iterated prisoner's dilemma tournaments, tit-for-tat (cooperate first, then copy opponent's last action) wins. But tit-for-tat-with-forgiveness (cooperate first, copy opponent, but occasionally cooperate even after a defection — ~5-10% chance) beats strict tit-for-tat. It breaks retaliatory spirals.

Can our bytecode brains express this? Yes — and the forgiveness rate becomes an **evolvable parameter** that the GA tunes.

### 16.2 What the Framework Tracks

Trust is a per-pair value maintained by the scheduler, not by the brain:

```go
type TrustMatrix struct {
    trust map[[2]byte]int8 // (npcA_id, npcB_id) → trust score (-31 to +31)
}
```

Events update trust:
- Successful bilateral trade: +3 both sides
- Unilateral trade attempt (I offered, they didn't): -2 for the defector
- Heal received: +5
- Attack received: -10
- Adjacent for 5+ ticks peacefully: +1 (familiarity)

Ring0 exposes the trust score for the nearest NPC:

```
Ring0[near_trust] = 17   ; trust level with nearest NPC (-31 to +31)
```

### 16.3 Forgiveness as a Threshold Byte

Here's the key insight. Tit-for-tat-with-forgiveness in PSIL is:

```asm
; Tit-for-tat with forgiveness
r0@ 17              ; read trust with nearest NPC (-31 to +31)
N                   ; push forgiveness threshold (THE EVOLVABLE PARAMETER)
>                   ; trust > threshold?
jnz cooperate       ; yes → trade with them
; else defect (forage alone / flee)
r0@ 13              ; food direction
r1! 0               ; move toward food
1                   ; eat
r1! 1               ; action
yield

; cooperate block
r0@ 18              ; nearest NPC direction
r1! 0               ; move toward them
4                   ; ActionTrade
r1! 1               ; action
r0@ 12              ; nearest NPC ID
r1! 2               ; target
yield
```

The threshold `N` is a SmallNum opcode (0x20-0x3F = values 0-31). That single byte IS the forgiveness rate:

| Threshold N | Meaning | Strategy |
|-------------|---------|----------|
| 0 | Cooperate if trust > 0 | **Strict tit-for-tat** — one defection and you're done |
| -5 (via push byte) | Cooperate even at trust -5 | **Forgiving** — tolerates a few defections |
| -15 | Cooperate even at trust -15 | **Very forgiving** — almost always cooperates |
| -31 | Always cooperate | **Unconditional cooperator** (sucker strategy) |
| 15 | Only cooperate at trust > 15 | **Grudging** — needs long history of good behavior |
| 31 | Never cooperate | **Always defect** |

The GA mutates this byte. A population of NPCs with different thresholds reproduces Axelrod's tournament *in bytecode*. Point mutations shift the threshold by 1 — that's a gentle exploration of the forgiveness landscape.

### 16.4 Encoding Negative Thresholds

SmallNum only encodes 0-31. For negative thresholds (the forgiving ones!), two options:

**Option A: Offset encoding.** Ring0[near_trust] reports trust as 0-62 instead of -31 to +31 (add 31). Threshold 31 = "cooperate at neutral trust." Threshold 16 = "cooperate at trust -15." No negative numbers needed.

**Option B: Use OpNeg.** The brain pushes a positive number and negates:

```asm
r0@ 17       ; trust
5            ; push 5
neg          ; negate → -5
>            ; trust > -5?
jnz cooperate
```

Costs one extra byte (0x11 = OpNeg). The GA can discover this — `neg` is a 1-byte opcode in the mutation pool.

**Recommendation:** Option A (offset encoding). Simpler, no extra byte, the brain doesn't need to understand negative numbers. Trust 0-62, midpoint 31 = neutral. The threshold is just a SmallNum. Pure.

### 16.5 What the GA Will Find

Prediction based on Axelrod's results:

1. **Early generations (0-1000 ticks):** Random thresholds. Some NPCs always cooperate (threshold=0), some never do (threshold=31). Always-cooperate gets exploited. Always-defect gets isolated (nobody trades with it, trust crashes).

2. **Mid generations (1000-10000):** Moderate thresholds (15-25 in offset encoding, i.e. slightly forgiving to neutral) dominate. These NPCs cooperate with established partners but defect against strangers. Clusters of cooperators form.

3. **Late generations (10000+):** The winning threshold converges to ~28-30 (offset) = trust > -3 to -1 (raw). Cooperate unless recently betrayed, forgive after a few ticks of peace. This IS tit-for-tat with ~5-10% forgiveness.

4. **The beautiful part:** We don't program this. We don't seed it. The GA discovers the optimal forgiveness rate through selection pressure. Different populations in different runs might converge on slightly different thresholds depending on the environment (food scarcity, population density, mutation rate).

### 16.6 The Full Social Genome

Combining trust-based cooperation with existing goal-based navigation, a "social NPC" genome looks like:

```asm
; Phase 1: Survival check
r0@ 1               ; health
10                  ; push 10
<                   ; health < 10?
jnz emergency_eat   ; critically low → eat NOW

; Phase 2: Social decision (tit-for-tat with forgiveness)
r0@ 17              ; trust with nearest NPC (0-62, midpoint 31)
28                  ; threshold (≈ forgiveness of -3)
>                   ; trust above threshold?
jnz cooperate       ; yes → approach and trade

; Phase 3: Default foraging
r0@ 13              ; food direction
r1! 0               ; move
1                   ; eat
r1! 1               ; action
yield

; Emergency eat
r0@ 13              ; food direction
r1! 0               ; move
1                   ; eat
r1! 1               ; action
yield

; Cooperate block
r0@ 18              ; nearest NPC direction
r1! 0               ; move toward them
4                   ; ActionTrade
r1! 1               ; action
r0@ 12              ; nearest NPC ID
r1! 2               ; target
yield
```

That's ~45 bytes. Three priorities: survive, socialize, forage. One evolvable parameter (the forgiveness threshold at byte offset 13). The rest is structure. Well within 128-byte genome, well within 200 gas.

### 16.7 Critical Review

**Praise:** This is the cleanest mapping from game theory to bytecode I've seen. One byte = one parameter = the entire forgiveness rate. The GA optimizes it. The result reproduces a Nobel-prize-winning finding in evolutionary game theory. And the NPC is 45 bytes.

The offset encoding for trust (0-62) is elegant — it avoids negative numbers entirely while preserving the full range. The brain treats trust as "just another sensor value" and compares it to "just another threshold." No special cases.

**Criticism:** This only works if the trust matrix is maintained by the framework. The brain doesn't *remember* interactions — it reads a pre-computed trust score. The "forgiveness" isn't really in the brain; it's in the threshold for trusting the framework's score. If trust decays naturally over time (which it should — familiarity fades), then forgiveness is partially a framework property, not purely a brain property.

Also: the 45-byte social genome has to be *seeded*. The GA won't discover "read trust, compare to threshold, branch to trade vs forage" from random bytecode. The structure is too specific. What the GA *will* do is take a seeded social genome and tune the threshold byte. So the claim "the GA discovers the optimal forgiveness rate" is true, but "the GA discovers tit-for-tat" is probably false — we seed the strategy, evolution refines the parameter.

**Honest framing:** We're giving NPCs the *capacity* for tit-for-tat with forgiveness. Evolution finds the *optimal forgiveness rate* for a given environment. That's still remarkable — it's evolutionary parameter optimization, not evolutionary strategy invention.

**One more thing:** Trust decay rate is itself a world parameter. If trust decays fast (lose 1 per tick), NPCs must constantly re-earn trust and forgiveness is expensive. If trust decays slowly (lose 1 per 50 ticks), relationships are sticky and forgiveness is cheap. The interaction between brain-level forgiveness threshold and world-level trust decay could produce very different social dynamics. Worth experimenting with.

---

## 17. Per-Brain RNG: Randomized Behavior

### 17.1 The Problem with Determinism

Currently, NPCs are fully deterministic: same Ring0 inputs → same Ring1 outputs, every tick. This means:
- A forager at the same distance from food does the same thing forever
- Tit-for-tat forgiveness requires a fixed threshold — either you forgive at trust=-3 or you don't
- Opponents can predict you perfectly (relevant when attack/deception evolves)
- No exploration — an NPC stuck in a local optimum stays there

Game theory says **mixed strategies** (randomized actions) beat pure strategies in many scenarios. The NPC needs a coin to flip.

### 17.2 Implementation: Ring0 Random Slot

Simplest approach — one new Ring0 slot, no new opcodes:

```
Ring0[rng] = 11   ; pseudo-random value 0-31, changes each tick
```

Per-NPC PRNG state stored in the NPC struct:

```go
type NPC struct {
    // ...existing fields...
    RngState [3]byte // tribonacci state
}

func (n *NPC) Rand() byte {
    // Tribonacci: s[n] = s[n-1] + s[n-2] + s[n-3]
    next := n.RngState[0] + n.RngState[1] + n.RngState[2]
    n.RngState[0] = n.RngState[1]
    n.RngState[1] = n.RngState[2]
    n.RngState[2] = next
    return next & 0x1F // mask to 0-31
}
```

Seeded from NPC ID + birth tick — every NPC gets a different sequence. 3 bytes of state. On Z80 this is ~8 bytes of assembly. The period is long enough (thousands of values before repeating) for our timescales.

### 17.3 What This Unlocks

**Probabilistic forgiveness** — replace the fixed threshold with a random comparison:

```asm
r0@ 17          ; trust with nearest NPC (0-62)
r0@ 11          ; random 0-31
>               ; trust > random?
jnz cooperate   ; probabilistic!
```

This is a *continuous probability curve*, not a step function:
- Trust = 62 (best friend): cooperates 100% of the time (62 > any 0-31)
- Trust = 31 (neutral): cooperates ~50% of the time
- Trust = 16 (distrusted): cooperates ~25% of the time
- Trust = 0 (enemy): cooperates ~0% of the time

No forgiveness parameter needed! The trust level itself IS the probability. And the comparison against a random value implements the mixed strategy in **4 bytes** of genome. The GA doesn't even need to tune a threshold — the math does it automatically.

**Exploration.** An NPC can break out of loops:

```asm
r0@ 11          ; random 0-31
3               ; push 3
<               ; random < 3? (~10% chance)
jnz do_random   ; yes → try something different
; else normal behavior...
```

10% of the time, do something unexpected. Walk in a random direction. Try to trade. Pick up a different item. This prevents getting stuck in local optima and enables discovery of new strategies.

**Unpredictability.** If combat evolves, a predictable NPC is a dead NPC. Random movement makes you harder to intercept. Random action selection makes you harder to exploit.

### 17.4 The Evolvable Part

The brain decides **how much** randomness to use. Compare:

```asm
; Conservative NPC (low randomness)
r0@ 11          ; random 0-31
2               ; push 2
<               ; random < 2? (~6% chance)
jnz random_act  ; rarely deviates

; Chaotic NPC (high randomness)
r0@ 11          ; random 0-31
15              ; push 15
<               ; random < 15? (~50% chance)
jnz random_act  ; often deviates
```

The comparison value (2 vs 15) is a SmallNum opcode — one evolvable byte controlling the randomness rate. The GA tunes this:
- In stable environments: low randomness wins (exploitation)
- In volatile environments: moderate randomness wins (exploration)
- In adversarial environments: unpredictability wins (defense)

### 17.5 Z80 Fit

The tribonacci PRNG on Z80:

```asm
; HL = pointer to NPC's 3-byte RNG state
npc_rand:
    ld a, (hl)      ; s[0]
    inc hl
    add a, (hl)     ; + s[1]
    inc hl
    add a, (hl)     ; + s[2]
    ; shift state: s[0]=s[1], s[1]=s[2], s[2]=next
    ld b, a          ; save next
    ld a, (hl)       ; s[2]
    dec hl
    ld (hl), a       ; s[1] = old s[2]
    dec hl
    ld a, (hl)       ; old s[0] — wait, already shifted
    ; ... (needs cleanup but ~12 bytes total)
    ld a, b
    and $1F          ; mask to 0-31
    ret
```

~12 bytes of Z80 code, 3 bytes of state per NPC. Trivial.

### 17.6 Critical Review

**Praise:** This is the highest value-to-cost ratio of anything in this report. One Ring0 slot, 3 bytes of NPC state, ~5 lines of Go code. And it unlocks mixed strategies, probabilistic forgiveness (Section 16 becomes simpler and more expressive), exploration behavior, and unpredictability. The `trust > random` pattern for probabilistic cooperation is particularly beautiful — zero design parameters, the math just works.

**Criticism:** Randomness can make behavior harder to debug and harder for the GA to evaluate. If an NPC cooperates 50% of the time due to randomness, its fitness has high variance. The GA might struggle to distinguish "good strategy with bad luck" from "bad strategy." Tournament selection (our current approach) partially handles this — averaging over 3 candidates smooths variance — but it's worth monitoring.

The tribonacci PRNG has biases (not cryptographically random, some values appear more often). For our purposes this doesn't matter — we need "unpredictable enough to break loops," not "uniformly random." But if NPCs evolve to exploit PRNG biases (e.g., knowing that value 7 appears more often), that's actually fascinating emergent behavior.

**One concern:** Slot 11 is currently `Ring0[day]` in the existing codebase. We'd need to either move day to another slot or use a different slot for RNG. Slot 25 (just past Ring0ExtCount=24) would work, or we bump Ring0ExtCount to 26.

---

*Status: Parked. 17 sections. Key insight: forgiveness strategies (hard/randomized/soft/unconditional) coexist in the population — the GA runs a live Axelrod tournament. Gas budget determines which strategies are affordable. Resume when Phase 2.0 (knowledge buffer) lands.*
