package thumber

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/BurntSushi/freetype-go/freetype"
	"github.com/BurntSushi/freetype-go/freetype/truetype"
	"github.com/disintegration/imaging"
	"github.com/sourcegraph/conc/pool"
	"golang.org/x/exp/slices"
	"golang.org/x/exp/slog"
	"golang.org/x/image/draw"

	"github.com/abdusco/thumber/pkg/thumber/internal/fonts"
)

func checkFfmpegInstalled() error {
	if _, err := exec.LookPath("ffmpeg"); err != nil {
		return fmt.Errorf("ffmpeg not installed or not in PATH")
	}
	if _, err := exec.LookPath("ffprobe"); err != nil {
		return fmt.Errorf("ffprobe not installed or not in PATH")
	}

	return nil
}

func extractThumbnail(ctx context.Context, filename string, timestamp time.Duration, width, height int) (Thumbnail, error) {
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
			return Thumbnail{}, fmt.Errorf("failed to run ffmpeg: %w\nstderr=%s", err, string(exitErr.Stderr))
		}
		return Thumbnail{}, fmt.Errorf("failed to run ffmpeg: %w", err)
	}

	img, _, err := image.Decode(bytes.NewReader(output))
	if err != nil {
		return Thumbnail{}, fmt.Errorf("failed to decode image: %w", err)
	}

	return Thumbnail{Image: img, Timestamp: timestamp}, nil
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
	From                time.Duration
	To                  time.Duration
	TileColumns         int
	TileCount           int
	Interval            time.Duration
	TileWidth           int
	TileHeight          int
	OverlayTimestamps   bool
	TimestampBackground color.Color
	Padding             int
}

func ParseColor(hex string) (color.Color, error) {
	switch hex {
	case "transparent":
		return color.Transparent, nil
	}

	// Remove hash symbol if present
	if hex[0] == '#' {
		hex = hex[1:]
	}

	var r, g, b, a int64

	if len(hex) == 6 {
		// RGB only
		r, _ = strconv.ParseInt(hex[0:2], 16, 64)
		g, _ = strconv.ParseInt(hex[2:4], 16, 64)
		b, _ = strconv.ParseInt(hex[4:6], 16, 64)
		a = 255 // Default alpha: 100%
	} else if len(hex) == 8 {
		// RGB + alpha
		r, _ = strconv.ParseInt(hex[0:2], 16, 64)
		g, _ = strconv.ParseInt(hex[2:4], 16, 64)
		b, _ = strconv.ParseInt(hex[4:6], 16, 64)
		a, _ = strconv.ParseInt(hex[6:8], 16, 64)
	} else {
		return color.RGBA{}, fmt.Errorf("invalid hex color string: %s", hex)
	}

	return color.RGBA{R: uint8(r), G: uint8(g), B: uint8(b), A: uint8(a)}, nil
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

type Thumbnail struct {
	image.Image
	Timestamp time.Duration
}

func (t *Thumbnail) overlayTimestamp(r timestampRenderer) error {
	textImg, err := r.Render(formatDuration(t.Timestamp))
	if err != nil {
		return err
	}
	padding := 10 // from the edges of the tile
	x := t.Image.Bounds().Dx() - textImg.Bounds().Dx() - padding
	y := t.Image.Bounds().Dy() - textImg.Bounds().Dy() - padding
	opacity := float64(1)
	t.Image = imaging.Overlay(t.Image, textImg, image.Pt(x, y), opacity)

	return nil
}

type timestampRenderer interface {
	Render(text string) (image.Image, error)
}

type defaultRenderer struct {
	Font            *truetype.Font
	FontSizePt      float64
	BackgroundColor color.Color
	ForegroundColor color.Color
}

func (r defaultRenderer) Render(text string) (image.Image, error) {
	c := freetype.NewContext()
	c.SetFont(r.Font)
	fontSizePx := int(c.PointToFix32(r.FontSizePt)) / 256
	c.SetFontSize(r.FontSizePt)

	tw, th, err := c.MeasureString(text)
	if err != nil {
		return nil, fmt.Errorf("failed to measure string: %w", err)
	}

	// freetype.Fix32 is a fixed-point representation of a number with 16 bits of precision for the fractional part.
	// To convert these values to pixels, we need to divide them by 256
	twPx := int(tw) / 256
	thPx := int(th) / 256

	padding := 4
	img := image.NewRGBA(image.Rect(0, 0, twPx+padding, thPx+padding))
	draw.Draw(img, img.Bounds(), image.NewUniform(r.BackgroundColor), image.Point{X: 0, Y: 0}, draw.Src)

	c.SetClip(img.Bounds())
	c.SetDst(img)
	c.SetSrc(image.White)

	x := padding / 2
	// adjust y position by 5% to account for baseline shift
	y := int(math.Ceil(float64(fontSizePx))*0.95) + padding/2
	if _, err := c.DrawString(text, freetype.Pt(x, y)); err != nil {
		return nil, fmt.Errorf("failed to draw string: %w", err)
	}

	return img, nil
}

func MakeThumbnails(ctx context.Context, videoPath string, opts ThumbOptions) ([]Thumbnail, error) {
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

	type indexedThumb struct {
		Thumbnail
		Index int
	}

	p := pool.NewWithResults[indexedThumb]().
		WithContext(ctx).
		WithMaxGoroutines(4).
		WithCollectErrored()

	for i := 0; i < totalTiles; i++ {
		i := i
		p.Go(func(ctx context.Context) (indexedThumb, error) {
			t := start + time.Duration(i)*interval
			slog.Debug("extracting thumbnail", "current", i+1, "total", totalTiles)
			th, err := extractThumbnail(ctx, videoPath, t, opts.TileWidth, opts.TileHeight)
			if err != nil {
				slog.Error("failed to extract thumbnail", "timestamp", t, "error", err)
				return indexedThumb{}, err
			}
			return indexedThumb{Thumbnail: th, Index: i}, nil
		})
	}

	results, err := p.Wait()
	if err != nil {
		return nil, err
	}

	slices.SortFunc(results, func(a, b indexedThumb) bool {
		return a.Index < b.Index
	})

	thumbnails := make([]Thumbnail, 0, len(results))
	for _, r := range results {
		thumbnails = append(thumbnails, r.Thumbnail)
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

	return makeContactSheet(
		thumbs,
		opts,
	), nil
}

func makeContactSheet(thumbs []Thumbnail, opts ThumbOptions) image.Image {
	rows := int(math.Ceil(float64(len(thumbs)) / float64(opts.TileColumns)))

	tileWidth := thumbs[0].Bounds().Dx()
	tileHeight := thumbs[0].Bounds().Dy()

	black := color.RGBA{}
	w := tileWidth*opts.TileColumns + (opts.TileColumns+1)*opts.Padding
	h := tileHeight*rows + (rows+1)*opts.Padding
	canvas := imaging.New(w, h, black)

	renderer := defaultRenderer{
		Font:            fonts.RobotoMonoMedium,
		FontSizePt:      12,
		BackgroundColor: opts.TimestampBackground,
		ForegroundColor: color.White,
	}

	for i, img := range thumbs {
		row := i / opts.TileColumns
		col := i % opts.TileColumns
		x := opts.Padding + col*tileWidth + col*opts.Padding
		y := opts.Padding + row*tileHeight + row*opts.Padding

		if opts.OverlayTimestamps {
			if err := img.overlayTimestamp(renderer); err != nil {
				slog.Error("failed to overlay timestamp text", "timestamp", img.Timestamp, "error", err)
				continue
			}
		}
		canvas = imaging.Paste(canvas, img, image.Pt(x, y))
	}
	return canvas
}
