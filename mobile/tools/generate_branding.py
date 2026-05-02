"""Generate launcher/splash assets from the Material Icons `receipt_long` glyph.

Outputs (all 1024x1024 PNG) to mobile/assets/branding/:
  - app_icon.png             white glyph centered on teal #009688 (legacy + iOS)
  - app_icon_foreground.png  white glyph centered on transparent (Android adaptive
                             foreground; padded so the glyph fits the adaptive
                             safe zone of ~66% of the canvas)
  - splash_icon.png          white glyph centered on transparent (native splash)
"""

from pathlib import Path

import cairosvg

ROOT = Path(__file__).resolve().parents[1]
OUT = ROOT / "assets" / "branding"
OUT.mkdir(parents=True, exist_ok=True)

TEAL = "#009688"
SIZE = 1024

# Material Icons receipt_long path data (24x24 viewBox), fetched from
# https://fonts.gstatic.com/s/i/materialicons/receipt_long/v9/24px.svg
ICON_PATHS = """
<path d="M19.5,3.5L18,2l-1.5,1.5L15,2l-1.5,1.5L12,2l-1.5,1.5L9,2L7.5,3.5L6,2v14H3v3
  c0,1.66,1.34,3,3,3h12c1.66,0,3-1.34,3-3V2L19.5,3.5z M19,19c0,0.55-0.45,1-1,1s-1-0.45-1-1v-3H8V5h11V19z"/>
<rect height="2" width="6" x="9" y="7"/>
<rect height="2" width="2" x="16" y="7"/>
<rect height="2" width="6" x="9" y="10"/>
<rect height="2" width="2" x="16" y="10"/>
"""


def build_svg(background: str | None, glyph_scale: float) -> str:
    """Compose a 1024x1024 SVG with the receipt_long glyph centered.

    glyph_scale is the fraction of the canvas occupied by the 24x24 glyph
    bounding box. 0.6 mimics typical Android safe-zone padding.
    """
    bg = (
        f'<rect width="{SIZE}" height="{SIZE}" fill="{background}"/>'
        if background
        else ""
    )
    glyph_size = SIZE * glyph_scale
    offset = (SIZE - glyph_size) / 2
    scale = glyph_size / 24
    return f"""<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="{SIZE}" height="{SIZE}" viewBox="0 0 {SIZE} {SIZE}">
  {bg}
  <g transform="translate({offset},{offset}) scale({scale})" fill="white">
    {ICON_PATHS}
  </g>
</svg>"""


def render(svg: str, name: str) -> None:
    out = OUT / name
    cairosvg.svg2png(
        bytestring=svg.encode("utf-8"),
        output_width=SIZE,
        output_height=SIZE,
        write_to=str(out),
    )
    print(f"wrote {out.relative_to(ROOT)}")


def main() -> None:
    render(build_svg(TEAL, glyph_scale=0.55), "app_icon.png")
    render(build_svg(None, glyph_scale=0.55), "app_icon_foreground.png")
    render(build_svg(None, glyph_scale=0.55), "splash_icon.png")


if __name__ == "__main__":
    main()
