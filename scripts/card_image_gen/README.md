# Card Image Generation

Generates project and prelude artwork through a local [ComfyUI](https://github.com/Comfy-Org/ComfyUI) server using FLUX.1 Schnell FP8. Corporation logos use a separate art process and are excluded from this generator.

## Local setup

Use an NVIDIA GPU with at least 8 GB VRAM, sufficient system RAM for offloading, and approximately 17 GB for model files plus Python dependencies. Other applications also consume VRAM; actual throughput and available memory must be checked with a sample generation.

Install outside the game checkout, in an isolated Python environment. Python 3.12 avoids depending on the system Python version. These commands use [uv](https://docs.astral.sh/uv/):

```bash
mkdir -p ~/ai
git clone https://github.com/Comfy-Org/ComfyUI.git ~/ai/ComfyUI
cd ~/ai/ComfyUI
uv python install 3.12
uv venv --python 3.12 .venv
uv pip install --python .venv/bin/python torch torchvision torchaudio --index-url https://download.pytorch.org/whl/cu130
uv pip install --python .venv/bin/python -r requirements.txt huggingface-hub
.venv/bin/python -c 'import torch; print(torch.__version__, torch.cuda.is_available(), torch.cuda.get_device_name())'
```

The CUDA wheel must support the installed NVIDIA driver. Consult the current [ComfyUI installation instructions](https://docs.comfy.org/installation/manual_install) and [PyTorch installer](https://pytorch.org/get-started/locally/) when installing on another machine. A separate CUDA toolkit installation is unnecessary for the bundled wheels.

Download these exact files through Hugging Face, pinning each repository revision when recording an art batch:

| Repository | File | ComfyUI directory |
| --- | --- | --- |
| `Kijai/flux-fp8` | `flux1-schnell-fp8-e4m3fn.safetensors` | `models/diffusion_models/` |
| `Kijai/flux-fp8` | `flux-vae-bf16.safetensors` | `models/vae/` |
| `comfyanonymous/flux_text_encoders` | `clip_l.safetensors` | `models/text_encoders/` |
| `comfyanonymous/flux_text_encoders` | `t5xxl_fp8_e4m3fn.safetensors` | `models/text_encoders/` |

For example, from the ComfyUI directory:

```python
from huggingface_hub import HfApi, hf_hub_download

repo = "Kijai/flux-fp8"
revision = HfApi().model_info(repo).sha
hf_hub_download(repo, "flux1-schnell-fp8-e4m3fn.safetensors",
                revision=revision, local_dir="models/diffusion_models")
```

Repeat for the other three files and their directories. No custom nodes or checkpoint symlinks are needed. Start the server manually, bound only to this machine:

```bash
cd ~/ai/ComfyUI
.venv/bin/python main.py --listen 127.0.0.1 --port 8188 --lowvram
```

Open `http://127.0.0.1:8188`. The game generator submits its node graph through the API, so no manual workflow construction is required. Keep generation sequential on an 8 GB card. The client waits up to 30 minutes for an individual job, including cold model loading and queue time; a timeout does not cancel or automatically resubmit it.

## Review candidates before publishing

Run commands from the game worktree root. Install the generator's pinned requirements through uv when invoking it:

```bash
# Inspect a prompt without contacting ComfyUI or writing files.
uv run --python 3.12 --with-requirements scripts/card_image_gen/requirements.txt \
  python -m scripts.card_image_gen.generate_cards --card ES004 --dry-run

# Save one isolated review candidate with a known seed.
uv run --python 3.12 --with-requirements scripts/card_image_gen/requirements.txt \
  python -m scripts.card_image_gen.generate_cards --card ES004 --seed 1001 \
  --candidate-dir output/imagegen/earth-withdrawal-review
```

Candidate mode writes `CARD-seed-SEED/image.png` and `manifest.json` inside the requested directory. It never calls the asset exporter or changes production images or `assets/catalog.json`. Existing candidates cannot be overwritten; use another seed or another review directory. Destinations inside production assets or the frontend are rejected, including symlinks to those directories.

Each manifest preserves the full prompt, seed, exact ComfyUI node graph, server software/device information and PNG SHA-256. Keep an `environment.json` alongside the batch recording the ComfyUI Git revision, installed packages, model repository revisions and model SHA-256 values. Same seed and software inputs support reproduction, but identical bytes across GPU or library changes are not guaranteed.

For a review batch, use explicit card IDs. Compare contact sheets with existing art, then inspect each selected image in its real card crop before promotion. All cards use the original subject, tag hints and deterministic lighting, perspective and palette recipe. Add only short subject overrides for persistent text or subject errors. Selected production images and their generation provenance are recorded in `assets/card-art-generation.json`.

## Production output

Omitting `--candidate-dir` writes directly to `assets/original/cards/CARD.png`, updates the catalog and runs the shared exporter. It can replace existing artwork. Use this only when production replacement is intended:

```bash
uv run --python 3.12 --with-requirements scripts/card_image_gen/requirements.txt \
  python -m scripts.card_image_gen.generate_cards --card 042 --seed 12345
```

To publish an already selected candidate, copy its original PNG to `assets/original/cards/CARD.png`, register it with profile `card` in `assets/catalog.json`, then run `bun run assets` from `frontend/`. Do not regenerate an approved candidate during promotion. The exporter produces the 480- and 960-pixel runtime WebP variants from the master; Node 24+, Bun, frontend dependencies and Git LFS sources are required.

The CLI also accepts `--start ID --end ID` or `--missing`. Missing artwork is determined by both catalog entry and source-file existence. Review the target set with `--dry-run` first; `--missing` can include cards outside the current feature.

Failed jobs report server validation or execution details and cause a nonzero command exit. Successful candidates from a partial batch are preserved. A timed-out job may still run in ComfyUI: inspect its queue before retrying. Nothing is automatically resubmitted.

## Verification

```bash
uv run --python 3.12 --with-requirements scripts/card_image_gen/requirements.txt \
  python -m unittest scripts.card_image_gen.test_card_generation
```

Tests cover candidate isolation and reproduction records, overwrite protection, invalid output, failed workflows, GPU execution errors, timeout identification and nonzero batch failure.
