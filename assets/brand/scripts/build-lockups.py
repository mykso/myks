# /// script
# requires-python = ">=3.11,<3.14"  # uharfbuzz has no free-threaded 3.14t build
# dependencies = ["fonttools", "uharfbuzz"]
# ///
"""Builds the outlined myks lockups in ../logo/ and the social card in ../social/ from the Outfit font.

Run from anywhere: uv run assets/brand/scripts/build-lockups.py
"""

import hashlib
import math
import pathlib
import urllib.request

import uharfbuzz as hb
from fontTools.pens.boundsPen import BoundsPen
from fontTools.pens.svgPathPen import SVGPathPen
from fontTools.pens.transformPen import TransformPen

# Pinned so a rebuild gives the same outlines.
FONT_URL = (
    "https://raw.githubusercontent.com/google/fonts/"
    "8b0a1d0f5983c89bc2b93f1b5fb55f9e252744b5/ofl/outfit/Outfit%5Bwght%5D.ttf"
)
FONT_SHA256 = "fc7287273e66929776e2ba54f144fe699080bec29f61bf649d70d871468aeade"

WORD = "myks"
WEIGHT = 600
TRACKING = -0.035  # em
# Every length below is in mark units: the mark's viewBox is 100, and in the
# horizontal lockup the font size equals the mark height, so 1em = 100.
EM = 100
GAP = 0.16 * EM  # mark box to the wordmark's first glyph origin
STACK_MARK = 1.5  # stacked lockup: mark box size, in em
STACK_GAP = 0.22 * EM  # stacked lockup: mark ink to wordmark ink

# (radius, path) of each arc; all arcs are centred on (50, 50).
ARCS = [
    (36, "M50 14 A36 36 0 1 1 16.17 62.31"),
    (24, "M70.78 62 A24 24 0 1 1 54.17 26.36"),
    (12, "M39.61 56 A12 12 0 1 1 59.19 57.71"),
]
STROKE = 9

PALETTES = {
    "on-dark": {"arcs": ["#FF5A4E", "#FFB020", "#A27BFF"], "word": "#FFFFFF"},
    "on-light": {"arcs": ["#E8443A", "#C77700", "#7B3FF2"], "word": "#1C1030"},
    "mono": {"arcs": ["currentColor"] * 3, "word": "currentColor"},
}

# Social card: GitHub's recommended social preview size, in px.
CARD_W, CARD_H = 1280, 640
CARD_BG = "#1C1030"
CARD_MARK = 1.7  # mark height in px per mark unit
TAGLINE = "Reviewable manifests for every cluster"  # the docs hero title
TAGLINE_SIZE = 44
TAGLINE_GAP = 56  # lockup ink to tagline ink

BRAND = pathlib.Path(__file__).resolve().parent.parent
OUT = BRAND / "logo"


def fetch_font() -> bytes:
    data = urllib.request.urlopen(FONT_URL).read()
    digest = hashlib.sha256(data).hexdigest()
    if digest != FONT_SHA256:
        raise SystemExit(f"font checksum mismatch: {digest}")
    return data


def mark_bounds():
    """Ink bounds of the stroked mark: arc extremes padded by half the stroke (round caps)."""
    xs, ys = [], []
    for r, d in ARCS:
        v = [float(n) for n in d.replace("M", "").replace("A", "").split()]
        x0, y0, x1, y1 = v[0], v[1], v[7], v[8]
        a0 = math.atan2(y0 - 50, x0 - 50)
        a1 = math.atan2(y1 - 50, x1 - 50)
        if a1 <= a0:
            a1 += math.tau  # sweep-flag 1 runs towards increasing angles
        for i in range(721):
            a = a0 + (a1 - a0) * i / 720
            xs.append(50 + r * math.cos(a))
            ys.append(50 + r * math.sin(a))
    h = STROKE / 2
    return min(xs) - h, min(ys) - h, max(xs) + h, max(ys) + h


def outline(font_data: bytes, text: str, weight: int, tracking: float, size: float):
    """Returns text as one SVG path (origin at the first glyph, baseline y=0) and its ink bounds."""
    face = hb.Face(hb.Blob(font_data))
    font = hb.Font(face)
    font.set_variations({"wght": weight})
    buf = hb.Buffer()
    buf.add_str(text)
    buf.guess_segment_properties()
    hb.shape(font, buf, {"kern": True, "liga": True})
    assert len(buf.glyph_infos) == len(text), f"unexpected shaping result for {text!r}"
    assert all(i.codepoint for i in buf.glyph_infos), f"font lacks a glyph of {text!r}"

    scale = size / face.upem
    svg, bounds = SVGPathPen(None, ntos=lambda v: f"{v:.2f}".rstrip("0").rstrip(".")), BoundsPen(None)
    x = 0.0
    for info, pos in zip(buf.glyph_infos, buf.glyph_positions):
        matrix = (scale, 0, 0, -scale, x + pos.x_offset * scale, -pos.y_offset * scale)
        for pen in (svg, bounds):
            font.draw_glyph_with_pen(info.codepoint, TransformPen(pen, matrix))
        x += pos.x_advance * scale + tracking * size
    return svg.getCommands(), bounds.bounds


