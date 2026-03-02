package sandbox

import (
	"math/bits"
	"math/rand"
)

// Biome types (fits in 3 bits, uses uint8 masks for 8-bit compat)
const (
	BiomeClearing byte = iota // 0: high food, no items
	BiomeForest               // 1: medium food, tools spawn
	BiomeMountain             // 2: low food, crystals/weapons
	BiomeSwamp                // 3: very low food, poison spawns
	BiomeVillage              // 4: medium food, all items, NPC-dense
	BiomeRiver                // 5: impassable barrier
	BiomeBridge               // 6: passable chokepoint over river
	NumBiomes                 // 7: count sentinel
)

// BiomeProps describes spawn and passability properties per biome.
type BiomeProps struct {
	FoodRate  float64 // probability of food spawn per tick in this biome
	ItemRate  float64 // probability of item spawn per tick
	ItemTypes []byte  // which item types can spawn here
	Poison    float64 // probability of poison spawn (instead of item)
	Passable  bool    // can NPCs walk here?
	ForgeRate float64 // probability of forge placement
}

// BiomeTable holds properties for each biome type.
var BiomeTable = [NumBiomes]BiomeProps{
	BiomeClearing: {FoodRate: 0.6, ItemRate: 0.0, ItemTypes: nil, Poison: 0.0, Passable: true, ForgeRate: 0.0},
	BiomeForest:   {FoodRate: 0.3, ItemRate: 0.05, ItemTypes: []byte{TileTool}, Poison: 0.02, Passable: true, ForgeRate: 0.0},
	BiomeMountain: {FoodRate: 0.1, ItemRate: 0.08, ItemTypes: []byte{TileCrystal, TileWeapon}, Poison: 0.0, Passable: true, ForgeRate: 0.15},
	BiomeSwamp:    {FoodRate: 0.05, ItemRate: 0.02, ItemTypes: []byte{TileTreasure}, Poison: 0.10, Passable: true, ForgeRate: 0.0},
	BiomeVillage:  {FoodRate: 0.3, ItemRate: 0.10, ItemTypes: []byte{TileTool, TileWeapon, TileTreasure, TileCrystal}, Poison: 0.0, Passable: true, ForgeRate: 0.20},
	BiomeRiver:    {FoodRate: 0.0, ItemRate: 0.0, ItemTypes: nil, Poison: 0.0, Passable: false, ForgeRate: 0.0},
	BiomeBridge:   {FoodRate: 0.1, ItemRate: 0.0, ItemTypes: nil, Poison: 0.0, Passable: true, ForgeRate: 0.0},
}

// BiomeChar returns a single-character representation of a biome type.
func BiomeChar(b byte) byte {
	switch b {
	case BiomeClearing:
		return '.'
	case BiomeForest:
		return 'T'
	case BiomeMountain:
		return '^'
	case BiomeSwamp:
		return '~'
	case BiomeVillage:
		return 'H'
	case BiomeRiver:
		return '='
	case BiomeBridge:
		return '#'
	default:
		return '?'
	}
}

// WFC directions (N, E, S, W)
const (
	dirN = 0
	dirE = 1
	dirS = 2
	dirW = 3
)

// opposite returns the opposite direction.
func opposite(dir int) int {
	return (dir + 2) % 4
}

// WFCCell represents one cell in the WFC grid.
type WFCCell struct {
	Possibilities byte // bitmask of possible biomes (bits 0-6)
	CollapsedTo   int8 // -1 = uncollapsed, 0-6 = biome type
}

// WFC implements a Wave Function Collapse biome generator.
type WFC struct {
	Width, Height int
	Grid          []WFCCell          // [height * width]
	Constraints   [4][NumBiomes]byte // [direction][biome] → allowed neighbor mask
	propStack     [][2]int           // (x, y) pairs for propagation
	Rng           *rand.Rand
}

// allPossibilities is the bitmask with all 7 biome types set.
const allPossibilities byte = (1 << NumBiomes) - 1

// NewWFC creates a WFC grid with all cells having all possibilities.
func NewWFC(w, h int, rng *rand.Rand) *WFC {
	wfc := &WFC{
		Width:  w,
		Height: h,
		Grid:   make([]WFCCell, w*h),
		Rng:    rng,
	}
	for i := range wfc.Grid {
		wfc.Grid[i] = WFCCell{
			Possibilities: allPossibilities,
			CollapsedTo:   -1,
		}
	}
	wfc.setConstraints()
	return wfc
}

