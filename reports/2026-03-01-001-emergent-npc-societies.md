# Emergent NPC Societies: Trade, Knowledge, Memetics, and Deception on Concatenative Bytecode

**Date:** 2026-03-01
**Status:** Design / Research
**Depends on:** [NPC Sandbox](../docs/npc-sandbox-journey.md), [ZX Game Integration Guide](../docs/guide-zx-game-integration.md)

## Abstract

The existing NPC sandbox demonstrates genetic evolution of bytecode brains on both Go and Z80. This report proposes three additional information channels — trade, shared knowledge, and memetic transmission — that together produce emergent social structures (alliances, economies, deception, culture) from minimal rules. The entire system fits in ~13K on a 128K ZX Spectrum, or ~6K on 48K with 8 NPCs. Prior work in digital organisms (Avida), evolved robot communication (Floreano), and artificial societies (Sugarscape) confirms that these dynamics emerge reliably from simple evolutionary pressure.

## 1. Motivation

The current sandbox has one information channel: **genes**. NPC genomes are inherited vertically (parent to child via GA) and mutated. This produces individual behavioral adaptation — NPCs evolve to find food, flee danger, or wander efficiently.

But real societies run on at least three channels:

| Channel | Direction | What transfers | Biological analogy |
|---------|-----------|---------------|-------------------|
| **Genes** | Vertical (parent → child) | Behavioral program (genome) | DNA inheritance |
| **Memes** | Horizontal (peer → peer) | Behavioral fragments (bytecode) | Cultural transmission [1] |
| **Knowledge** | Horizontal (peer → peer) | World facts (data) | Language, gossip |

Adding memes and knowledge creates selection pressure for **social intelligence**: brains that can trade, share, deceive, and form alliances survive longer than brains that only forage and flee.

## 2. Prior Art

### 2.1 Evolved Communication

Knoester et al. [2] demonstrated in Avida that digital organisms with no built-in communication ability evolved to share sensed values across the population via message passing. The organisms developed cooperative information-gathering protocols from scratch. Yaeger's PolyWorld [3] showed that agents evolved to use body color as an emergent communication signal — both the signal and its interpretation were evolved, not designed.

### 2.2 Evolved Deception

Mitri, Floreano, and Keller [4] showed that physical robots with evolved neural controllers and light-emitting signals evolved to suppress their signals near food by generation 50 — active deception by omission. The companion paper [5] established the key condition: honest communication evolves under kin selection; deception evolves under individual competition. This maps directly to our trust-matrix design: high-trust pairs cooperate, low-trust pairs deceive.

### 2.3 Memetic Algorithms

Moscato [6] formalized memetic algorithms as genetic search combined with cultural transmission, drawing explicitly on Dawkins' meme concept [1]. The taxonomy by Ong and Keane [7] distinguishes "Lamarckian" memetic transfer (acquired traits transmitted directly) from "Baldwinian" (acquired traits only improve fitness, not transmitted). Our bytecode copying is pure Lamarckian — the most powerful and most biologically controversial form, but trivially natural in concatenative bytecode.

### 2.4 Horizontal Gene Transfer

Wiser et al. [8] demonstrated in Avida that horizontal gene transfer (code transferred between organisms outside reproduction) increases both task acquisition rate and genomic modularity. In our system, memetic transmission is exactly HGT: one NPC copies a bytecode fragment into another's genome. The concatenative property guarantees the result is always a valid program.

### 2.5 Artificial Societies

Epstein and Axtell's Sugarscape [9] remains the canonical demonstration that minimal agent rules produce emergent social complexity: wealth inequality, cultural groups, trade with price dynamics, disease, and warfare — all from a 51-rule agent on a grid. Our system has comparable rule simplicity (sense-think-act with Ring0/Ring1) but adds evolvable behavior, which Sugarscape's fixed rules lacked.

### 2.6 Dwarf Fortress

Adams [10, 11] describes DF's approach to emergent narrative: decompose systems into minimal components, avoid overplanning, and let complex behavior emerge from interactions. DF's NPCs have fixed behavioral scripts — the same limitation as The Hobbit and Valhalla. Our contribution is making those scripts evolvable while maintaining the emergent-from-simplicity philosophy.

## 3. Design

### 3.1 Extended NPC State

Current NPC table entry: 14 bytes. Extended:

