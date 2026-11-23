from pathlib import Path
import subprocess
import tempfile
import unittest

from PIL import Image

from generate_favicons import SOURCE, generate_favicons


class GenerateFaviconsTest(unittest.TestCase):
    def test_renders_the_complete_opaque_black_and_white_package(self):
        expected_pngs = {
            "favicon/favicon-16x16.png": 16,
            "favicon/favicon-32x32.png": 32,
            "favicon/apple-touch-icon.png": 180,
            "favicon/android-chrome-192x192.png": 192,
            "favicon/android-chrome-512x512.png": 512,
            "favicon/mstile-150x150.png": 150,
            "favicon.png": 32,
        }
        expected_frames = {(16, 16), (32, 32), (48, 48)}
        with tempfile.TemporaryDirectory() as workspace:
            output = Path(workspace)
            generate_favicons(SOURCE, output)

            self.assertEqual(
                {str(path.relative_to(output)) for path in output.rglob("*") if path.is_file()},
                {*expected_pngs, "favicon/favicon.ico", "favicon.ico"},
            )
            for filename, size in expected_pngs.items():
                with self.subTest(filename=filename), Image.open(output / filename) as image:
                    rgba = image.convert("RGBA")
                    self.assertEqual(image.size, (size, size))
                    self.assertEqual(rgba.getchannel("A").getextrema(), (255, 255))
                    for corner in ((0, 0), (size - 1, 0), (0, size - 1), (size - 1, size - 1)):
                        self.assertEqual(rgba.getpixel(corner), (255, 255, 255, 255))
                    self.assertEqual(rgba.getchannel("R").tobytes(), rgba.getchannel("G").tobytes())
                    self.assertEqual(rgba.getchannel("G").tobytes(), rgba.getchannel("B").tobytes())
                    self.assertLess(rgba.getchannel("R").getextrema()[0], 64)

            for filename in ("favicon.ico", "favicon/favicon.ico"):
                with Image.open(output / filename) as icon:
                    self.assertEqual(icon.ico.sizes(), expected_frames)
                    for size in expected_frames:
                        frame = icon.ico.getimage(size).convert("RGBA")
                        self.assertEqual(frame.size, size)
                        self.assertEqual(frame.getchannel("A").getextrema(), (255, 255))
                        self.assertEqual(frame.getpixel((0, 0)), (255, 255, 255, 255))
            self.assertEqual((output / "favicon.ico").read_bytes(), (output / "favicon/favicon.ico").read_bytes())

    def test_rejects_transparency_and_wrong_background_without_replacing_assets(self):
        for background in ("none", "#F4EFE2"):
            with self.subTest(background=background), tempfile.TemporaryDirectory() as workspace:
                directory = Path(workspace)
                source = directory / "source.svg"
                source.write_text(
                    '<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96">'
                    f'<rect width="96" height="96" fill="{background}"/>'
                    '<circle cx="48" cy="48" r="20" fill="black"/></svg>'
                )
                output = directory / "output"
                output.mkdir()
                previous = {"favicon.png": b"previous PNG", "favicon.ico": b"previous ICO"}
                for filename, content in previous.items():
                    (output / filename).write_bytes(content)

                with self.assertRaises(ValueError):
                    generate_favicons(source, output)

                self.assertEqual({path.name for path in output.iterdir()}, set(previous))
                for filename, content in previous.items():
                    self.assertEqual((output / filename).read_bytes(), content)

    def test_renderer_failure_preserves_existing_assets_and_cleans_staging(self):
        with tempfile.TemporaryDirectory() as workspace:
            directory = Path(workspace)
            output = directory / "output"
            output.mkdir()
            existing = output / "favicon.ico"
            existing.write_bytes(b"previous ICO")

            with self.assertRaises(subprocess.CalledProcessError):
                generate_favicons(directory / "missing.svg", output)

            self.assertEqual(existing.read_bytes(), b"previous ICO")
            self.assertEqual(list(output.iterdir()), [existing])


if __name__ == "__main__":
    unittest.main()
