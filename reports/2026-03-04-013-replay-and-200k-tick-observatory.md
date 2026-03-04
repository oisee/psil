# Replay Observatory: Watching 200,000 Ticks of Evolution Unfold

**Report 013** | 2026-03-04 | psil sandbox

---

## New Capability: Simulation Recording & Replay

We now record full simulation state to JSONL files and replay them in a colored terminal UI with biome backgrounds, NPC markers, and real-time stats. The replay player supports pause, frame-step, and variable speed. A new side-panel legend makes every symbol immediately readable.

We also added **genome injection** — load a hand-crafted hex genome file and spawn copies into a running simulation at any tick, to observe how custom organisms interact with the evolved population.

This report analyzes the `warrior999` recording: 200 NPCs on a 56x56 biome map, seed 999, 200,000 ticks.

---

## The Recording: warrior999.jsonl

| Parameter | Value |
|-----------|-------|
| Seed | 999 |
| NPCs (initial) | 200 |
| World | 56x56 with WFC biomes |
| Ticks | 200,000 |
| Sample rate | every 100 ticks |
| Frames | 2,000 |
| File size | ~614 KB |

---

## Act I: The Culling (tick 0 - 10,000)

The population drops from 200 to 99 in the first 10,000 ticks — a 50% die-off. This is the harshest phase of the simulation.

```
tick      0: alive=200  avg_fit=110      items=80   trades=0
tick  10000: alive= 99  avg_fit=33,962   items=60   trades=940
```

Nearly all trading happens here — 940 of the final 988 trades occur in the first epoch. The initial item-holders (36 tools, 25 treasures, 16 weapons) scramble to exchange before half the population is gone. Teaching also spikes: 244 events in this window alone.

Attacks begin immediately: 419 attacks, 3 kills. Violence is common but rarely lethal.

**What the replay shows:** A chaotic opening. NPCs scatter across biomes, clusters form and dissolve, yellow item-holders dart between white naked NPCs. By frame 100, the map is visibly sparser.

---

## Act II: The Terraform Explosion (tick 10,000 - 50,000)

With population stable at ~99, survivors begin reshaping the world. Terraforming accelerates exponentially:

```
tick  10,000:      726,887 terraforms
tick  20,000:    1,642,192
tick  30,000:    3,115,737
tick  40,000:    7,056,484
tick  50,000:   12,236,441
```

By tick 50,000, NPCs have terraformed **12.2 million** times. On a 3,136-tile grid, that's ~3,900 terraforms per tile — the world is being rewritten constantly.

Average genome length grows from 24 to 124 bytes, nearly hitting the 128-byte cap. Evolution is packing in more complex programs.

```
tick      0: avg_genome=24 bytes
tick  25000: avg_genome=47 bytes
tick  50000: avg_genome=124 bytes  (genome cap = 128)
```

**What the replay shows:** Food tiles (`f`) flicker constantly — NPCs plant food, eat it, plant again. The map becomes a garden. Green food dots pulse across every biome.

---

## Act III: The Compass Convergence (tick 10,000 - 40,000)

A remarkable emergent behavior: the population converges on compass crafting.

```
tick      0: compass= 2/80 items  (2.5%)
tick  10000: compass=33/60 items  (55%)
tick  30000: compass=56/73 items  (77%)
tick 199900: compass=46/61 items  (75%)
```

Starting from just 2 compass holders, the population self-organizes so that **~75% of all item holders carry compasses** by mid-run, and this ratio holds for 170,000 ticks. Weapons nearly vanish (16 at start, 0-2 by endgame). Shields disappear entirely.

Evolution discovered that compass > shield > weapon > tool for long-term fitness. The compass provides navigation advantage; combined with the terraform-farming loop, it creates a dominant strategy.

**What the replay shows:** Yellow compass-carrying NPCs (`c`) dominate the map. The occasional tool (`t`) or treasure (`$`) is visible but rare. Weapon holders (`w`) are nearly extinct.

---

## Act IV: Stability and the Occasional War (tick 50,000 - 200,000)

The society enters a 150,000-tick steady state:

```
Population:   94-100 (remarkably stable)
Avg fitness:  ~218,000
Best fitness: ~550,000
Avg stress:   0-4 (very low)
Total kills:  6 (in 200,000 ticks!)
```

But it's not perfectly peaceful. Attack bursts punctuate the calm:

```
tick 160,000-167,000: +2,037 attacks (the "Late War")
tick 170,000:         attacks plateau, peace resumes
```

The "Late War" around tick 165,000 produces ~100 attacks per 100 ticks — a 10x spike over baseline. Yet only 0 additional kills occur during this burst. The NPCs fight but evolved genomes heal faster than they take damage.

Trading essentially stops after tick 10,000 (only 48 more trades in the remaining 190,000 ticks). Teaching continues at a declining rate: ~80/epoch early, ~30-60/epoch late. The knowledge economy outlasts the trade economy.

---

## Fitness Landscape at tick 200,000

```
Best:    551,480
Top 5:   551,480  551,450  538,140  520,720  514,640
Median:  183,020
Worst:       165
```

A 3,300x gap between best and worst. The worst NPC (fitness=165) is likely a fresh refill from the archetype pool, while the elders have been accumulating fitness for 4,900+ ticks of age.

### Age Distribution (Final)

```
Oldest:   4,900 ticks
Median:   1,700 ticks
Youngest:   100 ticks
```

The oldest NPCs have survived **every** evolution round (one every 200 ticks = 24.5 rounds). They're the immortal patriarchs of the colony.

### Final Genome Distribution

```
Range:  20 - 128 bytes
Average: 123 bytes
90 of 96 NPCs: 120-128 bytes
```

93.75% of genomes are at or near the 128-byte cap. Genome bloat is real — evolution fills every available byte. Only 6 NPCs have shorter genomes (likely recent refills).

---

## The Replay Experience

Running `go run ./tools/replay warrior999.jsonl`:

The new side-panel legend shows all biome colors, NPC symbols, and keyboard controls alongside the 56-column map. No more guessing what the symbols mean.

Key moments visible in replay:

- **Frame 1-100** (tick 0-10,000): Population collapse is visceral — NPCs blink out
- **Frame 200-500** (tick 20,000-50,000): The food-terraform engine kicks in, map pulses green
- **Frame 500+** (tick 50,000+): Serene garden society, occasional red dying NPCs quickly heal
- **Frame 1650-1670** (tick 165,000-167,000): Attack burst visible as brief cluster of red markers

The `+`/`-` keys let you speed through boring stretches and slow down for dramatic moments. Arrow keys for frame-by-frame analysis.

---

## Genome Injection: A New Experimental Tool

With `--inject`, we can now introduce foreign genomes into established populations:

```sh
# Extract a genome from a previous run's "Best genome:" output
echo "8a0d8c002181018c01f1" > forager.hex

# Inject 20 copies at tick 50,000 into a running sim
go run ./cmd/sandbox --npcs 200 --ticks 100000 --biomes \
    --inject forager.hex --inject-count 20 --inject-at 50000
```

This enables experiments like:
- **Invasion scenarios**: Drop fighters into a peaceful farming society
- **Cross-run transfer**: Take the best evolved genome from one seed, inject into another
- **Archetype stress tests**: See if a hand-crafted specialist survives in an evolved population
- **Dose-response**: Vary `--inject-count` to see the tipping point where invaders reshape the society

Combined with `--record`, every injection experiment is replayable.

---

## Summary Statistics

| Metric | Start | End | Note |
|--------|-------|-----|------|
| Population | 200 | 96 | Stable after initial culling |
| Avg fitness | 110 | 218,260 | 1,984x improvement |
| Best fitness | 451 | 551,480 | 1,223x improvement |
| Trades | 0 | 988 | 95% in first 10k ticks |
| Teaches | 0 | 1,233 | Steady throughout |
| Attacks | 0 | 5,976 | Burst pattern, not constant |
| Kills | 0 | 6 | 0.1% lethality rate |
| Heals | 0 | 2,892 | Outnumber kills 482:1 |
| Terraforms | 0 | 147,681,939 | 47,106 per tile, 738/tick |
| Compass holders | 2 | 46 | 75% of all item holders |
| Avg genome | 24 B | 123 B | 5.1x growth, hit cap |

---

## What's Next

- **Injection experiments**: Systematic injection of fighter genomes into peaceful colonies
- **Cross-seed genome transfer**: Do champion genomes from seed 999 dominate in seed 1337?
- **Longer runs**: 500k+ ticks to see if the compass monoculture ever breaks
- **Replay annotations**: Mark injection events and evolution rounds in the timeline