```
Offset  Size  Field           Current  New
+0      1     ID              yes      yes
+1      1     X               yes      yes
+2      1     Y               yes      yes
+3      1     Health          yes      yes
+4      1     Energy          yes      yes
+5      2     Age             yes      yes
+7      1     Hunger          yes      yes
+8      1     Food eaten      yes      yes
+9      2     Fitness         yes      yes
+11     1     Genome length   yes      yes
+12     2     Genome pointer  yes      yes
--- new fields ---
+14     1     Gold            -        0-255
+15     1     Item            -        item type (0=none)
+16     1     Charisma        -        memetic transmission power
+17     1     Faction         -        emergent group ID
+18     1     Knowledge ptr   -        offset into knowledge bank
+19     1     Knowledge count -        items in knowledge buffer
```

Extended NPC size: **20 bytes** (was 14). 32 NPCs = 640 bytes.

### 3.2 Knowledge Buffer

Per-NPC ring buffer of knowledge items. Each item is 4 bytes:

```
Byte 0: type     (FOOD_AT=1, DANGER_AT=2, TREASURE_AT=3, NPC_IS=4, SECRET=5, RUMOR=6)
Byte 1: arg1     (x coordinate, or NPC ID)
Byte 2: arg2     (y coordinate, or trait/value)
Byte 3: confidence  (0-255, decays each tick, 0 = forgotten)
```

8 slots per NPC, 4 bytes each = 32 bytes per NPC. 32 NPCs = 1,024 bytes.

Confidence decay creates forgetting: knowledge that isn't refreshed (by revisiting the location or hearing it again) fades. This prevents stale information from dominating decisions and creates value for scouts who bring fresh intelligence.

### 3.3 Trust Matrix

A signed 8-bit value per NPC pair. Range: -128 (enemy) to +127 (trusted ally).

```
trust[A][B] += delta    when A receives accurate info from B
trust[A][B] -= delta    when A discovers B lied
trust[A][B] += delta    when B shares food with A
trust[A][B] -= delta    when B attacks A
```

32 × 32 = 1,024 bytes. Symmetric storage (only upper triangle) would halve this, but asymmetric trust is more interesting: A trusts B doesn't mean B trusts A.

### 3.4 Extended Ring0 (Sensors)

| Slot | Name | Meaning |
|------|------|---------|
| 0-10 | *(existing)* | self, health, energy, hunger, fear, food, danger, near, x, y, day |
| 11 | `known_food_dist` | Distance to nearest *known* food (from knowledge buffer) |
| 12 | `known_food_dir` | Direction to it (1=N, 2=E, 3=S, 4=W) |
| 13 | `known_danger_dist` | Distance to nearest *known* danger |
| 14 | `my_gold` | Own gold count |
| 15 | `my_item` | Own item type |
| 16 | `near_npc_item` | Nearest NPC's item type |
| 17 | `near_npc_trust` | Trust toward nearest NPC (-128..127, mapped to 0..255) |
| 18 | `last_told_type` | Type of last received knowledge |
| 19 | `last_told_by` | ID of NPC who told us something |
| 20 | `secret_count` | Number of knowledge items held |
| 21 | `faction` | Own faction ID |
| 22 | `near_npc_faction` | Nearest NPC's faction |
| 23 | `charisma` | Own charisma stat |

### 3.5 Extended Ring1 (Actions)

| Slot | Name | Values |
|------|------|--------|
| 0 | `move` | 0=none, 1=N, 2=E, 3=S, 4=W *(unchanged)* |
| 1 | `action` | 0=idle, 1=eat, 2=attack, 3=share, **4=trade, 5=tell, 6=ask, 7=lie, 8=teach** |
| 2 | `target` | Target NPC ID *(unchanged)* |
| 3 | `param` | Action parameter (item to trade, knowledge slot to share, etc.) |

New actions:

- **trade (4)**: Offer own item to target NPC. Both brains must output trade on the same tick for exchange to happen (bilateral consent).
- **tell (5)**: Share knowledge item `param` with nearest NPC. Copies knowledge item from own buffer to target's buffer.
- **ask (6)**: Request knowledge from nearest NPC about topic `param`. Target NPC's brain runs a "response check" — if it outputs tell on the next tick, information transfers.
- **lie (7)**: Transmit fabricated knowledge (random location) to nearest NPC as if it were true. Target cannot distinguish lie from tell unless they verify.
- **teach (8)**: Copy bytes `[param*4 .. param*4+3]` of own genome into target NPC's genome at a random position. Requires adjacency and target's charisma < own charisma (dominant ideas spread).

