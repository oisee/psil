#!/usr/bin/env python3
"""Generate Mermaid xychart-beta diagrams from sandbox CSV.

Usage:
    go run ./cmd/sandbox --csv --npcs 500 --ticks 10000 --seed 42 > data.csv
    python3 tools/mermaid_timeline.py data.csv                   # all charts
    python3 tools/mermaid_timeline.py data.csv -m alive,trades   # specific metrics
    python3 tools/mermaid_timeline.py data.csv -o charts.md      # write to file
    python3 tools/mermaid_timeline.py data.csv --flowchart       # system flowchart

Outputs fenced Mermaid blocks (```mermaid ... ```) ready to paste into
GitHub markdown, reports, or READMEs.
"""

import argparse
import csv
import sys


def load_csv(path):
    f = sys.stdin if path == "-" else open(path)
    reader = csv.DictReader(f)
    rows = list(reader)
    if f is not sys.stdin:
        f.close()
    cols = {}
    for key in rows[0]:
        cols[key] = [int(r[key]) for r in rows]
    return cols


def downsample(values, max_points=30):
    """Mermaid xychart can get unwieldy with too many points."""
    if len(values) <= max_points:
        return values
    step = len(values) / max_points
    result = []
    for i in range(max_points):
        start = int(i * step)
        end = int((i + 1) * step)
        result.append(sum(values[start:end]) // (end - start))
    return result


def xychart(title, x_values, series, x_label="tick", y_label="value"):
    """Generate a Mermaid xychart-beta block."""
    x_down = downsample(x_values)
    lines = [
        "```mermaid",
        "xychart-beta",
        f'    title "{title}"',
        f'    x-axis "{x_label}" [{", ".join(str(v) for v in x_down)}]',
    ]
    for name, values in series:
        y_down = downsample(values)
        lines.append(f'    line [{", ".join(str(v) for v in y_down)}]')
    lines.append("```")
    return "\n".join(lines)


def flowchart():
    """Static system architecture flowchart for sandbox."""
    return """```mermaid
flowchart TD
    subgraph Simulation
        W[World NxN Grid] --> S[Scheduler]
        S --> |tick| VM[micro-PSIL VM]
        VM --> |actions| W
    end

    subgraph NPC Lifecycle
        Spawn[Spawn] --> Live[Live & Act]
        Live --> |eat/trade/craft/teach| Live
        Live --> |age/starve/poison| Die[Die]
        Die --> |evolve| Spawn
    end

    subgraph Economy
        Food[Food Tiles] --> |eat| Energy[NPC Energy]
        Items[Item Tiles] --> |pickup| Trade[Trade]
        Trade --> |bilateral| Gold[Gold]
        Trade --> Craft[Craft]
        Craft --> |forge| Shield[Shield/Compass]
    end

    subgraph Knowledge
        Genome[NPC Genome] --> |teach| Student[Student Genome]
        Student --> |mutate| Evolved[Evolved Genome]
        Evolved --> |GA| Genome
    end

    S --> |sample| TL[Timeline CSV]
    TL --> |pyplot| Plot[PNG Charts]
    TL --> |mermaid| MD[Markdown Diagrams]
```"""


METRIC_GROUPS = {
    "alive":     ("Population", "alive", "count"),
    "trades":    ("Cumulative Trades", "trades", "count"),
    "teaches":   ("Cumulative Teaches", "teaches", "count"),
    "gold":      ("Total Gold", "gold", "gold"),
    "avg_stress": ("Average Stress", "avg_stress", "stress (0-100)"),
    "food":      ("Food on Map", "food", "tiles"),
    "items":     ("Items on Map", "items", "tiles"),
    "avg_fit":   ("Average Fitness", "avg_fit", "fitness"),
    "best_fit":  ("Best Fitness", "best_fit", "fitness"),
    "holders":   ("Item Holders", "holders", "count"),
    "crafted":   ("Crafted Items (shield+compass)", "crafted", "count"),
    "crystal_npcs": ("Crystal NPCs", "crystal_npcs", "count"),
}

# Composite charts that combine related metrics
COMPOSITES = {
    "population": ("Population & Stress", [("alive", "alive"), ("avg_stress", "avg_stress")]),
    "economy":    ("Economy", [("trades", "trades"), ("teaches", "teaches")]),
    "fitness":    ("Fitness", [("best_fit", "best_fit"), ("avg_fit", "avg_fit")]),
    "resources":  ("Resources", [("food", "food"), ("items", "items")]),
}


def main():
    parser = argparse.ArgumentParser(description="Generate Mermaid diagrams from sandbox CSV")
    parser.add_argument("csv_file", help="CSV file path or - for stdin")
    parser.add_argument("-m", "--metrics", help="Comma-separated metrics (default: composites)")
    parser.add_argument("-o", "--output", help="Write to file instead of stdout")
    parser.add_argument("--flowchart", action="store_true", help="Include system flowchart")
    parser.add_argument("--all", action="store_true", help="All individual metrics")
    args = parser.parse_args()

    out = open(args.output, "w") if args.output else sys.stdout
    cols = load_csv(args.csv_file)
    ticks = cols["tick"]

    if args.flowchart:
        print(flowchart(), file=out)
        print(file=out)

    if args.metrics:
        for key in args.metrics.split(","):
            key = key.strip()
            if key in METRIC_GROUPS:
                title, col, ylabel = METRIC_GROUPS[key]
                print(xychart(title, ticks, [(col, cols[col])], y_label=ylabel), file=out)
                print(file=out)
    elif args.all:
        for key, (title, col, ylabel) in METRIC_GROUPS.items():
            print(xychart(title, ticks, [(col, cols[col])], y_label=ylabel), file=out)
            print(file=out)
    else:
        # Default: composite charts
        for key, (title, series_defs) in COMPOSITES.items():
            series = [(name, cols[col]) for name, col in series_defs]
            print(xychart(title, ticks, series), file=out)
            print(file=out)

    if args.output:
        out.close()
        print(f"Wrote to {args.output}", file=sys.stderr)


if __name__ == "__main__":
    main()
