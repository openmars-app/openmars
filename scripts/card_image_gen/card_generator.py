"""Main orchestration for card image generation."""

import asyncio
import hashlib
import json
import random
from datetime import datetime, timezone
from io import BytesIO
from pathlib import Path
from typing import Optional

from PIL import Image

from . import config
from .comfyui_client import ComfyUIClient, build_flux_workflow
from .image_processor import process_and_save
from .prompt_builder import build_prompt


class CardImageGenerator:
    """Generates card images using ComfyUI with Flux Schnell."""

    def __init__(self, candidate_dir: Optional[Path] = None) -> None:
        self.client = ComfyUIClient()
        self.cards = [card for card in self._load_cards() if card["type"] != "corporation"]
        self.output_dir = Path(config.OUTPUT_DIR)
        self.candidate_dir = candidate_dir.resolve() if candidate_dir else None
        if self.candidate_dir:
            for protected in (config.PROJECT_ROOT / "assets", config.PROJECT_ROOT / "frontend"):
                if self.candidate_dir.is_relative_to(protected.resolve()):
                    raise ValueError("Candidate output must be outside production assets and frontend")
        self.server_info: Optional[dict] = None

    @staticmethod
    def _load_cards() -> list[dict]:
        with open(config.CARD_JSON_PATH) as f:
            return json.load(f)

    def find_card(self, card_id: str) -> dict:
        """Look up a card by its ID."""
        for card in self.cards:
            if card["id"] == card_id:
                return card
        raise ValueError(f"Card '{card_id}' not found")

    def get_missing_cards(self) -> list[dict]:
        """Return cards that don't have generated images yet."""
        catalog = json.loads(config.CATALOG_PATH.read_text())
        available = {
            entry["id"] for entry in catalog["assets"]
            if (config.PROJECT_ROOT / "assets" / "original" / entry["source"]).is_file()
        }
        return [card for card in self.cards if f"cards/{card['id']}" not in available]

    async def generate_card(
        self, card: dict, seed: Optional[int] = None
    ) -> Path:
        """Generate an image for a single card and save it."""
        if seed is None:
            seed = random.randint(0, 2**32 - 1)

        candidate_path = None
        if self.candidate_dir:
            candidate_path = self.candidate_dir / f"{card['id']}-seed-{seed}"
            if candidate_path.exists():
                raise FileExistsError(f"Candidate already exists: {candidate_path}")
            if self.server_info is None:
                self.server_info = await self.client.system_stats()

        prompt = build_prompt(card)
        print(f"[{card['id']}] {card['name']}")
        print(f"  Prompt: {prompt[:120]}...")
        print(f"  Seed:   {seed}")

        # Never automatically resubmit: a timed-out job may still be running.
        image_data = await self.client.generate(prompt, seed)
        with Image.open(BytesIO(image_data)) as image:
            image.verify()
            if image.format != "PNG" or image.size != (config.IMAGE_WIDTH, config.IMAGE_HEIGHT):
                raise ValueError("ComfyUI returned an invalid format or unexpected image dimensions")

        if candidate_path:
            candidate_path.mkdir(parents=True)
            output_path = candidate_path / "image.png"
            output_path.write_bytes(image_data)
            manifest = {
                "card_id": card["id"],
                "card_name": card["name"],
                "created_at": datetime.now(timezone.utc).isoformat(),
                "prompt": prompt,
                "seed": seed,
                "workflow": build_flux_workflow(prompt, seed),
                "server": self.server_info,
                "image_sha256": hashlib.sha256(image_data).hexdigest(),
            }
            (candidate_path / "manifest.json").write_text(json.dumps(manifest, indent=2) + "\n")
        else:
            output_path = self.output_dir / f"{card['id']}.png"
            process_and_save(image_data, output_path)

        print(f"  Saved:  {output_path}")
        return output_path

    async def generate_batch(
        self, cards: list[dict], seed: Optional[int] = None
    ) -> list[Path]:
        """Generate images for multiple cards sequentially."""
        results: list[Path] = []
        total = len(cards)
        failures: list[str] = []
        for i, card in enumerate(cards, 1):
            print(f"\n--- Card {i}/{total} ---")
            try:
                path = await self.generate_card(card, seed=seed)
                results.append(path)
            except Exception as exc:
                print(f"  FAILED: {exc}")
                failures.append(f"{card['id']}: {exc}")

            if i < total:
                await asyncio.sleep(config.BATCH_DELAY)

        print(f"\nDone. Generated {len(results)}/{total} images.")
        if failures:
            raise RuntimeError("Generation failed for " + "; ".join(failures))
        return results