// setConstraints loads the biome adjacency table.
// Each entry: Constraints[dir][biome] = bitmask of biomes allowed in that direction.
// Constraints are symmetric: if A can be north of B, then B can be south of A.
func (wfc *WFC) setConstraints() {
	// Helper to set symmetric constraint
	set := func(a, b byte) {
		for d := 0; d < 4; d++ {
			wfc.Constraints[d][a] |= 1 << b
			wfc.Constraints[d][b] |= 1 << a
		}
	}

	// Self-adjacency (all biomes can be next to themselves)
	for b := byte(0); b < NumBiomes; b++ {
		set(b, b)
	}

	// Clearing: Forest, Village, River, Bridge
	set(BiomeClearing, BiomeForest)
	set(BiomeClearing, BiomeVillage)
	set(BiomeClearing, BiomeRiver)
	set(BiomeClearing, BiomeBridge)

	// Forest: Mountain, Swamp, River
	set(BiomeForest, BiomeMountain)
	set(BiomeForest, BiomeSwamp)
	set(BiomeForest, BiomeRiver)
	set(BiomeForest, BiomeBridge)

	// Mountain: Village (rare — allowed for adjacency, WFC handles probability)
	set(BiomeMountain, BiomeVillage)

	// Swamp: River
	set(BiomeSwamp, BiomeRiver)

	// Village: Bridge
	set(BiomeVillage, BiomeBridge)

	// River: Bridge (already set via Clearing and Forest)
	// Bridge already connected to River, Clearing, Village, Forest above
}

// idx converts (x,y) to flat index.
func (wfc *WFC) idx(x, y int) int {
	return y*wfc.Width + x
}

// inBounds checks if coordinates are valid.
func (wfc *WFC) inBounds(x, y int) bool {
	return x >= 0 && x < wfc.Width && y >= 0 && y < wfc.Height
}

// neighborCoords returns the (x,y) of the neighbor in the given direction.
func neighborCoords(x, y, dir int) (int, int) {
	switch dir {
	case dirN:
		return x, y - 1
	case dirE:
		return x + 1, y
	case dirS:
		return x, y + 1
	case dirW:
		return x - 1, y
	}
	return x, y
}

// Collapse forces a cell to a specific biome.
func (wfc *WFC) Collapse(x, y int, biome byte) bool {
	i := wfc.idx(x, y)
	cell := &wfc.Grid[i]
	if cell.CollapsedTo >= 0 {
		return cell.CollapsedTo == int8(biome)
	}
	if cell.Possibilities&(1<<biome) == 0 {
		return false // not possible
	}
	cell.Possibilities = 1 << biome
	cell.CollapsedTo = int8(biome)
	wfc.propStack = append(wfc.propStack, [2]int{x, y})
	return true
}

// CollapseRandom collapses a cell to a random valid biome from its possibilities.
func (wfc *WFC) CollapseRandom(x, y int) bool {
	i := wfc.idx(x, y)
	cell := &wfc.Grid[i]
	if cell.CollapsedTo >= 0 {
		return true
	}
	poss := cell.Possibilities
	count := bits.OnesCount8(poss)
	if count == 0 {
		return false // contradiction
	}
	// Pick random one
	pick := wfc.Rng.Intn(count)
	for b := byte(0); b < NumBiomes; b++ {
		if poss&(1<<b) != 0 {
			if pick == 0 {
				return wfc.Collapse(x, y, b)
			}
			pick--
		}
	}
	return false
}

// getAllowed computes the union of allowed neighbors for a cell's possibilities in a direction.
func (wfc *WFC) getAllowed(poss byte, dir int) byte {
	allowed := byte(0)
	for b := byte(0); b < NumBiomes; b++ {
		if poss&(1<<b) != 0 {
			allowed |= wfc.Constraints[dir][b]
		}
	}
	return allowed
}