### 3.6 Memetic Transmission Mechanics

When NPC A "teaches" NPC B:

1. Extract 4-byte fragment from A's genome at offset `param * 4`
2. Pick random position in B's genome
3. Overwrite 4 bytes at that position with A's fragment
4. Probability of success = `A.charisma / (A.charisma + B.charisma)`
5. If B's brain contains a `yield` or `halt` before the insertion point, the prefix behavior is preserved

This creates:
- **Idea propagation**: successful patterns spread through populations
- **Resistance**: NPCs with high charisma are harder to overwrite
- **Cultural drift**: fragments mutate during transmission (point mutation applied to copied bytes with low probability)

### 3.7 Emergent Dynamics

The three channels interact to produce emergent social phenomena:

**Trade economies**: Brains that output `trade` when holding surplus items and needing other items will survive longer. Evolution discovers barter. NPCs near food sources evolve to farm; NPCs near other NPCs evolve to trade.

**Gossip networks**: A scout finds food → tells ally B → B tells ally C. Information propagates along trust edges. High-trust clusters form information pools.

**Misinformation cascades**: A liar NPC tells B "food at (5,5)" → B trusts and moves there → finds nothing → B's trust in liar decreases. But B already told C, who told D... The lie propagates faster than verification.

**Memetic cultures**: NPCs in proximity share bytecode fragments via teach. Over time, spatial clusters converge on similar brain patterns — "cultures." A cluster of NPCs that all have a "flee when health < 10" prefix behaves like a cautious tribe.

**Arms races**: As liars evolve, skeptics (brains that check trust before acting on knowledge) counter-evolve. As attackers evolve, alliance-formers (brains that share food with high-trust neighbors) counter-evolve.

**Emergent factions**: Trust + proximity + memetic similarity create proto-factions without anyone coding faction logic. The `faction` field is assigned by the scheduler based on trust clustering (connected components in the trust graph above a threshold).

## 4. Memory Budget

### 4.1 Full System (128K Spectrum, 32 NPCs, 64x64 world)

| Component | Bytes | Notes |
|-----------|-------|-------|
| VM core (library mode) | 1,500 | Existing |
| Scheduler (extended sense/think/act) | 800 | +trade, tell, lie, teach handlers |
| GA + memetic engine | 500 | Point mutation + HGT |
| Tick loop, init, helpers | 600 | Existing + knowledge decay |
| **Total code** | **3,400** | |
| NPC table (32 × 20) | 640 | Extended entries |
| Knowledge buffers (32 × 32) | 1,024 | 8 items × 4 bytes per NPC |
| Trust matrix (32 × 32) | 1,024 | Signed 8-bit per pair |
| World grid (64 × 64) | 4,096 | Terrain + items |
| Genome bank (32 × 64) | 2,048 | Banked RAM on 128K |
| VM runtime (stack + memory) | 448 | Existing |
| Item table | 128 | 16 item types × 8 bytes |
| **Total data** | **9,408** | |
| **Grand total** | **~12,800** | Leaves ~36K for game |

### 4.2 Minimal System (48K Spectrum, 8 NPCs, 32x32 world)

| Component | Bytes |
|-----------|-------|
| Code (VM + scheduler + GA) | 3,400 |
| NPC table (8 × 20) | 160 |
| Knowledge (8 × 32) | 256 |
| Trust matrix (8 × 8) | 64 |
| World grid (32 × 32) | 1,024 |
| Genomes (8 × 48) | 384 |
| VM runtime | 448 |
| **Total** | **~5,740** |

Leaves ~43K on a 48K Spectrum. Tight but viable.

### 4.3 128K Bank Layout

```
Bank 0 (main):  VM + scheduler + world grid + NPC table
Bank 1 ($C000): Genome bank (256 genomes × 64 bytes)
Bank 3 ($C000): Knowledge bank + trust matrix
Bank 5 ($C000): Extended game data (items, map overlay, dialogue)
```

## 5. Comparison with Existing Systems

| System | Genes | Memes | Knowledge | Deception | Hardware |
|--------|-------|-------|-----------|-----------|----------|
| Sugarscape [9] | No (fixed rules) | No | Limited (vision) | No | Desktop |
| Avida [2, 8] | Yes | Yes (HGT) | Yes (messages) | No | Desktop |
| Floreano robots [4, 5] | Yes (neural) | No | Yes (signals) | Yes (evolved) | Robots |
| PolyWorld [3] | Yes (neural) | No | Color signals | No | Desktop |
| Dwarf Fortress [10, 11] | No | No | Fixed scripts | No | Desktop |
| **micro-PSIL** | **Yes (bytecode GP)** | **Yes (Lamarckian HGT)** | **Yes (knowledge buffers)** | **Yes (lie action)** | **Z80 (3.5 MHz)** |

