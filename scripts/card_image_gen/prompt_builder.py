"""Prompt engineering for card image generation."""

import hashlib
import re

BASE_STYLE = (
    "photorealistic sci-fi digital art, high detail, professional concept art, "
    "no text, no words, no lettering, no watermarks, "
    "astronomically accurate, only one sun in the sky, "
    "no duplicate planets or moons, "
    "planets and moons appear small and distant high in the sky"
)

# Rotating variety elements keyed by card ID hash to keep prompts deterministic
# but visually diverse across the full set.
_PERSPECTIVES = [
    "wide panoramic shot",
    "cinematic close-up",
    "dramatic low angle",
    "bird's eye view",
    "first-person perspective",
    "over-the-shoulder view",
    "extreme wide establishing shot",
    "intimate macro detail",
]

_LIGHTING = [
    "golden hour warm lighting",
    "cool blue twilight",
    "harsh dramatic shadows",
    "soft diffused glow",
    "neon-lit cyberpunk ambiance",
    "stark high-contrast chiaroscuro",
    "ethereal backlit silhouette",
    "warm amber firelight",
]

_COLOR_PALETTES = [
    "warm orange and rust tones",
    "cool blue and silver palette",
    "vibrant greens and teals",
    "deep purple and violet hues",
    "fiery red and gold spectrum",
    "muted earth tones",
    "icy white and pale blue",
    "rich amber and bronze",
]

# Short atmospheric hints per tag — kept brief so they don't overpower the card name
TAG_HINTS = {
    "building": "futuristic industrial structures",
    "power": "energy infrastructure",
    "science": "scientific equipment",
    "microbe": "bioluminescent microorganisms",
    "animal": "alien fauna",
    "plant": "lush vegetation",
    "space": "deep space backdrop",
    "earth": "tiny Earth far away high in the sky",
    "jovian": "small distant Jupiter high in the sky",
    "venus": "small distant Venus high in the sky",
    "city": "domed city",
    "mars": "red Martian terrain",
    "moon": "lunar surface",
    "clone": "genetic engineering",
    "wild": "alien wilderness",
    "event": "celestial phenomenon",
}

CARD_TYPE_MOOD = {
    "automated": "industrial scene",
    "active": "dynamic scene",
    "event": "dramatic moment",
    "corporation": "corporate power",
    "prelude": "early colonization",
}

# Override card names that Flux tends to render as visible text.
# Maps card ID → visual description used instead of the card name.
_NAME_OVERRIDES = {
    # Review rejected lettering on signs and abstract names without a clear subject.
    "ES017": "towering radio antenna array transmitting across the Martian landscape",
    "ES021": "cargo shuttle delivering advanced machinery to a Martian landing platform",
    "ES030": "sealed high-security vault inside a Martian orbital station",
    "ES033": "busy Martian colony bazaar, covered stalls and pedestrians beneath domes",
    "066": "territorial marker planted on vast alien landscape, flag on rocky hilltop",
    "068": "corporate funding ceremony on Mars, executives shaking hands",
    "098": "luxurious tropical paradise colony under a glass dome, palm trees and pools",
    "123": "massive industrial manufacturing complex, smokestacks and conveyor belts",
    "145": "geothermal tectonic energy harvesting plant, glowing fissures in the ground",
    "B02": "eco-friendly green corporation campus surrounded by forests",
    "B05": "futuristic invention laboratory, holographic blueprints floating in air",
    "B09": "lightning-powered energy corporation, massive tesla coils crackling with electricity",
    "P05": "biodome ecosystem with thriving plants and wildlife under a protective shell",
    "T02": "exiled political figure walking away from a grand government building",
    "T09": "public relations command center, wall of holographic screens showing broadcasts",
    "T10": "festive crowd gathering at a Martian colony, fireworks in the dusty sky",
    "XC5": "sleek corporate skyscraper overlooking a volcanic Martian mountain at twilight",
    # Cards where name causes text artifacts
    "012": "massive spacecraft hauling water tanks from Europa moon toward distant Mars, icy moon surface below",
    "014": "futuristic research campus with scientists working on blueprints, robotic arms assembling prototypes",
    "026": "vast canyon national park on Mars with hiking trails, terraformed greenery along cliff edges, visitors exploring",
    # Asteroid/comet impact cards — show them hitting the planet, not floating in space
    "009": "massive fiery asteroid slamming into Martian surface, huge explosion and shockwave, debris flying",
    "010": "icy comet crashing into Mars, enormous impact explosion, steam and water erupting from crater",
    "011": "gigantic asteroid impact on Mars, catastrophic explosion, massive dust cloud rising, shockwave visible from orbit",
    "037": "glowing green nitrogen-rich asteroid streaking through Martian atmosphere, blazing trail, impacting surface",
    "075": "icy comet being guided toward Mars by spacecraft, comet entering atmosphere with bright trail",
    "078": "two massive ice asteroids impacting Martian polar region, twin explosions, water ice scattered everywhere",
    "080": "colossal ice asteroid devastating Martian landscape, enormous impact crater forming, tidal wave of meltwater",
    "170": "asteroid with glowing ammonia trail entering Martian atmosphere, fiery reentry, microbe-filled debris",
    "097": "nuclear detonation on Martian surface, massive mushroom cloud rising from blast crater, intense heat shockwave, scorched barren terrain",
    "209": "small asteroid striking Martian terrain, localized explosion, dust plume rising from impact crater",
    "218": "comet plunging into Venus thick atmosphere, brilliant fireball visible from orbit",
    "246": "huge asteroid slamming into Venus surface at oblique angle, enormous explosion, debris cloud engulfing the planet",
    "P19": "metallic asteroid crashing into Mars, molten metal splashing from impact, glowing crater",
    "EXP004": "nuclear detonation on Mars, enormous mushroom cloud rising over Martian desert, shockwave rippling across red terrain, intense blinding flash",
}


