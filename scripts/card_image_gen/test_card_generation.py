"""Candidate isolation, generation failures, and review-record contracts."""

import hashlib
import json
import tempfile
import unittest
from io import BytesIO
from pathlib import Path
from unittest.mock import AsyncMock, patch

from aiohttp import web
from PIL import Image

from . import config
from .card_generator import CardImageGenerator
from .comfyui_client import ComfyUIClient
from .generate_cards import main
from .prompt_builder import BASE_STYLE, build_prompt


CARD = {"id": "ES004", "name": "Remote Analysis Software", "type": "active"}


class CandidateTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.temp = tempfile.TemporaryDirectory()
        self.root = Path(self.temp.name)
        self.addCleanup(self.temp.cleanup)
        self.catalog = self.root / "assets" / "catalog.json"
        self.catalog.parent.mkdir()
        self.catalog.write_text('{"assets": []}\n')
        self.master = self.root / "assets" / "original" / "cards" / "ES004.png"
        self.master.parent.mkdir(parents=True)
        self.master.write_bytes(b"existing artwork")
        for name, value in (("PROJECT_ROOT", self.root), ("CATALOG_PATH", self.catalog),
                            ("OUTPUT_DIR", self.master.parent), ("BATCH_DELAY", 0)):
            self.enterContext(patch.object(config, name, value))
        self.enterContext(patch.object(CardImageGenerator, "_load_cards", return_value=[CARD]))
        self.publish = self.enterContext(patch("scripts.card_image_gen.card_generator.process_and_save"))
        self.png = BytesIO()
        Image.new("RGB", (960, 720), "navy").save(self.png, "PNG")
        self.generator = CardImageGenerator(self.root / "candidates")
        self.generator.client.generate = AsyncMock(return_value=self.png.getvalue())
        self.generator.client.system_stats = AsyncMock(return_value={"system": {"comfyui_version": "test"}})

    async def test_candidate_keeps_production_unchanged_and_records_reproduction(self):
        path = await self.generator.generate_card(CARD, seed=1001)
        self.assertEqual(path.read_bytes(), self.png.getvalue())
        record = json.loads((path.parent / "manifest.json").read_text())
        self.assertEqual(record["prompt"], build_prompt(CARD))
        self.assertEqual(record["seed"], 1001)
        self.assertEqual(record["workflow"]["6"]["inputs"]["seed"], 1001)
        self.assertEqual(record["image_sha256"], hashlib.sha256(path.read_bytes()).hexdigest())
        self.assertEqual(record["server"]["system"]["comfyui_version"], "test")
        self.assertEqual(self.catalog.read_text(), '{"assets": []}\n')
        self.assertEqual(self.master.read_bytes(), b"existing artwork")
        self.publish.assert_not_called()

    async def test_same_card_and_seed_cannot_overwrite_reviewed_candidate(self):
        path = await self.generator.generate_card(CARD, seed=1001)
        before = path.read_bytes()
        with self.assertRaises(FileExistsError):
            await self.generator.generate_card(CARD, seed=1001)
        self.assertEqual(path.read_bytes(), before)
        self.generator.client.generate.assert_awaited_once()

    async def test_generation_error_is_not_resubmitted_or_published(self):
        self.generator.client.generate.side_effect = TimeoutError("still running")
        with self.assertRaisesRegex(RuntimeError, "ES004: still running"):
            await self.generator.generate_batch([CARD], seed=1001)
        self.generator.client.generate.assert_awaited_once()
        self.assertFalse(self.generator.candidate_dir.exists())
        self.publish.assert_not_called()

    async def test_invalid_image_is_not_saved(self):
        invalid = BytesIO()
        Image.new("RGB", (1, 1)).save(invalid, "PNG")
        self.generator.client.generate.return_value = invalid.getvalue()
        with self.assertRaisesRegex(ValueError, "dimensions"):
            await self.generator.generate_card(CARD, seed=1001)
        self.assertFalse(self.generator.candidate_dir.exists())
        self.publish.assert_not_called()

    async def test_partial_batch_reports_failure_and_preserves_successful_candidates(self):
        other_card = {**CARD, "id": "ES010", "name": "Independence Petition"}
        self.generator.client.generate.side_effect = [RuntimeError("GPU execution failed"), self.png.getvalue()]
        with self.assertRaisesRegex(RuntimeError, "ES004: GPU execution failed"):
            await self.generator.generate_batch([CARD, other_card], seed=1001)
        self.assertFalse((self.generator.candidate_dir / "ES004-seed-1001").exists())
        successful = self.generator.candidate_dir / "ES010-seed-1001"
        self.assertTrue((successful / "image.png").is_file())
        self.assertTrue((successful / "manifest.json").is_file())
        self.publish.assert_not_called()

    async def test_candidate_destination_cannot_be_production_or_symlink_to_it(self):
        for destination in (self.root / "assets" / "original", self.root / "frontend" / "public"):
            with self.assertRaisesRegex(ValueError, "outside production"):
                CardImageGenerator(destination)
        link = self.root / "review-link"
        link.symlink_to(self.catalog.parent, target_is_directory=True)
        with self.assertRaisesRegex(ValueError, "outside production"):
            CardImageGenerator(link)


