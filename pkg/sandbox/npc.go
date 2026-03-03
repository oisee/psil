package sandbox

// MaxAge is the maximum age (in ticks) before an NPC dies of old age.
const MaxAge = 5000 // ~50 GA cycles at evolve-every-100

// Ring0 sensor slots (read-only, filled by world before brain runs)
const (
	Ring0Self   = 0  // own NPC ID
	Ring0Health = 1  // current health
	Ring0Energy = 2  // current energy
	Ring0Hunger = 3  // ticks since last ate
	Ring0Fear   = 4  // nearest enemy distance
	Ring0Food   = 5  // nearest food distance
	Ring0Danger = 6  // nearest poison distance
	Ring0Near   = 7  // nearest NPC distance
	Ring0X      = 8  // own X position
	Ring0Y      = 9  // own Y position
	Ring0Day       = 10 // current tick mod cycle
	Ring0Count     = 11 // number of original Ring0 slots
	Ring0NearID    = 12 // ID of nearest NPC
	Ring0FoodDir   = 13 // direction toward nearest food (1=N,2=E,3=S,4=W,0=none)
	Ring0MyGold    = 14 // NPC's gold count
	Ring0MyItem    = 15 // NPC's held item type
	Ring0NearItem  = 16 // distance to nearest item tile
	Ring0NearTrust = 17 // trust of nearest NPC (stub, Phase 3)
	Ring0NearDir   = 18 // direction toward nearest NPC
	Ring0ItemDir   = 19 // direction toward nearest item tile
	Ring0Rng       = 20 // per-NPC random number (0-31)
	Ring0Stress    = 21 // current stress level
	Ring0MyGas     = 22 // effective gas (base + modifier)
	Ring0OnForge   = 23 // 1 if standing on forge tile, 0 otherwise
	Ring0MyAge     = 24 // remaining life (MaxAge - Age)
	Ring0Taught    = 25 // number of times genome was modified by others
	Ring0Biome      = 26 // biome type at NPC position (0-6), 0 if biomes disabled
	Ring0TileType   = 27 // tile type under NPC (food, tool, forge, etc.)
	Ring0Similarity = 28 // genetic similarity to nearest NPC (0-100)
	Ring0TileAhead  = 29 // tile type in move direction
	Ring0Cooldown   = 30 // ticks remaining on current tile cooldown
	Ring0ExtCount   = 31 // extended Ring0 slot count
)

// Ring1 action slots (writable by brain, read by scheduler)
const (
	Ring1Move    = 0 // move direction (0=none, 1=N, 2=E, 3=S, 4=W)
	Ring1Action  = 1 // action (0=idle, 1=eat, 2=attack, 3=share)
	Ring1Target  = 2 // action target ID
	Ring1Emotion = 3 // emotional state
	Ring1Count   = 4 // number of Ring1 slots
)

// Move directions
const (
	DirNone  = 0
	DirNorth = 1
	DirEast  = 2
	DirSouth = 3
	DirWest  = 4
)

// Action types
const (
	ActionIdle      = 0
	ActionEat       = 1
	ActionAttack    = 2
	ActionShare     = 3
	ActionTrade     = 4
	ActionCraft     = 5
	ActionTeach     = 6
	ActionHeal      = 7
	ActionHarvest   = 8
	ActionTerraform = 9
)

// Item types
const (
	ItemNone     = 0
	ItemFoodPack = 1
	ItemTool     = 2
	ItemWeapon   = 3
	ItemTreasure = 4
	ItemCrystal  = 5
	ItemShield   = 6
	ItemCompass  = 7
)

// Modifier kinds
const (
	ModNone    = 0
	ModGas     = 1
	ModForage  = 2
	ModAttack  = 3
	ModDefense = 4
	ModEnergy  = 5
	ModHealth  = 6
	ModStealth = 7
	ModTrade   = 8
	ModStress  = 9
)

// Modifier represents a timed or permanent effect on an NPC.
type Modifier struct {
	Kind     byte  // ModGas, ModForage, etc.
	Mag      int8  // magnitude
	Duration int16 // ticks remaining; -1 = permanent, 0 = expired
	Source   byte  // item type, tile type, or 0 = innate
}

