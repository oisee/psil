package sandbox

// Ring0 sensor slots (read-only, filled by world before brain runs)
const (
	Ring0Self   = 0  // own NPC ID
	Ring0Health = 1  // current health
	Ring0Energy = 2  // current energy
	Ring0Hunger = 3  // ticks since last ate
	Ring0Fear   = 4  // nearest enemy distance
	Ring0Food   = 5  // nearest food distance
	Ring0Danger = 6  // danger level around
	Ring0Near   = 7  // nearest NPC distance
	Ring0X      = 8  // own X position
	Ring0Y      = 9  // own Y position
	Ring0Day    = 10 // current tick mod cycle
	Ring0Count  = 11 // number of Ring0 slots
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
	ActionIdle   = 0
	ActionEat    = 1
	ActionAttack = 2
	ActionShare  = 3
)

// NPC represents a creature in the sandbox world.
type NPC struct {
	ID      byte
	X, Y    int
	Health  int
	Energy  int
	Age     int
	Genome  []byte
	Fitness int

	// Internal state
	Hunger   int // ticks since last ate
	FoodEaten int
}

// Alive returns true if NPC is still alive.
func (n *NPC) Alive() bool {
	return n.Health > 0
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
