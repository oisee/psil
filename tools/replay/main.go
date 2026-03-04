package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

// Frame types mirroring pkg/sandbox/recorder.go (minimal for playback).

type recordHeader struct {
	Type      string `json:"type"`
	Seed      int64  `json:"seed"`
	NPCs      int    `json:"npcs"`
	WorldSize int    `json:"world_size"`
	Ticks     int    `json:"ticks"`
	EveryN    int    `json:"every_n"`
	Biomes    bool   `json:"biomes"`
	BiomeGrid []byte `json:"biome_grid"`
}

type recordNPC struct {
	ID     uint16 `json:"id"`
	X      int    `json:"x"`
	Y      int    `json:"y"`
	HP     int    `json:"hp"`
	Energy int    `json:"e"`
	Item   byte   `json:"it"`
	Gold   int    `json:"g"`
	Age    int    `json:"a"`
	Fit    int    `json:"f"`
	Stress int    `json:"s"`
	GenLen int    `json:"gl"`
}

type recordStats struct {
	Attacks    int `json:"atk"`
	Kills      int `json:"kil"`
	Heals      int `json:"hea"`
	Harvests   int `json:"har"`
	Terraforms int `json:"ter"`
	Trades     int `json:"trd"`
	Teaches    int `json:"tch"`
}

type frame struct {
	Type     string      `json:"type"`
	Tick     int         `json:"t"`
	NPCs     []recordNPC `json:"npcs"`
	Grid     []byte      `json:"grid"`
	Stats    recordStats `json:"s"`
	FoodRate float64     `json:"fr"`
}

func main() {
	speed := flag.Int("speed", 10, "playback speed (frames per second)")
	start := flag.Int("start", 0, "starting frame index")
	flag.Parse()

	if flag.NArg() < 1 {
		fmt.Fprintf(os.Stderr, "Usage: replay <file.jsonl> [--speed N] [--start N]\n")
		os.Exit(1)
	}

	f, err := os.Open(flag.Arg(0))
	if err != nil {
		fmt.Fprintf(os.Stderr, "open: %v\n", err)
		os.Exit(1)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 4*1024*1024), 4*1024*1024) // 4MB line buffer

	// Read header
	if !scanner.Scan() {
		fmt.Fprintf(os.Stderr, "empty file\n")
		os.Exit(1)
	}
	var hdr recordHeader
	if err := json.Unmarshal(scanner.Bytes(), &hdr); err != nil {
		fmt.Fprintf(os.Stderr, "header: %v\n", err)
		os.Exit(1)
	}
	if hdr.Type != "header" {
		fmt.Fprintf(os.Stderr, "expected header, got %q\n", hdr.Type)
		os.Exit(1)
	}

	// Load all frames
	var frames []frame
	for scanner.Scan() {
		var fr frame
		if err := json.Unmarshal(scanner.Bytes(), &fr); err != nil {
			fmt.Fprintf(os.Stderr, "frame %d: %v\n", len(frames), err)
			continue
		}
		frames = append(frames, fr)
	}
	if len(frames) == 0 {
		fmt.Fprintf(os.Stderr, "no frames loaded\n")
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "Loaded %d frames (seed=%d, %dx%d, %d ticks)\n",
		len(frames), hdr.Seed, hdr.WorldSize, hdr.WorldSize, hdr.Ticks)

	// Enter raw terminal mode
	rawOn := exec.Command("stty", "raw", "-echo")
	rawOn.Stdin = os.Stdin
	if err := rawOn.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "stty raw: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		rawOff := exec.Command("stty", "-raw", "echo")
		rawOff.Stdin = os.Stdin
		rawOff.Run()
		fmt.Print("\033[?25h") // show cursor
	}()

	fmt.Print("\033[2J")   // clear screen
	fmt.Print("\033[?25l") // hide cursor

	idx := *start
	if idx >= len(frames) {
		idx = len(frames) - 1
	}
	if idx < 0 {
		idx = 0
	}
	paused := false
	fps := *speed
	if fps < 1 {
		fps = 1
	}
	if fps > 60 {
		fps = 60
	}

	// Non-blocking stdin reader
	keyCh := make(chan byte, 32)
	go func() {
		buf := make([]byte, 1)
		for {
			n, err := os.Stdin.Read(buf)
			if n > 0 {
				keyCh <- buf[0]
			}
			if err != nil {
				return
			}
		}
	}()

	ticker := time.NewTicker(time.Second / time.Duration(fps))
	defer ticker.Stop()

	render(hdr, frames[idx], idx, len(frames), fps, paused)

	for {
		select {
		case key := <-keyCh:
			switch key {
			case 'q', 'Q', 3: // q or Ctrl-C
				fmt.Print("\033[H\033[2J")
				return
			case ' ':
				paused = !paused
				render(hdr, frames[idx], idx, len(frames), fps, paused)
			case '+', '=':
				fps++
				if fps > 60 {
					fps = 60
				}
				ticker.Reset(time.Second / time.Duration(fps))
				render(hdr, frames[idx], idx, len(frames), fps, paused)
			case '-', '_':
				fps--
				if fps < 1 {
					fps = 1
				}
				ticker.Reset(time.Second / time.Duration(fps))
				render(hdr, frames[idx], idx, len(frames), fps, paused)
			case 27: // escape sequence (arrow keys)
				// Read next two bytes for arrow key
				select {
				case b2 := <-keyCh:
					if b2 == '[' {
						select {
						case b3 := <-keyCh:
							switch b3 {
							case 'D': // left arrow
								if idx > 0 {
									idx--
								}
								render(hdr, frames[idx], idx, len(frames), fps, paused)
							case 'C': // right arrow
								if idx < len(frames)-1 {
									idx++
								}
								render(hdr, frames[idx], idx, len(frames), fps, paused)
							}
						case <-time.After(50 * time.Millisecond):
						}
					}
				case <-time.After(50 * time.Millisecond):
				}
			}
		case <-ticker.C:
			if !paused && idx < len(frames)-1 {
				idx++
				render(hdr, frames[idx], idx, len(frames), fps, paused)
			}
		}
	}
}