// Propagate runs constraint propagation from the stack.
// Returns false if a contradiction is found.
func (wfc *WFC) Propagate() bool {
	for len(wfc.propStack) > 0 {
		top := wfc.propStack[len(wfc.propStack)-1]
		wfc.propStack = wfc.propStack[:len(wfc.propStack)-1]
		cx, cy := top[0], top[1]
		cell := &wfc.Grid[wfc.idx(cx, cy)]

		for d := 0; d < 4; d++ {
			nx, ny := neighborCoords(cx, cy, d)
			if !wfc.inBounds(nx, ny) {
				continue
			}
			ni := wfc.idx(nx, ny)
			neighbor := &wfc.Grid[ni]
			if neighbor.CollapsedTo >= 0 {
				continue // already collapsed, skip
			}

			allowed := wfc.getAllowed(cell.Possibilities, d)
			newPoss := neighbor.Possibilities & allowed
			if newPoss == 0 {
				return false // contradiction
			}
			if newPoss != neighbor.Possibilities {
				neighbor.Possibilities = newPoss
				// Auto-collapse if single possibility
				if bits.OnesCount8(newPoss) == 1 {
					for b := byte(0); b < NumBiomes; b++ {
						if newPoss&(1<<b) != 0 {
							neighbor.CollapsedTo = int8(b)
							break
						}
					}
				}
				wfc.propStack = append(wfc.propStack, [2]int{nx, ny})
			}
		}
	}
	return true
}

// FindMinEntropy finds the uncollapsed cell with the fewest possibilities.
// Returns (x, y, found). found=false means all cells are collapsed.
func (wfc *WFC) FindMinEntropy() (int, int, bool) {
	minCount := 100
	bestX, bestY := -1, -1
	// Add randomness to break ties
	tieCount := 0
	for y := 0; y < wfc.Height; y++ {
		for x := 0; x < wfc.Width; x++ {
			cell := &wfc.Grid[wfc.idx(x, y)]
			if cell.CollapsedTo >= 0 {
				continue
			}
			count := bits.OnesCount8(cell.Possibilities)
			if count == 0 {
				return x, y, true // contradiction cell, caller will handle
			}
			if count < minCount {
				minCount = count
				bestX, bestY = x, y
				tieCount = 1
			} else if count == minCount {
				tieCount++
				// Reservoir sampling for tie-breaking
				if wfc.Rng.Intn(tieCount) == 0 {
					bestX, bestY = x, y
				}
			}
		}
	}
	if bestX < 0 {
		return 0, 0, false // all collapsed
	}
	return bestX, bestY, true
}

// Generate runs the WFC algorithm to completion.
// Returns true if generation succeeded without contradiction.
func (wfc *WFC) Generate(maxIter int) bool {
	for iter := 0; iter < maxIter; iter++ {
		x, y, found := wfc.FindMinEntropy()
		if !found {
			return true // all collapsed
		}
		cell := &wfc.Grid[wfc.idx(x, y)]
		if bits.OnesCount8(cell.Possibilities) == 0 {
			return false // contradiction
		}
		if !wfc.CollapseRandom(x, y) {
			return false
		}
		if !wfc.Propagate() {
			return false
		}
	}
	// Check if all cells collapsed
	for _, c := range wfc.Grid {
		if c.CollapsedTo < 0 {
			return false
		}
	}
	return true
}

// ToBiomeGrid extracts a flat biome array from the collapsed WFC grid.
func (wfc *WFC) ToBiomeGrid() []byte {
	grid := make([]byte, len(wfc.Grid))
	for i, c := range wfc.Grid {
		if c.CollapsedTo >= 0 {
			grid[i] = byte(c.CollapsedTo)
		} else {
			grid[i] = BiomeClearing // fallback
		}
	}
	return grid
}

// Anchor represents a pre-placed biome at a specific location.
type Anchor struct {
	X, Y  int
	Biome byte
	Name  string
}

// PlaceAnchors collapses anchor positions before running WFC generation.
func (wfc *WFC) PlaceAnchors(anchors []Anchor) bool {
	for _, a := range anchors {
		if !wfc.inBounds(a.X, a.Y) {
			continue
		}
		if !wfc.Collapse(a.X, a.Y, a.Biome) {
			return false
		}
		if !wfc.Propagate() {
			return false
		}
	}
	return true
}

