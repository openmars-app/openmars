"""Configuration constants for card image generation."""

from pathlib import Path

# ComfyUI settings
COMFYUI_URL = "http://127.0.0.1:8188"

# Model files (relative to ComfyUI models folder)
UNET_MODEL = "flux1-schnell-fp8-e4m3fn.safetensors"
VAE_MODEL = "flux-vae-bf16.safetensors"
CLIP_L_MODEL = "clip_l.safetensors"
T5XXL_MODEL = "t5xxl_fp8_e4m3fn.safetensors"

# Project paths
PROJECT_ROOT = Path(__file__).resolve().parents[2]
CARD_JSON_PATH = PROJECT_ROOT / "backend" / "assets" / "cards.json"
OUTPUT_DIR = PROJECT_ROOT / "assets" / "original" / "cards"
CATALOG_PATH = PROJECT_ROOT / "assets" / "catalog.json"

# Image specifications
IMAGE_WIDTH = 960
IMAGE_HEIGHT = 720

# Flux Schnell generation parameters
STEPS = 4
CFG = 1.0
SAMPLER = "euler"
SCHEDULER = "simple"
GUIDANCE = 3.5

# Batch settings
BATCH_DELAY = 1.0  # Delay between generations in seconds
