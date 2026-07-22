#!/usr/bin/env python3
"""Generate the Todo Desktop app icon (one-off tool, kept for reproducibility).

Design: warm-black rounded tile + paper-white bold check + amber accent dot,
matching the app's design tokens (#1d1c19 / #f7f6f3 / #b45309).
Rendered at 1024px with 4x supersampling, then downscaled per target size.
"""

import os
from PIL import Image, ImageDraw

HERE = os.path.dirname(os.path.abspath(__file__))

INK = (29, 28, 25, 255)        # --fg / --accent  #1d1c19
PAPER = (247, 246, 243, 255)   # --bg             #f7f6f3
AMBER = (180, 83, 9, 255)      # --warm           #b45309

BASE = 1024


def render(size: int = BASE) -> Image.Image:
    s = size
    img = Image.new("RGBA", (s, s), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    # Rounded-square tile.
    d.rounded_rectangle([0, 0, s - 1, s - 1], radius=int(s * 0.22), fill=INK)

    # Bold check: short arm down-right, long arm up-right.
    w = int(s * 0.115)  # stroke width
    p1 = (s * 0.27, s * 0.54)
    p2 = (s * 0.44, s * 0.70)
    p3 = (s * 0.72, s * 0.33)
    d.line([p1, p2, p3], fill=PAPER, width=w, joint="curve")
    # Round the stroke caps.
    r = w / 2
    for p in (p1, p3):
        d.ellipse([p[0] - r, p[1] - r, p[0] + r, p[1] + r], fill=PAPER)

    # Amber accent dot — sits where a "done" marker would, lower right.
    cx, cy, dr = s * 0.735, s * 0.72, s * 0.075
    d.ellipse([cx - dr, cy - dr, cx + dr, cy + dr], fill=AMBER)
    return img


def save(img: Image.Image, name: str, size: int) -> None:
    out = img.resize((size, size), Image.LANCZOS)
    out.save(os.path.join(HERE, name))
    print(f"{name}: {size}x{size}")


def main() -> None:
    base = render()

    targets = {
        "32x32.png": 32,
        "64x64.png": 64,
        "128x128.png": 128,
        "128x128@2x.png": 256,
        "icon.png": 512,
        "StoreLogo.png": 50,
        "Square30x30Logo.png": 30,
        "Square44x44Logo.png": 44,
        "Square71x71Logo.png": 71,
        "Square89x89Logo.png": 89,
        "Square107x107Logo.png": 107,
        "Square142x142Logo.png": 142,
        "Square150x150Logo.png": 150,
        "Square284x284Logo.png": 284,
        "Square310x310Logo.png": 310,
    }
    for name, size in targets.items():
        save(base, name, size)

    # Multi-size .ico (embedded into the exe by tauri-build).
    base.save(
        os.path.join(HERE, "icon.ico"),
        format="ICO",
        sizes=[(16, 16), (24, 24), (32, 32), (48, 48), (64, 64), (128, 128), (256, 256)],
    )
    print("icon.ico: 16..256")


if __name__ == "__main__":
    main()
