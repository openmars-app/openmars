#!/usr/bin/env python3
"""CLI entry point for card image generation.

Usage:
    # Generate a single card
    python -m scripts.card_image_gen.generate_cards --card 042

    # Dry run (show prompt only)
    python -m scripts.card_image_gen.generate_cards --card 042 --dry-run

    # Generate all missing cards
    python -m scripts.card_image_gen.generate_cards --missing

    # Generate a range
    python -m scripts.card_image_gen.generate_cards --start 042 --end 050

    # Fixed seed for reproducibility
    python -m scripts.card_image_gen.generate_cards --card 042 --seed 12345
"""

import argparse
import asyncio
import sys
from pathlib import Path

from .card_generator import CardImageGenerator
from .prompt_builder import build_prompt


def _get_target_cards(args: argparse.Namespace, gen: CardImageGenerator) -> list[dict]:
    if args.card:
        return [gen.find_card(args.card)]

    if args.missing:
        cards = gen.get_missing_cards()
        if not cards:
            print("All cards already have images.")
            sys.exit(0)
        return cards

    if args.start or args.end:
        all_ids = [c["id"] for c in gen.cards]
        start_idx = all_ids.index(args.start) if args.start else 0
        end_idx = all_ids.index(args.end) + 1 if args.end else len(all_ids)
        return gen.cards[start_idx:end_idx]

    print("Specify --card, --missing, or --start/--end. Use --help for details.")
    sys.exit(1)


async def _run(args: argparse.Namespace) -> None:
    gen = CardImageGenerator(candidate_dir=args.candidate_dir)
    cards = _get_target_cards(args, gen)

    if args.dry_run:
        for card in cards:
            prompt = build_prompt(card)
            print(f"\n[{card['id']}] {card['name']}")
            print(f"  Type: {card.get('type', 'N/A')}")
            print(f"  Tags: {card.get('tags', [])}")
            print(f"  Prompt: {prompt}")
        print(f"\n{len(cards)} card(s) shown.")
        return

    await gen.generate_batch(cards, seed=args.seed)


def main() -> None:
    parser = argparse.ArgumentParser(
        description="Generate Open Mars card images via ComfyUI + Flux Schnell"
    )
    parser.add_argument("--card", help="Generate a specific card by ID (e.g. 042)")
    parser.add_argument("--missing", action="store_true", help="Generate all missing card images")
    parser.add_argument("--start", help="Start card ID for a range")
    parser.add_argument("--end", help="End card ID for a range")
    parser.add_argument("--dry-run", action="store_true", help="Show prompts without generating")
    parser.add_argument("--seed", type=int, help="Fixed seed for reproducibility")
    parser.add_argument("--candidate-dir", type=Path,
                        help="Save review candidates and manifests without publishing assets")
    args = parser.parse_args()

    try:
        asyncio.run(_run(args))
    except (RuntimeError, ValueError, OSError) as exc:
        print(f"ERROR: {exc}", file=sys.stderr)
        sys.exit(1)


if __name__ == "__main__":
    main()