// CheckReachability verifies that all passable cells form a single connected component.
// Uses BFS from the first passable cell.
func (wfc *WFC) CheckReachability() bool {
	biomes := wfc.ToBiomeGrid()
	w, h := wfc.Width, wfc.Height

	// Find first passable cell
	startIdx := -1
	passableCount := 0
	for i, b := range biomes {
		if BiomeTable[b].Passable {
			passableCount++
			if startIdx < 0 {
				startIdx = i
			}
		}
	}
	if passableCount == 0 {
		return true // trivially connected
	}

	// BFS
	visited := make([]bool, len(biomes))
	visited[startIdx] = true
	queue := []int{startIdx}
	reachable := 1

	for len(queue) > 0 {
		cur := queue[0]
		queue = queue[1:]
		cx := cur % w
		cy := cur / w
		for _, d := range [][2]int{{0, -1}, {1, 0}, {0, 1}, {-1, 0}} {
			nx, ny := cx+d[0], cy+d[1]
			if nx < 0 || nx >= w || ny < 0 || ny >= h {
				continue
			}
			ni := ny*w + nx
			if visited[ni] {
				continue
			}
			if !BiomeTable[biomes[ni]].Passable {
				continue
			}
			visited[ni] = true
			reachable++
			queue = append(queue, ni)
		}
	}

	return reachable == passableCount
}

// DefaultAnchors generates spread anchor positions for the sandbox.
func DefaultAnchors(w, h int, rng *rand.Rand) []Anchor {
	var anchors []Anchor

	// Divide grid into sectors and place anchors with jitter
	jitter := func(pos, size int) int {
		j := pos + rng.Intn(3) - 1
		if j < 0 {
			j = 0
		}
		if j >= size {
			j = size - 1
		}
		return j
	}

	// 2-3 Village anchors spread across map
	// Place villages at roughly 1/4, 1/2, 3/4 positions
	villagePositions := [][2]int{
		{w / 4, h / 4},
		{w * 3 / 4, h * 3 / 4},
	}
	if w >= 16 {
		villagePositions = append(villagePositions, [2]int{w * 3 / 4, h / 4})
	}
	for _, vp := range villagePositions {
		anchors = append(anchors, Anchor{
			X: jitter(vp[0], w), Y: jitter(vp[1], h),
			Biome: BiomeVillage, Name: "village",
		})
	}

	// 1-2 Mountain clusters
	mtPositions := [][2]int{
		{w / 4, h * 3 / 4},
	}
	if w >= 16 {
		mtPositions = append(mtPositions, [2]int{w / 2, h / 6})
	}
	for _, mp := range mtPositions {
		anchors = append(anchors, Anchor{
			X: jitter(mp[0], w), Y: jitter(mp[1], h),
			Biome: BiomeMountain, Name: "mountain",
		})
	}

	// 1 River anchor near center to seed a barrier
	anchors = append(anchors, Anchor{
		X: jitter(w/2, w), Y: jitter(h/2, h),
		Biome: BiomeRiver, Name: "river",
	})

	return anchors
}

// GenerateBiomeGrid creates a biome grid using WFC with retries.
// wfcW, wfcH are the WFC grid dimensions (typically worldSize/2).
// Returns the WFC-resolution grid and success flag.
func GenerateBiomeGrid(wfcW, wfcH int, rng *rand.Rand, maxRetries int) ([]byte, bool) {
	for attempt := 0; attempt < maxRetries; attempt++ {
		wfc := NewWFC(wfcW, wfcH, rng)
		anchors := DefaultAnchors(wfcW, wfcH, rng)

		if !wfc.PlaceAnchors(anchors) {
			continue
		}
		if !wfc.Generate(wfcW * wfcH * 10) {
			continue
		}
		if !wfc.CheckReachability() {
			continue
		}
		return wfc.ToBiomeGrid(), true
	}
	return nil, false
}

// ExpandBiomeGrid scales a WFC-resolution biome grid to world resolution.
// Each WFC cell becomes a scale×scale block of world tiles.
func ExpandBiomeGrid(wfcGrid []byte, wfcW, wfcH, scale int) []byte {
	worldW := wfcW * scale
	worldH := wfcH * scale
	grid := make([]byte, worldW*worldH)
	for wy := 0; wy < wfcH; wy++ {
		for wx := 0; wx < wfcW; wx++ {
			biome := wfcGrid[wy*wfcW+wx]
			for dy := 0; dy < scale; dy++ {
				for dx := 0; dx < scale; dx++ {
					gx := wx*scale + dx
					gy := wy*scale + dy
					grid[gy*worldW+gx] = biome
				}
			}
		}
	}
	return grid
}