class ComfyFailureTests(unittest.IsolatedAsyncioTestCase):
    async def asyncSetUp(self):
        self.history = {}
        self.response = {"node_errors": {"1": {"message": "model missing"}}}
        app = web.Application()

        async def history(_request):
            return web.json_response(self.history)

        async def prompt(_request):
            return web.json_response(self.response, status=400)

        app.router.add_get("/history/job", history)
        app.router.add_post("/prompt", prompt)
        self.runner = web.AppRunner(app)
        await self.runner.setup()
        site = web.TCPSite(self.runner, "127.0.0.1", 0)
        await site.start()
        self.addAsyncCleanup(self.runner.cleanup)
        port = site._server.sockets[0].getsockname()[1]
        self.client = ComfyUIClient(f"http://127.0.0.1:{port}")

    async def test_reports_workflow_validation_details(self):
        with self.assertRaisesRegex(RuntimeError, "model missing"):
            await self.client.queue_prompt({})

    async def test_reports_gpu_execution_error(self):
        self.history = {"job": {"status": {"status_str": "error", "messages": [
            ["execution_error", {"node_id": "6", "exception_message": "CUDA out of memory"}]
        ]}}}
        with self.assertRaisesRegex(RuntimeError, "CUDA out of memory"):
            await self.client.wait_for_completion("job")

    async def test_timeout_keeps_job_identifier_for_inspection(self):
        with self.assertRaisesRegex(TimeoutError, "job.*may still be queued or running"):
            await self.client.wait_for_completion("job", poll_interval=0, timeout=0)


class CommandAndPromptTests(unittest.TestCase):
    def test_cli_partial_failure_returns_nonzero(self):
        with patch("sys.argv", ["generate_cards", "--card", "ES004"]), \
                patch("scripts.card_image_gen.generate_cards._run", new=AsyncMock(side_effect=RuntimeError("failed"))):
            with self.assertRaises(SystemExit) as exit_result:
                main()
            self.assertEqual(exit_result.exception.code, 1)

    def test_scenario_cards_use_the_original_subject_and_style_recipe(self):
        import json
        cards = json.loads(config.CARD_JSON_PATH.read_text())
        cards = [card for card in cards if card["id"].startswith(("ES", "EP"))]
        self.assertEqual(len(cards), 48)
        prompts = [build_prompt(card) for card in cards]
        self.assertEqual(len(set(prompts)), 48)
        for card, prompt in zip(cards, prompts):
            self.assertGreater(len(prompt), len(BASE_STYLE))
            self.assertTrue(prompt.endswith(BASE_STYLE))
        prompt = build_prompt({**CARD, "tags": ["science"], "description": [
            {"type": "requirement", "text": "Earth connected"},
            {"type": "generic", "text": "Gain 3 steel."},
        ]})
        self.assertIn("dynamic scene", prompt)
        self.assertNotIn("Gain 3 steel", prompt)
        self.assertNotIn("Earth connected", prompt)

    def test_selected_art_manifest_matches_every_new_production_asset(self):
        manifest = json.loads((config.PROJECT_ROOT / "assets/card-art-generation.json").read_text())
        expected = {f"ES{i:03}" for i in range(1, 45)} | {f"EP{i:02}" for i in range(1, 5)}
        self.assertEqual({record["card_id"] for record in manifest["cards"]}, expected)
        for record in manifest["cards"] + manifest["corporations"]:
            asset = config.PROJECT_ROOT / record["asset"]
            self.assertEqual(hashlib.sha256(asset.read_bytes()).hexdigest(), record["image_sha256"])
        for record in manifest["cards"]:
            self.assertIn(record["seed"], record["review"]["candidate_seeds"])
            self.assertEqual(record["workflow"]["4"]["inputs"]["t5xxl"], record["prompt"])
        self.assertEqual(len(manifest["environment"]["models"]), 4)

    def test_original_reference_prompt(self):
        prompt = build_prompt({"id": "020", "name": "Research Outpost", "type": "active",
                               "tags": ["building", "city", "science"]})
        self.assertEqual(prompt, "Research Outpost, detailed scene of Research Outpost, "
                         "futuristic industrial structures, domed city, scientific equipment, "
                         "dynamic scene, extreme wide establishing shot, "
                         "neon-lit cyberpunk ambiance, deep purple and violet hues, " + BASE_STYLE)


if __name__ == "__main__":
    unittest.main()
