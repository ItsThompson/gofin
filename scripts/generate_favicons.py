# /// script
# requires-python = ">=3.10"
# dependencies = ["Pillow==13.0.0"]
# ///
"""Run: uv run --python 3.13 scripts/generate_favicons.py. Requires rsvg-convert."""

import argparse
from pathlib import Path
import shutil
import subprocess
import tempfile

from PIL import Image

ROOT = Path(__file__).resolve().parents[1]
PUBLIC = ROOT / "frontend/apps/shell/public"
SOURCE = PUBLIC / "favicon/favicon.svg"
BACKGROUND = (255, 255, 255, 255)
PNG_SIZES = {
    "favicon/favicon-16x16.png": 16,
    "favicon/favicon-32x32.png": 32,
    "favicon/apple-touch-icon.png": 180,
    "favicon/android-chrome-192x192.png": 192,
    "favicon/android-chrome-512x512.png": 512,
    "favicon/mstile-150x150.png": 150,
    "favicon.png": 32,
}
ICO_SIZES = [(16, 16), (32, 32), (48, 48)]


def generate_favicons(source: Path, output: Path) -> None:
    renderer = shutil.which("rsvg-convert")
    if renderer is None:
        raise RuntimeError("Install rsvg-convert before generating favicons: brew install librsvg")

    output = output.resolve()
    output.mkdir(parents=True, exist_ok=True)
    with tempfile.TemporaryDirectory(prefix=".favicon-", dir=output) as workspace:
        staging = Path(workspace)
        for filename, size in PNG_SIZES.items():
            destination = staging / filename
            destination.parent.mkdir(parents=True, exist_ok=True)
            subprocess.run(
                [renderer, "--width", str(size), "--height", str(size), str(source), "--output", str(destination)],
                check=True,
            )

        with Image.open(staging / "favicon/android-chrome-512x512.png") as image:
            image.save(staging / "favicon/favicon.ico", format="ICO", sizes=ICO_SIZES)
        shutil.copyfile(staging / "favicon/favicon.ico", staging / "favicon.ico")

        # Validate the entire package before replacing any existing assets.
        for filename, size in PNG_SIZES.items():
            with Image.open(staging / filename) as image:
                rgba = image.convert("RGBA")
                if image.size != (size, size) or rgba.getchannel("A").getextrema() != (255, 255):
                    raise ValueError(f"Incorrect dimensions or transparent pixels: {filename}")
                corners = ((0, 0), (size - 1, 0), (0, size - 1), (size - 1, size - 1))
                if any(rgba.getpixel(corner) != BACKGROUND for corner in corners):
                    raise ValueError(f"Incorrect background color: {filename}")

        with Image.open(staging / "favicon/favicon.ico") as icon:
            if icon.ico.sizes() != set(ICO_SIZES):
                raise ValueError("The ICO must contain 16, 32, and 48 pixel frames")
            for size in ICO_SIZES:
                frame = icon.ico.getimage(size).convert("RGBA")
                width, height = size
                corners = ((0, 0), (width - 1, 0), (0, height - 1), (width - 1, height - 1))
                if frame.getchannel("A").getextrema() != (255, 255):
                    raise ValueError(f"Transparent ICO pixels: {size}")
                if any(frame.getpixel(corner) != BACKGROUND for corner in corners):
                    raise ValueError(f"Incorrect ICO background: {size}")

        filenames = [*PNG_SIZES, "favicon/favicon.ico", "favicon.ico"]
        for filename in filenames:
            destination = output / filename
            destination.parent.mkdir(parents=True, exist_ok=True)
            (staging / filename).replace(destination)

    print(f"Generated {len(filenames)} PNG/ICO assets in {output} from {source.name}.")


if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Render the GoFin weave SVG into the favicon package.")
    parser.add_argument("--output-dir", type=Path, default=PUBLIC)
    args = parser.parse_args()
    generate_favicons(SOURCE, args.output_dir)
