# thumber

A mini utility/library that generates contact sheets for videos using ffmpeg.

## Usage

```shell
Usage: thumber <video-path>

Arguments:
  <video-path>    Path to video

Flags:
  -h, --help                   Show context-sensitive help.
  -o, --output-path=STRING     Output path to save JPEG, use - for stdout
      --from="10"              Starting point in seconds, 11h22m33s or mm:ss or hh:mm:ss format
      --to=DURATION            Stopping point
      --tile-width=540         Tile width in px
      --tile-height=INT        Tile height in px, optional
      --columns=3              Columns of tile grid
      --interval-seconds=60    Interval between tiles in seconds
      --padding=INT            Padding around tiles in px
      --overlay-timestamps     Overlay timestamp on each tile
      --debug                  Enable verbose logging
```