# Keywords that indicate a sentence is about game mechanics, not visuals
_MECHANIC_KEYWORDS = re.compile(
    r"(?:production|M€|VP|step|tile|draw|discard|opponent|player|"
    r"terraform rating|global parameter|standard project|requires?\s+\d|"
    r"tag requirement|tag cards|into your hand|from the deck|reveal cards|"
    r"resources?(?:\s+on)?|pay\s+\d|spend\s+\d|place\s+a\s+|"
    r"gain\s+\d|lose\s+\d|reduce|raise|increase|decrease|"
    r"Earth-dependent|eligible project|Action:|Effect:)",
    re.IGNORECASE,
)


def _variety_for_card(card_id: str) -> str:
    """Pick deterministic but varied perspective, lighting, and palette from card ID."""
    h = int(hashlib.md5(card_id.encode()).hexdigest(), 16)
    perspective = _PERSPECTIVES[h % len(_PERSPECTIVES)]
    lighting = _LIGHTING[(h >> 8) % len(_LIGHTING)]
    palette = _COLOR_PALETTES[(h >> 16) % len(_COLOR_PALETTES)]
    return f"{perspective}, {lighting}, {palette}"


def _extract_visual_concepts(description: str) -> str:
    """Strip game-mechanic sentences, keep visually descriptive ones."""
    if not description:
        return ""
    # Remove markdown bold
    text = re.sub(r"\*\*([^*]+)\*\*", r"\1", description)
    # Split into sentences and keep only non-mechanic ones
    sentences = re.split(r"[.!]\s*", text)
    visual = [s.strip() for s in sentences if s.strip() and not _MECHANIC_KEYWORDS.search(s)]
    result = ". ".join(visual)
    if len(result) < 5:
        return ""
    return result[:100]


def build_prompt(card: dict) -> str:
    """Build a generation prompt from card data.

    The card name is the dominant subject. Tags and type provide light
    atmospheric context without overwhelming the scene.
    """
    parts: list[str] = []

    # Card name as primary subject.
    # Use visual overrides for text-prone cards to prevent the model
    # from rendering the card name as visible signage/text.
    name = card["name"]
    card_id = card["id"]
    card_type = card.get("type", "automated")
    if card_id in _NAME_OVERRIDES:
        parts.append(_NAME_OVERRIDES[card_id])
    elif card_type == "corporation":
        parts.append(f"scene inspired by the concept of {name}, unnamed corporation headquarters")
    else:
        parts.append(f"{name}, detailed scene of {name}")

    # Visual concepts from description (short, secondary)
    description = card.get("description", "")
    if isinstance(description, list):
        description = " ".join(
            entry.get("text", "") for entry in description if entry.get("type") == "generic"
        )
    concepts = _extract_visual_concepts(description)
    if concepts:
        parts.append(concepts)

    # Light tag hints (just atmosphere, not full scene descriptions)
    tags = card.get("tags", [])
    hints = [TAG_HINTS[t] for t in tags if t in TAG_HINTS]
    if hints:
        parts.append(", ".join(hints))

    # Card type mood
    card_type = card.get("type", "automated")
    if card_type in CARD_TYPE_MOOD:
        parts.append(CARD_TYPE_MOOD[card_type])

    # Visual variety per card
    parts.append(_variety_for_card(card["id"]))

    # Consistent base style
    parts.append(BASE_STYLE)

    return ", ".join(parts)