The unique contribution: all four channels on 8-bit hardware, enabled by the concatenative bytecode representation that makes every operation (mutation, crossover, HGT, composition) produce valid programs.

## 6. Open Questions

1. **Convergence speed**: How many ticks until interesting social structures emerge? Sugarscape shows structure within ~100 steps with 250 agents. With 32 NPCs and evolution every 128 ticks, we may need 5,000–10,000 ticks.

2. **Lie detection**: Should the scheduler provide any lie-detection mechanism (e.g., NPCs that visit a reported location and find nothing get a trust penalty on the informer), or should verification be entirely brain-driven?

3. **Teaching consent**: Should the target NPC be able to resist teaching? Current design uses charisma comparison. Alternative: target must also output a "learn" action (bilateral consent, like trade).

4. **Knowledge capacity**: 8 items per NPC may be too few for rich gossip networks. 16 items (64 bytes per NPC) doubles the knowledge bank but may be worth it.

5. **Faction computation**: Computing connected components in the trust graph every N ticks is O(N^2). With 32 NPCs this is ~1,024 comparisons — fast enough on Z80 but worth profiling.

## References

[1] R. Dawkins, *The Selfish Gene* (Oxford University Press, 1976). Chapter 11 introduces the concept of memes as cultural replicators.

[2] D.B. Knoester, P.K. McKinley, B. Beckmann, and C. Ofria, "Directed Evolution of Communication and Cooperation in Digital Organisms," *Advances in Artificial Life (ECAL)*, LNCS 4648, Springer, 2007.

[3] L. Yaeger, "Computational Genetics, Physiology, Metabolism, Neural Systems, Learning, Vision, and Behavior or PolyWorld: Life in a New Context," *Proceedings of Artificial Life III*, Addison-Wesley, 1994.

[4] S. Mitri, D. Floreano, and L. Keller, "The Evolution of Information Suppression in Communicating Robots with Conflicting Interests," *PNAS* 106(37), pp. 15786–15790, 2009.

[5] D. Floreano, S. Mitri, S. Magnenat, and L. Keller, "Evolutionary Conditions for the Emergence of Communication in Robots," *Current Biology* 17, pp. 514–519, 2007.

[6] P. Moscato, "On Evolution, Search, Optimization, Genetic Algorithms and Martial Arts: Towards Memetic Algorithms," Caltech Concurrent Computation Program Technical Report 826, 1989.

[7] Y.S. Ong and A.J. Keane, "A Taxonomy for the Operator Content of Memetic Algorithms," *IEEE Transactions on Evolutionary Computation* 9(5), 2005.

[8] M.J. Wiser, R. Canino-Koning, and C. Ofria, "Horizontal Gene Transfer Leads to Increased Task Acquisition and Genomic Modularity in Digital Organisms," *Proceedings of ALIFE 2019*, MIT Press.

[9] J.M. Epstein and R.L. Axtell, *Growing Artificial Societies: Social Science from the Bottom Up* (MIT Press / Brookings Institution Press, 1996).

[10] T. Adams, "Simulation Principles from Dwarf Fortress," in *Game AI Pro 2*, ed. S. Rabin, CRC Press, 2015.

[11] T. Adams, "Emergent Narrative in Dwarf Fortress," in *Procedural Storytelling in Game Design*, ed. T. Short and T. Adams, CRC Press, 2019.

[12] K. Sims, "Evolving Virtual Creatures," *SIGGRAPH '94 Proceedings*, 1994.

[13] B. Skyrms, *Evolution of the Social Contract* (Cambridge University Press, 1996).

[14] J.R. Koza, *Genetic Programming: On the Programming of Computers by Means of Natural Selection* (MIT Press, 1992).

[15] T. Perkis, "Stack-Based Genetic Programming," *Proceedings of the 1994 IEEE World Congress on Computational Intelligence*, 1994.

[16] M. von Thun, "Joy: Forth's Functional Cousin," 2001.

[17] E.S. Colizzi and P. Hogeweg, "Stigmergic Gene Transfer and Emergence of Universal Coding," *HFSP Journal*, 2010.
