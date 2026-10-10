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


# Earth Withdrawal subjects are explicit so card rules never become image lettering.
_NAME_OVERRIDES.update(
{'ES001': 'Licensed fabrication robots processing Martian ore beneath an Earth telemetry '
          'display.',
 'ES002': 'Priority cargo capsules approaching a Martian freight terminal.',
 'ES003': 'Local greenhouses managed through remote environmental-control consoles.',
 'ES004': 'Analysts using an Earth-hosted scientific platform from a Martian laboratory.',
 'ES005': 'Modular generators with remote maintenance links and local cooling equipment.',
 'ES006': 'A dispatcher allocating limited cargo capacity between industrial and biological '
          'supplies.',
 'ES007': 'A construction project displaying its Terran financing authorization.',
 'ES008': 'Remote guidance coordinating a large Martian terraforming operation.',
 'ES009': 'Service crews bringing nursery plants and equipment to a newly completed settlement.',
 'ES010': 'Delegates presenting an independence petition in a Martian assembly chamber.',
 'ES011': 'Local engineers taking control of launch infrastructure.',
 'ES012': 'A local administrative office adjoining its research archive.',
 'ES013': 'A human settlement executive closes an Earth subsidy authorization terminal inside a '
          'pressurized Martian office. Local factories beyond the window, no floating paperwork, '
          'no aliens.',
 'ES014': 'Two human diplomats in normal formal clothing inside a pressurized Martian meeting '
          'room exchange a secure data case to extend an infrastructure cooperation agreement. '
          'No aliens, no planet in the sky.',
 'ES015': 'Replacement relay panels and salvaged components at a service dock.',
 'ES016': 'An extended service agreement beside a freight terminal operating through the night.',
 'ES017': 'A powerful communications array drawing electricity from newly installed generators.',
 'ES018': 'Sealed material lockers awaiting the final release authorization.',
 'ES019': 'A reserve nursery opening beneath an independently powered greenhouse.',
 'ES020': 'Researchers preserving technical libraries in rugged offline storage.',
 'ES041': 'A distant relay realigning its antenna toward Earth.',
 'ES042': 'Settlement antennas synchronized into a temporary ground array.',
 'ES043': 'A brief transmission unlocking pre-positioned supply manifests.',
 'ES044': 'A transmission mast drawing emergency power, its cooling radiators glowing at night.',
 'ES021': 'Engineers exchange a complete technical archive beside an imported turbine inside a '
          'Martian habitat.',
 'ES022': 'Two scientists collaboratively inspect a locally manufactured prototype and abstract '
          'technical visualization in a Martian laboratory.',
 'ES023': 'Industrial engineers exchange precision tooling and a steel casting at a glowing '
          'Martian foundry.',
 'ES024': 'A specialist arrives at a Martian airlock carrying rugged data cases, welcomed by '
          'local engineers.',
 'ES025': 'An urgent compact cargo capsule descends toward a Martian landing pad carrying '
          'precision machine-control modules.',
 'ES026': 'A bioreactor synthesizes greenhouse nutrients from local Martian minerals, green '
          'culture vessels and carefully maintained pipework.',
 'ES027': 'Researchers operate a locally mirrored technical archive, rugged storage racks and '
          'dim cool screens in a Martian laboratory.',
 'ES028': 'Industrial waste-heat recovery pipes and heat exchangers distribute warmth to Martian '
          'greenhouses, amber thermal glow.',
 'ES029': 'A local electrically powered parts foundry staffed by Martian engineers, precision '
          'metal components and small heat-recovery pipes.',
 'ES030': 'A secure vault inside a Martian settlement contains sealed reserve funds in robust '
          'anonymous cases, quiet warm security lighting.',
 'ES031': 'Delivered emergency supplies in sealed titanium and steel cargo bays, a Martian '
          'warehouse with strong depth and overhead light.',
 'ES032': 'A control-room team rehearses isolated operation of a Martian settlement, focused '
          'operators and abstract diagnostic displays.',
 'ES033': 'A domestic industrial trade fair inside a Martian habitat, locally made tools and '
          'materials exchanged by workers.',
 'ES034': 'An intersettlement freight market exchanges local steel and titanium, compact '
          'transport rovers beside organized cargo stacks.',
 'ES035': 'An autonomous furnace maintained by Martian technicians, robotic handling arms and '
          'bright molten steel within an enclosed local foundry.',
 'ES036': 'An independent research campus on Mars studies mineral and atmospheric samples, '
          'practical scientific equipment and sunlit red terrain beyond.',
 'ES037': 'Botanists maintain an independent greenhouse network, healthy plants and moisture '
          'suspended in warm habitat light.',
 'ES038': 'A municipal power-dispatch center connects illuminated Martian domed settlements, '
          'energy infrastructure crossing dark red terrain.',
 'ES039': 'Certified local scientific records stored beside research instruments in a meticulous '
          'Martian archive, no readable writing.',
 'ES040': 'Martian industrial representatives sign a local commercial charter around a circular '
          'table, independent workshops visible beyond, no readable writing.',
 'EP01': 'An early Martian settlement receives imported aerospace tooling, pallets of steel '
         'beside precise titanium-processing equipment.',
 'EP02': 'The first settlement tooling workshop, a modest well-stocked workbench with steel '
         'components and engineering instruments.',
 'EP03': 'Locally held emergency reserves sealed in an early Mars settlement vault, anonymous '
         'resource containers and reliable locks.',
 'EP04': 'Settlement founders plan self-sufficiency beside a newly installed local generator, '
         'collaborative practical work and red Martian terrain.'}
)

_CORPORATION_LOGOS = {'EC01': 'Use case: logo-brand. Standalone landscape 2:1 futuristic corporation wordmark. Exact '
         'text in three lines: TERRAN / SUPPLY / AUTHORITY. Bold cream geometric angular '
         'lettering with engineered cutouts on near-black charcoal. Electric blue orbital '
         'cargo-link emblem integrated to the left. Flat graphic mark, precise margins, legible '
         'at small size. No scenery, buildings, headquarters, subtitles or watermark.',
 'EC02': 'Use case: logo-brand. Standalone landscape 2:1 futuristic corporation wordmark. Exact '
         'text in three lines: MARTIAN / INDEPENDENCE / COOPERATIVE. Bold cream geometric '
         'angular lettering on near-black charcoal. Rust-copper accent, geometric interlocking '
         'hexagonal industrial emblem integrated with the lettering. Strong retro-futurist '
         'space-industry identity, flat graphic mark, precise margins, legible at small size. No '
         'scenery, buildings, headquarters, subtitles or watermark.'}

# Keywords that indicate a sentence is about game mechanics, not visuals
_MECHANIC_KEYWORDS = re.compile(
    r"(?:production|M€|VP|step|tile|draw|discard|opponent|player|"
    r"terraform rating|global parameter|standard project|requires?\s+\d|"
    r"tag requirement|tag cards|into your hand|from the deck|reveal cards|"
    r"resources?(?:\s+on)?|pay\s+\d|spend\s+\d|place\s+a\s+|"
    r"gain\s+\d|lose\s+\d|reduce|raise|increase|decrease|"
    r"Action:|Effect:)",
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
    if card["id"] in _CORPORATION_LOGOS:
        return _CORPORATION_LOGOS[card["id"]]

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
        description = " ".join(entry.get("text", "") for entry in description)
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
