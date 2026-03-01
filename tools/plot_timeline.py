#!/usr/bin/env python3
"""Plot sandbox timeline from CSV.

Usage:
    go run ./cmd/sandbox --csv --npcs 500 --ticks 50000 --seed 42 > data.csv
    python3 tools/plot_timeline.py data.csv
    python3 tools/plot_timeline.py data.csv -o timeline.png
    # or pipe directly:
    go run ./cmd/sandbox --csv --npcs 500 --ticks 50000 2>/dev/null | python3 tools/plot_timeline.py -
"""

import argparse
import csv
import sys

import matplotlib.pyplot as plt


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


def plot(cols, out):
    ticks = cols["tick"]

    fig, axes = plt.subplots(3, 2, figsize=(14, 10), sharex=True)
    fig.suptitle("Sandbox Simulation Timeline", fontsize=14, fontweight="bold")

    # Population & stress
    ax = axes[0, 0]
    ax.plot(ticks, cols["alive"], label="alive", color="#2196F3", linewidth=1.5)
    ax.set_ylabel("count")
    ax.set_title("Population")
    ax.legend(loc="upper right")
    ax2 = ax.twinx()
    ax2.plot(ticks, cols["avg_stress"], label="avg stress", color="#F44336",
             linewidth=1, alpha=0.7, linestyle="--")
    ax2.set_ylabel("stress (0-100)")
    ax2.legend(loc="upper left")

    # Economy
    ax = axes[0, 1]
    ax.plot(ticks, cols["trades"], label="trades (cum)", color="#4CAF50", linewidth=1.5)
    ax.plot(ticks, cols["teaches"], label="teaches (cum)", color="#FF9800", linewidth=1.5)
    ax.set_ylabel("count (cumulative)")
    ax.set_title("Economy & Knowledge")
    ax.legend()

    # Trade & teach rates
    ax = axes[1, 0]
    dt = [ticks[i] - ticks[i - 1] for i in range(1, len(ticks))]
    trade_rate = [(cols["trades"][i] - cols["trades"][i - 1])
                  for i in range(1, len(ticks))]
    teach_rate = [(cols["teaches"][i] - cols["teaches"][i - 1])
                  for i in range(1, len(ticks))]
    ax.plot(ticks[1:], trade_rate, label="trades/interval", color="#4CAF50",
            linewidth=1, alpha=0.8)
    ax.plot(ticks[1:], teach_rate, label="teaches/interval", color="#FF9800",
            linewidth=1, alpha=0.8)
    ax.set_ylabel("per interval")
    ax.set_title("Activity Rates")
    ax.legend()

    # Resources
    ax = axes[1, 1]
    ax.plot(ticks, cols["food"], label="food", color="#8BC34A", linewidth=1.5)
    ax.plot(ticks, cols["items"], label="items", color="#9C27B0", linewidth=1.5)
    ax.set_ylabel("on map")
    ax.set_title("Resources")
    ax.legend()
    ax2 = ax.twinx()
    ax2.plot(ticks, cols["gold"], label="gold", color="#FFC107",
             linewidth=1, alpha=0.7, linestyle="--")
    ax2.set_ylabel("total gold")
    ax2.legend(loc="upper left")

    # Fitness
    ax = axes[2, 0]
    ax.plot(ticks, cols["best_fit"], label="best fitness", color="#E91E63", linewidth=1.5)
    ax.plot(ticks, cols["avg_fit"], label="avg fitness", color="#3F51B5", linewidth=1.5)
    ax.set_ylabel("fitness")
    ax.set_xlabel("tick")
    ax.set_title("Fitness")
    ax.legend()

    # Items & crafting
    ax = axes[2, 1]
    ax.plot(ticks, cols["holders"], label="holders", color="#00BCD4", linewidth=1.5)
    ax.plot(ticks, cols["crafted"], label="crafted (shield/compass)", color="#FF5722",
            linewidth=1.5)
    ax.plot(ticks, cols["crystal_npcs"], label="crystal NPCs", color="#9E9E9E",
            linewidth=1.5)
    ax.set_ylabel("count")
    ax.set_xlabel("tick")
    ax.set_title("Items & Crafting")
    ax.legend()

    plt.tight_layout()

    if out:
        plt.savefig(out, dpi=150, bbox_inches="tight")
        print(f"Saved to {out}", file=sys.stderr)
    else:
        plt.show()


def main():
    parser = argparse.ArgumentParser(description="Plot sandbox timeline CSV")
    parser.add_argument("csv_file", help="CSV file path or - for stdin")
    parser.add_argument("-o", "--output", help="Save to file (png/pdf/svg) instead of showing")
    args = parser.parse_args()

    cols = load_csv(args.csv_file)
    plot(cols, args.output)


if __name__ == "__main__":
    main()
