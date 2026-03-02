#!/usr/bin/env python3
"""Plot A/B comparison of growth vs classic crossover from two CSV files.

Usage:
    python3 tools/plot_ab_comparison.py growth.csv classic.csv -o comparison.png
"""

import argparse
import csv
import sys

import matplotlib.pyplot as plt


def load_csv(path):
    with open(path) as f:
        reader = csv.DictReader(f)
        rows = list(reader)
    cols = {}
    for key in rows[0]:
        cols[key] = [int(r[key]) for r in rows]
    return cols


def plot(growth, classic, out, seed="42"):
    ticks = growth["tick"]

    fig, axes = plt.subplots(3, 2, figsize=(14, 10), sharex=True)
    fig.suptitle(
        f"Growth/Exchange vs Classic Crossover (seed={seed}, 200 NPCs, 10k ticks)",
        fontsize=13, fontweight="bold",
    )

    colors = {"growth": "#2196F3", "classic": "#F44336"}

    # avgFit
    ax = axes[0, 0]
    ax.plot(ticks, growth["avg_fit"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["avg_fit"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("fitness")
    ax.set_title("Average Fitness")
    ax.legend()

    # bestFit
    ax = axes[0, 1]
    ax.plot(ticks, growth["best_fit"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["best_fit"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("fitness")
    ax.set_title("Best Fitness")
    ax.legend()

    # Trades
    ax = axes[1, 0]
    ax.plot(ticks, growth["trades"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["trades"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("cumulative trades")
    ax.set_title("Trade Volume")
    ax.legend()

    # Alive
    ax = axes[1, 1]
    ax.plot(ticks, growth["alive"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["alive"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("NPCs alive")
    ax.set_title("Population")
    ax.legend()

    # Genome avg
    ax = axes[2, 0]
    ax.plot(ticks, growth["genome_avg"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["genome_avg"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("bytes")
    ax.set_title("Average Genome Size")
    ax.set_xlabel("tick")
    ax.legend()

    # Gold
    ax = axes[2, 1]
    ax.plot(ticks, growth["gold"], label="growth", color=colors["growth"], linewidth=1.5)
    ax.plot(ticks, classic["gold"], label="classic", color=colors["classic"], linewidth=1.5, linestyle="--")
    ax.set_ylabel("total gold")
    ax.set_title("Economy (Gold)")
    ax.set_xlabel("tick")
    ax.legend()

    plt.tight_layout()
    if out:
        plt.savefig(out, dpi=150)
        print(f"Saved to {out}", file=sys.stderr)
    else:
        plt.show()


def main():
    parser = argparse.ArgumentParser(description="Plot A/B crossover comparison")
    parser.add_argument("growth_csv", help="CSV from growth crossover run")
    parser.add_argument("classic_csv", help="CSV from classic crossover run")
    parser.add_argument("-o", "--output", help="Output file (PNG/PDF/SVG)")
    parser.add_argument("--seed", default="42", help="Seed label for title")
    args = parser.parse_args()

    growth = load_csv(args.growth_csv)
    classic = load_csv(args.classic_csv)
    plot(growth, classic, args.output, args.seed)


if __name__ == "__main__":
    main()
