# WorldWeaver Hackathon Demo Video

This folder contains the complete submission-video workspace.

## Final output

- `final/worldweaver-hackathon-demo.mp4` — corrected 1920×1080 submission video, approximately 2:03, H.264/AAC
- `final/worldweaver-hackathon-demo-poster.png` — thumbnail/poster frame

The video includes real gameplay captured from the local Docker Compose stack, proportionally correct multiplayer evidence from two browser clients, generated visual art for the story cards, and one continuous Daniel narration track with padded scene transitions.

## Re-render

From the project root, start the stack first:

```bash
docker compose up -d
```

Then from this folder:

```bash
npm install
npx playwright install chromium
npm run capture
./render.sh
```

`capture.mjs` records the real app into `captures/` and `build/gameplay.webm`. It uses full-size 1920×1080 clients, pointer movement and drag gestures, and waits for the simulation to react. `render.sh` generates the narration, overlays typography on the generated raster art, builds the segments, and re-encodes the final MP4 to avoid hard audio cuts.

The narration uses macOS Daniel voice synthesis at a slower, cleaner pace. The padded WAV files in `build/audio/` can be replaced with a human voice recording before rerunning the render.