// ANSI color helpers
const (
	reset   = "\033[0m"
	bold    = "\033[1m"
	// Foreground
	fgBlack   = "\033[30m"
	fgRed     = "\033[31m"
	fgGreen   = "\033[32m"
	fgYellow  = "\033[33m"
	fgBlue    = "\033[34m"
	fgMagenta = "\033[35m"
	fgCyan    = "\033[36m"
	fgWhite   = "\033[37m"
	// Background
	bgBlack   = "\033[40m"
	bgRed     = "\033[41m"
	bgGreen   = "\033[42m"
	bgYellow  = "\033[43m"
	bgBlue    = "\033[44m"
	bgMagenta = "\033[45m"
	bgCyan    = "\033[46m"
	bgWhite   = "\033[47m"
	// Bright foreground
	fgBrightRed     = "\033[91m"
	fgBrightGreen   = "\033[92m"
	fgBrightYellow  = "\033[93m"
	fgBrightCyan    = "\033[96m"
	fgBrightWhite   = "\033[97m"
	// Bright background
	bgBrightBlack = "\033[100m"
)

// Biome types (matching pkg/sandbox/wfc.go)
const (
	biomeClearing = 0
	biomeForest   = 1
	biomeMountain = 2
	biomeSwamp    = 3
	biomeVillage  = 4
	biomeRiver    = 5
	biomeBridge   = 6
)

func biomeBG(b byte) string {
	switch b {
	case biomeForest:
		return "\033[48;5;22m" // dark green
	case biomeMountain:
		return "\033[48;5;240m" // dark gray
	case biomeSwamp:
		return "\033[48;5;58m" // olive/brown
	case biomeVillage:
		return "\033[48;5;136m" // tan/brown
	case biomeRiver:
		return "\033[48;5;24m" // dark blue
	case biomeBridge:
		return "\033[48;5;94m" // wood brown
	default: // clearing
		return "\033[48;5;236m" // very dark gray (neutral)
	}
}

