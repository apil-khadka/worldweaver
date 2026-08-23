#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "$0")" && pwd)"
cd "$ROOT"

mkdir -p build/audio build/segments final
FONT="/Library/Fonts/SF-Pro-Text-Medium.otf"
W=1920
H=1080

audio_duration() {
  ffprobe -v error -show_entries format=duration -of csv=p=0 "$1"
}

echo "Preparing a clean multiplayer proof frame..."
# Both source captures are 16:9 at 1920x1080. Fit the second client into a
# proportional picture-in-picture; never force a width/height resize.
magick captures/09-player-one.png -resize 1920x1080 -gravity center -extent 1920x1080 \
  \( captures/08-player-two.png -resize 720x405 -bordercolor '#7af2e7' -border 5 \) \
  -gravity southeast -geometry +82+82 -composite -strip build/multiplayer-pip.png

echo "Generating a continuous, padded narration track with Daniel..."
for scene in 01-hook 02-gameplay 03-multiplayer 04-architecture 05-kiro 06-close; do
  say -v Daniel -r 165 -o "build/audio/${scene}.aiff" -f "narration/${scene}.txt"
  raw_duration="$(audio_duration "build/audio/${scene}.aiff")"
  padded_duration="$(awk -v d="$raw_duration" 'BEGIN { printf "%.3f", d + 0.55 }')"
  ffmpeg -y -loglevel error -i "build/audio/${scene}.aiff" \
    -af "aresample=48000,apad=pad_dur=0.55,pan=stereo|c0=c0|c1=c0" \
    -t "$padded_duration" -c:a pcm_s16le "build/audio/${scene}.wav"
done

make_card_segment() {
  local image="$1" audio="$2" output="$3" title="$4" subtitle="$5"
  local card_image="${output%.mp4}.png"
  magick "$image" -resize "${W}x${H}^" -gravity center -extent "${W}x${H}" \
    -fill '#000000a0' -draw 'rectangle 100,100 1090,360' \
    -font "$FONT" -gravity northwest -pointsize 74 -fill white -annotate +150+145 "$title" \
    -pointsize 34 -fill '#7af2e7' -annotate +154+250 "$subtitle" -strip "$card_image"
  ffmpeg -y -loglevel error -loop 1 -i "$card_image" -i "$audio" \
    -vf "scale=${W}:${H}:force_original_aspect_ratio=decrease,pad=${W}:${H}:(ow-iw)/2:(oh-ih)/2,format=yuv420p" \
    -map 0:v:0 -map 1:a:0 -r 30 -c:v libx264 -preset medium -crf 18 -tune stillimage \
    -c:a aac -b:a 192k -ar 48000 -ac 2 -shortest -movflags +faststart "$output"
}

echo "Building real interaction and product-story segments..."
ffmpeg -y -loglevel error -stream_loop -1 -i build/gameplay.webm -i build/audio/02-gameplay.wav \
  -map 0:v:0 -map 1:a:0 \
  -vf "scale=${W}:${H}:force_original_aspect_ratio=decrease,pad=${W}:${H}:(ow-iw)/2:(oh-ih)/2,format=yuv420p,fps=30" \
  -c:v libx264 -preset medium -crf 18 -c:a aac -b:a 192k -ar 48000 -ac 2 -shortest \
  -movflags +faststart build/segments/02-gameplay.mp4

make_card_segment assets/hero-generated-v2.png build/audio/01-hook.wav build/segments/01-hook.mp4 \
  "WORLDWEAVER" "ONE WORLD. MANY FORCES."
make_card_segment build/multiplayer-pip.png build/audio/03-multiplayer.wav build/segments/03-multiplayer.mp4 \
  "REAL-TIME WORLD" "SECOND PLAYER. SAME LANDSCAPE."
make_card_segment assets/architecture-generated-v2.png build/audio/04-architecture.wav build/segments/04-architecture.mp4 \
  "AUTHORITATIVE SIMULATION" "ONE SERVER. MANY BROWSERS."
make_card_segment assets/workflow-generated-v2.png build/audio/05-kiro.wav build/segments/05-kiro.mp4 \
  "FROM SPEC TO SHIP" "PLANNED. BUILT. VERIFIED."
make_card_segment assets/hero-generated-v2.png build/audio/06-close.wav build/segments/06-close.mp4 \
  "SHIP A WORLD" "WORLDWEAVER // KIRO HACKATHON"

printf "file '%s'\n" \
  "$ROOT/build/segments/01-hook.mp4" \
  "$ROOT/build/segments/02-gameplay.mp4" \
  "$ROOT/build/segments/03-multiplayer.mp4" \
  "$ROOT/build/segments/04-architecture.mp4" \
  "$ROOT/build/segments/05-kiro.mp4" \
  "$ROOT/build/segments/06-close.mp4" > build/concat.txt

echo "Concatenating with a fresh encode to remove audio hard-cuts..."
ffmpeg -y -loglevel error -f concat -safe 0 -i build/concat.txt \
  -c:v libx264 -preset medium -crf 18 -pix_fmt yuv420p -r 30 \
  -c:a aac -b:a 192k -ar 48000 -ac 2 -movflags +faststart \
  final/worldweaver-hackathon-demo-v2.mp4

ffmpeg -y -loglevel error -ss 00:00:03 -i final/worldweaver-hackathon-demo-v2.mp4 -frames:v 1 \
  -q:v 2 final/worldweaver-hackathon-demo-v2-poster.png

echo "Final video: $ROOT/final/worldweaver-hackathon-demo-v2.mp4"
ffprobe -v error -show_entries format=duration,size \
  -show_entries stream=codec_name,width,height,codec_type,sample_rate,channels \
  -of default=noprint_wrappers=1 final/worldweaver-hackathon-demo-v2.mp4
