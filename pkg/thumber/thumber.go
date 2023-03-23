package thumber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/freetype-go/freetype"
	"github.com/BurntSushi/freetype-go/freetype/truetype"
	"github.com/disintegration/imaging"
	"golang.org/x/exp/slog"

	"github.com/abdusco/thumber/pkg/thumber/internal/fonts"
)

func drawTimestamp(font *truetype.Font, timestamp string) (image.Image, error) {

	c := freetype.NewContext()
	c.SetFont(font)
	c.SetDPI(72)
	fontSize := float64(12)
	fontSizePx := int(c.PointToFix32(fontSize)) / 256
	c.SetFontSize(fontSize)

	tw, th, err := c.MeasureString(timestamp)
	if err != nil {
		return nil, fmt.Errorf("failed to measure string: %w", err)
	}

	// freetype.Fix32 is a fixed-point representation of a number with 16 bits of precision for the fractional part. To convert these values to pixels, we need to divide them by 256
	twPx := int(tw) / 256
	thPx := int(th) / 256

	padding := 4
	img := image.NewRGBA(image.Rect(0, 0, twPx+padding, thPx+padding))
	draw.Draw(img, img.Bounds(), image.Black, image.Point{X: 0, Y: 0}, draw.Src)

	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.White)

	x := padding / 2
	// adjust y position by 5% to account for baseline shift
	y := int(math.Ceil(float64(fontSizePx))*0.95) + padding/2
	if _, err := c.DrawString(timestamp, freetype.Pt(x, y)); err != nil {
		return nil, fmt.Errorf("failed to draw string: %w", err)
	}

	return img, nil
}

func checkFfmpegInstalled() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not installed or not in PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not installed or not in PATH")
	}

	return nil
}

func extractThumbnail(ctx context.Context, filename string, timestamp time.Duration, width, height int) (image.Image, error) {
	if width == 0 {
		width = -1
	} else if height == 0 {
		height = -1
	}

	cmd := exec.CommandContext(
		ctx,
		"ffmpeg",
		"-ss", fmt.Sprintf("%dms", timestamp.Milliseconds()),
		"-i", filename,
		"-vf", fmt.Sprintf("scale=%d:%d", width, height),
		"-vframes", "1",
		"-q:v", "1",
		"-f", "image2",
		"pipe:1",
	)

	output, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return nil, fmt.Errorf("failed to run ffmpeg: %w\nstderr=%s", err, string(exitErr.Stderr))
		}
		return nil, fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("failed to decode image: %w", err)
	}

	return img, nil
}

func readDuration(ctx context.Context, videoPath string) (time.Duration, error) {
	cmd := exec.CommandContext(
		ctx,
		"ffprobe",
		"-v", "error",
		"-show_entries",
		"format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		videoPath,
	)

	out, err := cmd.Output()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return 0, fmt.Errorf("failed to run ffprobe: %w\nstderr=%s", err, string(exitErr.Stderr))
		}
		return 0, fmt.Errorf("failed to run ffprobe: %w", err)
	}

	seconds, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse seconds: %w", err)
	}

	return time.Second * time.Duration(seconds), nil
}

type ThumbOptions struct {
	From              time.Duration
	To                time.Duration
	TileColumns       int
	TileCount         int
	Interval          time.Duration
	TileWidth         int
	TileHeight        int
	OverlayTimestamps bool
	Padding           int
}

func (o ThumbOptions) Validate() error {
	if o.From != 0 && o.To != 0 && o.From > o.To {
		return fmt.Errorf("starting point cannot be after ending point")
	}
	if o.Interval != 0 && o.TileCount != 0 {
		return fmt.Errorf("interval and tile count cannot be set together")
	}

	return nil
}

func MakeThumbnails(ctx context.Context, videoPath string, opts ThumbOptions) ([]image.Image, error) {
	if err := opts.Validate(); err != nil {
		return nil, fmt.Errorf("invalid options: %w", err)
	}

	if err := checkFfmpegInstalled(); err != nil {
		return nil, err
	}

	duration, err := readDuration(ctx, videoPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read video duration: %w", err)
	}

	duration = duration.Truncate(time.Second)

	start := opts.From
	end := duration
	if opts.To != 0 {
		end = opts.To
	}
	duration = end - start

	if opts.Interval > duration {
		return nil, fmt.Errorf("interval is larger than available video duration %s: %w", duration, err)
	}

	totalTiles := opts.TileCount
	if opts.Interval != 0 {
		if opts.Interval > duration {
			return nil, fmt.Errorf("interval is larger than available video duration %s: %w", duration, err)
		}
		totalTiles = int(duration / opts.Interval)
	}
	interval := duration / time.Duration(totalTiles)

	if interval < time.Second*10 {
		slog.Warn("interval is very small", "interval", interval)
	}

	w := opts.TileWidth
	h := opts.TileHeight

	var thumbnails []image.Image

	for i := 0; i < totalTiles; i++ {
		t := start + time.Duration(i)*interval
		slog.Debug("extracting thumbnail", "current", i+1, "total", totalTiles)
		img, err := extractThumbnail(ctx, videoPath, t, w, h)
		if err != nil {
			slog.Error("failed to extract thumbnail", "timestamp", t, "error", err)
			continue
		}

		if opts.OverlayTimestamps {
			tsImage, err := drawTimestamp(fonts.RobotoMonoMedium, formatDuration(t))
			if err != nil {
				slog.Error("failed to draw timestamp text", "timestamp", t, "error", err)
				continue
			}
			opacity := float64(1)
			img = imaging.Overlay(img, tsImage, image.Pt(img.Bounds().Dx()-tsImage.Bounds().Dx()-10, img.Bounds().Dy()-tsImage.Bounds().Dy()-10), opacity)
		}
		thumbnails = append(thumbnails, img)
	}

	return thumbnails, nil
}

func formatDuration(d time.Duration) string {
	h := d / time.Hour
	d -= h * time.Hour
	m := d / time.Minute
	d -= m * time.Minute
	s := d / time.Second
	return fmt.Sprintf("%02d:%02d:%02d", h, m, s)
}

func Generate(ctx context.Context, videoPath string, opts ThumbOptions) (image.Image, error) {
	thumbs, err := MakeThumbnails(ctx, videoPath, opts)
	if err != nil {
		return nil, fmt.Errorf("failed to make thumbnails: %w", err)
	}
	if len(thumbs) == 0 {
		return nil, fmt.Errorf("generated 0 images")
	}

	return makeContactSheet(thumbs, opts.TileColumns, opts.Padding), nil
}

func makeContactSheet(thumbs []image.Image, columns, padding int) image.Image {
	rows := int(math.Ceil(float64(len(thumbs)) / float64(columns)))

	tw := thumbs[0].Bounds().Dx()
	th := thumbs[0].Bounds().Dy()

	w := tw*columns + (columns+1)*padding
	h := th*rows + (rows+1)*padding
	black := color.RGBA{}
	canvas := imaging.New(w, h, black)

	for i, img := range thumbs {
		row := i / columns
		col := i % columns
		x := padding + col*tw + col*padding
		y := padding + row*th + row*padding

		canvas = imaging.Paste(canvas, img, image.Pt(x, y))
	}
	return canvas
}