// ItemModifiers maps item types to the modifier they grant when held.
var ItemModifiers = map[byte]Modifier{
	ItemTool:     {Kind: ModForage, Mag: 1, Duration: -1, Source: ItemTool},
	ItemWeapon:   {Kind: ModAttack, Mag: 10, Duration: -1, Source: ItemWeapon},
	ItemTreasure: {Kind: ModTrade, Mag: 3, Duration: -1, Source: ItemTreasure},
	ItemShield:   {Kind: ModDefense, Mag: 5, Duration: -1, Source: ItemShield},
	ItemCompass:  {Kind: ModForage, Mag: 2, Duration: -1, Source: ItemCompass},
}

// NPC represents a creature in the sandbox world.
type NPC struct {
	ID      uint16
	X, Y    int
	Health  int
	Energy  int
	Age     int
	Genome  []byte
	Fitness int

	// Internal state
	Hunger     int          // ticks since last ate
	FoodEaten  int
	Gold       int          // currency
	Item       byte         // held item (0=none, 1=food-pack, 2=tool, 3=weapon, 4=treasure)
	RngState   [3]byte      // tribonacci PRNG state
	Mods       [4]Modifier  // active modifiers (fixed-size, no heap)
	Stress     int          // stress level (0-100)
	CraftCount int          // number of items crafted
	Taught     int          // times this NPC's genome was externally modified
	TeachCount int          // times this NPC successfully taught others
	LastDir    byte         // last move direction (for tile-ahead sensor)
}

// Alive returns true if NPC is still alive.
func (n *NPC) Alive() bool {
	return n.Health > 0
}

// Rand returns a pseudo-random number in 0-31 using tribonacci PRNG.
func (n *NPC) Rand() byte {
	next := n.RngState[0] + n.RngState[1] + n.RngState[2]
	n.RngState[0] = n.RngState[1]
	n.RngState[1] = n.RngState[2]
	n.RngState[2] = next
	return next & 0x1F
}

// GenomeSimilarity returns 0-100 indicating how similar two genomes are.
func GenomeSimilarity(a, b []byte) int {
	if len(a) == 0 && len(b) == 0 {
		return 100
	}
	maxLen := len(a)
	if len(b) > maxLen {
		maxLen = len(b)
	}
	if maxLen == 0 {
		return 0
	}
	matches := 0
	minLen := len(a)
	if len(b) < minLen {
		minLen = len(b)
	}
	for i := 0; i < minLen; i++ {
		if a[i] == b[i] {
			matches++
		}
	}
	return matches * 100 / maxLen
}

// ModSum returns the total magnitude of all active modifiers of the given kind.
func (n *NPC) ModSum(kind byte) int {
	sum := 0
	for _, m := range n.Mods {
		if m.Kind == kind && m.Duration != 0 {
			sum += int(m.Mag)
		}
	}
	return sum
}

// AddMod adds a modifier, evicting the shortest-duration entry if full.
func (n *NPC) AddMod(m Modifier) {
	// Find an empty slot (Duration == 0)
	for i := range n.Mods {
		if n.Mods[i].Duration == 0 {
			n.Mods[i] = m
			return
		}
	}
	// Evict shortest-duration (non-permanent) entry
	evict := -1
	shortest := int16(32767)
	for i := range n.Mods {
		d := n.Mods[i].Duration
		if d == -1 {
			continue // don't evict permanent
		}
		if d < shortest {
			shortest = d
			evict = i
		}
	}
	if evict == -1 {
		// All permanent; evict first slot
		evict = 0
	}
	n.Mods[evict] = m
}

// RemoveMod removes all modifiers with the given source.
func (n *NPC) RemoveMod(source byte) {
	for i := range n.Mods {
		if n.Mods[i].Source == source {
			n.Mods[i] = Modifier{}
		}
	}
}

// NewNPC creates an NPC with default stats and the given genome.
func NewNPC(genome []byte) *NPC {
	g := make([]byte, len(genome))
	copy(g, genome)
	return &NPC{
		Health: 100,
		Energy: 100,
		Genome: g,
	}
}
