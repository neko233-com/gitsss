# Built-in avatars

The 24 PNGs are locally rendered snapshots of DiceBear 10.6.0 styles:

- `00.png`–`11.png`: [Adventurer](https://www.dicebear.com/styles/adventurer/)
  by [Lisa Wischofsky](https://www.instagram.com/lischi_art/),
  [CC BY 4.0](https://creativecommons.org/licenses/by/4.0/).
  Source: https://www.figma.com/community/file/1184595184137881796
- `12.png`–`23.png`: [Lorelei](https://www.dicebear.com/styles/lorelei/)
  by Lisa Wischofsky, [CC0 1.0](https://creativecommons.org/publicdomain/zero/1.0/).
  Source: https://www.figma.com/community/file/1198749693280469639

These are remixes with pastel backgrounds and deterministic seeds, rasterized
to 256px with resvg 2.6.2 on 2026-10-16. `manifest.json` records every asset's
source, license, seed, size and SHA-256. The profile picker and HTML manual also
credit the artist. Artwork licenses remain independent of the application's MIT
license. No DiceBear service or library is required at runtime.

Selection uses the existing avatar storage pipeline. Custom uploads are cropped
to square, downscaled to the configured rendered size without upscaling, and
stored as the smaller suitable PNG/JPEG representation; transparency uses PNG.
Existing avatar files are unchanged. New GIF uploads use their first frame;
unsupported animated WebP uploads show the existing decode error.
