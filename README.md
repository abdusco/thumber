# thumber

A mini utility/library that generates contact sheets for videos using ffmpeg.

## Requirements

- `ffmpeg` installed and available in `$PATH`.

## Usage

```shell
thumber -o image.jpg --overlay-timestamps video.mp4
```

```shell
Usage: thumber <video-path>

Arguments:
  <video-path>    Path to video

Flags:
  -h, --help                   Show context-sensitive help.
      --version                Show version and exit
  -o, --output-path=STRING     Output path to save JPEG, use - for stdout.
                               Defaults to $filename.thumbs.jpg
      --from="10"              Starting point in seconds, 11h22m33s or mm:ss or
                               hh:mm:ss format
      --to=DURATION            Stopping point
      --tile-width=540         Tile width in px
      --tile-height=INT        Tile height in px, optional
      --columns=3              Columns of tile grid
      --interval-seconds=60    Interval between tiles in seconds
      --quality=80             JPEG quality
      --padding=INT            Padding around tiles in px
      --overlay-timestamps     Overlay timestamp on each tile
      --overlay-background="transparent"
                               Timestamp background color as RGB or RGBA hex
                               color or "transparent" e.g. #FFF59D
      --debug                  Enable verbose logging

```