def num(v: float) -> str:
    return f"{v:.2f}".rstrip("0").rstrip(".")


def svg(view, mark_transform, word_transform, word_d, colors) -> str:
    x0, y0, x1, y1 = view
    arcs = "".join(f'<path d="{d}" stroke="{c}"/>' for (_, d), c in zip(ARCS, colors["arcs"]))
    mt = f' transform="{mark_transform}"' if mark_transform else ""
    wt = f' transform="{word_transform}"' if word_transform else ""
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="{num(x0)} {num(y0)} {num(x1 - x0)} {num(y1 - y0)}"'
        ' role="img" aria-label="myks"><title>myks</title>'
        f'<g fill="none" stroke-width="{STROKE}" stroke-linecap="round"{mt}>{arcs}</g>'
        f'<path{wt} d="{word_d}" fill="{colors["word"]}"/></svg>\n'
    )


def card(font_data, ink, word_h, word_d) -> str:
    """The social card: the on-dark horizontal lockup over the tagline, centred on plum."""
    ix0, iy0, ix1, iy1 = ink
    tag_d, (tx0, ty0, tx1, ty1) = outline(font_data, TAGLINE, 500, -0.01, TAGLINE_SIZE)
    k = CARD_MARK
    top = (CARD_H - ((iy1 - iy0) * k + TAGLINE_GAP + ty1 - ty0)) / 2
    lx, ly = CARD_W / 2 - (ix0 + ix1) / 2 * k, top - iy0 * k
    tag_y = top + (iy1 - iy0) * k + TAGLINE_GAP - ty0
    arcs = "".join(f'<path d="{d}" stroke="{c}"/>' for (_, d), c in zip(ARCS, PALETTES["on-dark"]["arcs"]))
    return (
        f'<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 {CARD_W} {CARD_H}" width="{CARD_W}" height="{CARD_H}"'
        f' role="img" aria-label="myks: {TAGLINE}"><title>myks: {TAGLINE}</title>'
        '<defs><radialGradient id="glow"><stop offset="0" stop-color="#FF5A4E" stop-opacity="0.22"/>'
        '<stop offset="1" stop-color="#FF5A4E" stop-opacity="0"/></radialGradient></defs>'
        f'<rect width="{CARD_W}" height="{CARD_H}" fill="{CARD_BG}"/>'
        f'<circle cx="{num(lx + 50 * k)}" cy="{num(ly + 50 * k)}" r="{num(110 * k)}" fill="url(#glow)"/>'
        f'<g transform="translate({num(lx)} {num(ly)}) scale({k})">'
        f'<g fill="none" stroke-width="{STROKE}" stroke-linecap="round">{arcs}</g>'
        f'<path transform="{word_h}" d="{word_d}" fill="{PALETTES["on-dark"]["word"]}"/></g>'
        f'<path transform="translate({num(CARD_W / 2 - (tx0 + tx1) / 2)} {num(tag_y)})" d="{tag_d}" fill="#C9C1D8"/>'
        "</svg>\n"
    )


def main():
    font_data = fetch_font()
    word_d, (wx0, wy0, wx1, wy1) = outline(font_data, WORD, WEIGHT, TRACKING, EM)
    mx0, my0, mx1, my1 = mark_bounds()
    pad = 1  # keeps antialiased edges inside the viewBox

    # Horizontal: mark box 0..100, wordmark ink centred on the mark ink.
    tx = EM + GAP
    ty = (my0 + my1) / 2 - (wy0 + wy1) / 2
    horizontal = (mx0 - pad, min(my0, wy0 + ty) - pad, wx1 + tx + pad, max(my1, wy1 + ty) + pad)
    word_h = f"translate({num(tx)} {num(ty)})"

    # Stacked: mark scaled up and centred over the wordmark.
    s = STACK_MARK
    mcx = (wx0 + wx1) / 2 - 50 * s
    mcy = wy0 - STACK_GAP - my1 * s
    stacked = (
        min(wx0, mx0 * s + mcx) - pad,
        my0 * s + mcy - pad,
        max(wx1, mx1 * s + mcx) + pad,
        wy1 + pad,
    )
    mark_s = f"translate({num(mcx)} {num(mcy)}) scale({s})"

    for name, colors in PALETTES.items():
        files = {
            f"myks-logo-{name}.svg": svg(horizontal, "", word_h, word_d, colors),
            f"myks-logo-stacked-{name}.svg": svg(stacked, mark_s, "", word_d, colors),
        }
        for file, text in files.items():
            assert "<text" not in text
            (OUT / file).write_text(text)
            print(OUT / file)

    ink = (mx0, min(my0, wy0 + ty), wx1 + tx, max(my1, wy1 + ty))
    social = BRAND / "social" / "myks-social.svg"
    social.parent.mkdir(exist_ok=True)
    social.write_text(card(font_data, ink, word_h, word_d))
    print(social)


if __name__ == "__main__":
    main()