func render(hdr recordHeader, fr frame, idx, total, fps int, paused bool) {
	var sb strings.Builder
	sb.WriteString("\033[H") // cursor home

	ws := hdr.WorldSize

	// Build occupancy map from NPCs
	occMap := make(map[int]recordNPC, len(fr.NPCs))
	for _, npc := range fr.NPCs {
		occMap[npc.Y*ws+npc.X] = npc
	}

	hasBiomes := len(hdr.BiomeGrid) >= ws*ws

	// Side-panel legend (rendered alongside map if it fits)
	showPanel := ws < 100
	panelCol := ws + 3 // column where legend starts (1-based for ANSI)

	type legendLine struct {
		text string
	}
	var legend []legendLine
	if showPanel {
		// Build legend lines to overlay on map rows
		legend = []legendLine{
			{fmt.Sprintf("%s│%s TERRAIN          NPCs", reset, bold)},
			{fmt.Sprintf("%s│%s %s  %s Clearing      %s@%s naked", reset, reset, biomeBG(biomeClearing) + "  " + reset, reset, bold + fgBrightWhite + "@" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s Forest        %st%s tool", reset, reset, biomeBG(biomeForest) + "  " + reset, reset, bold + fgBrightYellow + "t" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s Mountain      %sw%s weapon", reset, reset, biomeBG(biomeMountain) + "  " + reset, reset, bold + fgBrightYellow + "w" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s Swamp         %s$%s treasure", reset, reset, biomeBG(biomeSwamp) + "  " + reset, reset, bold + fgBrightYellow + "$" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s Village       %s*%s crystal", reset, reset, biomeBG(biomeVillage) + "  " + reset, reset, bold + fgBrightYellow + "*" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s River         %ss%s shield", reset, reset, biomeBG(biomeRiver) + "  " + reset, reset, bold + fgBrightYellow + "s" + reset, reset)},
			{fmt.Sprintf("%s│%s %s  %s Bridge        %sc%s compass", reset, reset, biomeBG(biomeBridge) + "  " + reset, reset, bold + fgBrightYellow + "c" + reset, reset)},
			{fmt.Sprintf("%s│%s", reset, reset)},
			{fmt.Sprintf("%s│%s %s(yellow%s=has item)", reset, reset, fgBrightYellow, reset)},
			{fmt.Sprintf("%s│%s %s(red%s=dying HP<30)", reset, reset, fgBrightRed, reset)},
			{fmt.Sprintf("%s│%s %s(white%s=no item)", reset, reset, fgBrightWhite, reset)},
			{fmt.Sprintf("%s│%s", reset, reset)},
			{fmt.Sprintf("%s│%s TILES", reset, bold)},
			{fmt.Sprintf("%s│%s %sf%s food      %s!%s poison", reset, reset, fgGreen + "f" + reset, reset, bold + fgBrightRed + "!" + reset, reset)},
			{fmt.Sprintf("%s│%s %st%s tool      %s#%s wall", reset, reset, fgCyan + "t" + reset, reset, fgBlue + "#" + reset, reset)},
			{fmt.Sprintf("%s│%s %sw%s weapon    %s·%s empty", reset, reset, fgRed + "w" + reset, reset, "\033[38;5;239m" + "·" + reset, reset)},
			{fmt.Sprintf("%s│%s %s$%s treasure", reset, reset, fgYellow + "$" + reset, reset)},
			{fmt.Sprintf("%s│%s %s*%s crystal   CONTROLS", reset, reset, bold + fgMagenta + "*" + reset, reset)},
			{fmt.Sprintf("%s│%s %sF%s forge     Space  pause", reset, reset, bold + fgBrightCyan + "F" + reset, reset)},
			{fmt.Sprintf("%s│%s             ←/→    step", reset, reset)},
			{fmt.Sprintf("%s│%s             +/-    speed", reset, reset)},
			{fmt.Sprintf("%s│%s             q      quit", reset, reset)},
		}
	}

	// Render grid
	for y := 0; y < ws; y++ {
		for x := 0; x < ws; x++ {
			i := y*ws + x

			// Get biome background color
			bg := ""
			if hasBiomes && i < len(hdr.BiomeGrid) {
				bg = biomeBG(hdr.BiomeGrid[i])
			}

			if npc, ok := occMap[i]; ok {
				// NPC: bright white on biome background
				sb.WriteString(bg)
				sb.WriteString(bold)
				if npc.HP < 30 {
					sb.WriteString(fgBrightRed) // dying = red
				} else if npc.Item != 0 {
					sb.WriteString(fgBrightYellow) // has item = yellow
				} else {
					sb.WriteString(fgBrightWhite) // normal = white
				}
				sb.WriteByte(npcChar(npc))
				sb.WriteString(reset)
			} else if i < len(fr.Grid) {
				tile := fr.Grid[i]
				sb.WriteString(bg)
				sb.WriteString(tileColor(tile))
				sb.WriteString(tileChar(tile))
				sb.WriteString(reset)
			} else {
				sb.WriteByte(' ')
			}
		}

		// Append legend panel on this row
		if showPanel && y < len(legend) {
			fmt.Fprintf(&sb, "\033[%dG%s", panelCol, legend[y].text)
		}
		sb.WriteString("\033[K\r\n") // clear to EOL + newline
	}

	// Status bar
	pauseStr := ""
	if paused {
		pauseStr = " [PAUSED]"
	}
	fmt.Fprintf(&sb, "\033[K\r\nTick %d/%d | Frame %d/%d | NPCs: %d | Atk: %d | Heal: %d | Trade: %d | Speed: %dfps%s\033[K\r\n",
		fr.Tick, hdr.Ticks, idx+1, total, len(fr.NPCs),
		fr.Stats.Attacks, fr.Stats.Heals, fr.Stats.Trades,
		fps, pauseStr)
	if !showPanel {
		// Fallback footer legend for very large maps
		sb.WriteString(bold + fgBrightWhite + "\u2588" + reset + "=NPC " +
			bold + fgBrightYellow + "\u2588" + reset + "=NPC+item " +
			bold + fgBrightRed + "\u2588" + reset + "=dying  " +
			fgGreen + "f" + reset + "=food " +
			fgCyan + "t" + reset + "=tool " +
			fgRed + "w" + reset + "=weapon " +
			fgYellow + "$" + reset + "=treasure " +
			fgMagenta + "*" + reset + "=crystal " +
			fgBrightCyan + "F" + reset + "=forge\033[K\r\n")
		sb.WriteString("[Space]=pause [\u2190\u2192]=step [+/-]=speed [q]=quit\033[K\r\n")
	}

	fmt.Print(sb.String())
}

// npcChar returns a character for the NPC based on its dominant behavior hint
func npcChar(npc recordNPC) byte {
	if npc.Item != 0 {
		// Show item type carried
		switch npc.Item {
		case 2: // tool
			return 't'
		case 3: // weapon
			return 'w'
		case 4: // treasure
			return '$'
		case 5: // crystal
			return '*'
		case 6: // shield
			return 's'
		case 7: // compass
			return 'c'
		default:
			return 'o'
		}
	}
	return '@'
}

func tileColor(t byte) string {
	switch t {
	case 2: // food
		return fgGreen
	case 4: // tool
		return fgCyan
	case 5: // weapon
		return fgRed
	case 6: // treasure
		return fgYellow
	case 7: // crystal
		return bold + fgMagenta
	case 8: // forge
		return bold + fgBrightCyan
	case 9: // poison
		return bold + fgBrightRed
	case 1: // wall
		return fgBlue
	default:
		return "\033[38;5;239m" // dim gray for empty
	}
}

func tileChar(t byte) string {
	switch t {
	case 2: // TileFood
		return "f"
	case 4: // TileTool
		return "t"
	case 5: // TileWeapon
		return "w"
	case 6: // TileTreasure
		return "$"
	case 7: // TileCrystal
		return "*"
	case 8: // TileForge
		return "F"
	case 9: // TilePoison
		return "!"
	case 1: // TileWall
		return "#"
	default:
		return "\u00b7" // middle dot
	}
}
