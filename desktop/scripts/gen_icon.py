#!/usr/bin/env python3
"""Generate the desktop app icon: a white rounded square with a black checkmark.

Outputs multi-resolution ICO (16/32/48/64/128/256) and a 256 PNG. Minimalist
greyscale to match the desktop client's aesthetic.
"""
import math
from PIL import Image, ImageDraw

SIZE = 256  # base canvas size; downscaled for each resolution

def draw(size: int) -> Image.Image:
    img = Image.new("RGBA", (size, size), (0, 0, 0, 0))
    d = ImageDraw.Draw(img)

    pad = size * 0.06
    radius = size * 0.22
    # White rounded-square background.
    d.rounded_rectangle(
        [pad, pad, size - pad, size - pad],
        radius=radius,
        fill=(255, 255, 255, 255),
    )

    # Black checkmark, stroked polyline.
    lw = max(2, int(size * 0.11))
    # Checkmark points tuned to sit centred with optical balance.
    p1 = (size * 0.30, size * 0.52)
    p2 = (size * 0.45, size * 0.66)
    p3 = (size * 0.72, size * 0.36)
    d.line([p1, p2, p3], fill=(26, 26, 26, 255), width=lw, joint="curve")

    # Round the endpoints slightly for a softer look.
    r = lw / 2
    for p in (p1, p2, p3):
        d.ellipse([p[0]-r, p[1]-r, p[0]+r, p[1]+r], fill=(26, 26, 26, 255))

    return img


def main() -> None:
    sizes = [16, 32, 48, 64, 128, 256]
    images = [draw(s) for s in sizes]
    base = images[-1]
    base.save("internal/platform/windows/icon.png")
    base.save(
        "internal/platform/windows/icon.ico",
        format="ICO",
        sizes=[(s, s) for s in sizes],
    )
    print("generated icon.ico and icon.png")


if __name__ == "__main__":
    main()
