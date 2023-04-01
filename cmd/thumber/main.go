package main

import (
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/kong"
	"golang.org/x/exp/slog"

	"github.com/abdusco/thumber/pkg/thumber"
	"github.com/abdusco/thumber/version"
)

func main() {
	var args cliArgs
	cliCtx := kong.Parse(
		&args,
		kong.Name("thumber"),
		kong.Vars{"version": version.Version.String()},
	)

	logLevel := slog.LevelInfo
	if args.Debug {
		logLevel = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.HandlerOptions{Level: logLevel}.NewTextHandler(os.Stderr)))

	if err := cliCtx.Run(); err != nil {
		log.Fatal(err)
	}
}

type cliArgs struct {
	Version           kong.VersionFlag `help:"Show version and exit"`
	VideoPath         string           `arg:"" help:"Path to video"`
	OutputPath        string           `short:"o" help:"Output path to save JPEG, use - for stdout. Defaults to $filename.thumbs.jpg"`
	From              Duration         `default:"10" help:"Starting point in seconds, 11h22m33s or mm:ss or hh:mm:ss format"`
	To                Duration         `help:"Stopping point"`
	TileWidth         int              `default:"540" help:"Tile width in px"`
	TileHeight        int              `help:"Tile height in px, optional"`
	Columns           int              `default:"3" help:"Columns of tile grid"`
	IntervalSeconds   int              `default:"60" help:"Interval between tiles in seconds"`
	JPEGQuality       int              `name:"quality" default:"80" help:"JPEG quality"`
	Padding           int              `help:"Padding around tiles in px"`
	OverlayTimestamps bool             `help:"Overlay timestamp on each tile"`
	OverlayBackground string           `help:"Timestamp background color as RGB or RGBA hex color or \"transparent\" e.g. #FFF59D" default:"transparent"`
	Debug             bool             `help:"Enable verbose logging"`
}

func (a cliArgs) Run() error {
	from, err := a.From.Duration()
	if err != nil {
		return fmt.Errorf("invalid from: %w", err)
	}

	to, err := a.To.Duration()
	if err != nil {
		return fmt.Errorf("invalid to: %w", err)
	}

	color, err := thumber.ParseColor(a.OverlayBackground)
	if err != nil {
		return fmt.Errorf("invalid overlay background color: %w", err)
	}

	opts := thumber.ThumbOptions{
		From:                from,
		To:                  to,
		TileColumns:         a.Columns,
		Interval:            time.Second * time.Duration(a.IntervalSeconds),
		TileWidth:           a.TileWidth,
		TileHeight:          a.TileHeight,
		Padding:             a.Padding,
		OverlayTimestamps:   a.OverlayTimestamps,
		TimestampBackground: color,
	}
	slog.Debug("parsed options", "options", opts)

	img, err := thumber.Generate(context.Background(), a.VideoPath, opts)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnails: %w", err)
	}

	f, err := a.OutputFile()
	if err != nil {
		return fmt.Errorf("failed to open file for writing: %w", err)
	}

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: a.JPEGQuality}); err != nil {
		return fmt.Errorf("failed to encode as jpeg: %w", err)
	}
	return nil
}

func (a cliArgs) OutputFile() (io.Writer, error) {
	if a.OutputPath == "-" {
		return os.Stdout, nil
	}

	if a.OutputPath == "" {
		dir := filepath.Dir(a.VideoPath)
		base := strings.TrimSuffix(filepath.Base(a.VideoPath), filepath.Ext(a.VideoPath))
		a.OutputPath = filepath.Join(dir, fmt.Sprintf("%s.thumbs.jpg", base))
	}

	return os.Create(a.OutputPath)
}

type Duration string

func (d Duration) Duration() (time.Duration, error) {
	if d == "" {
		return 0, nil
	}

	if d, err := time.ParseDuration(string(d)); err == nil {
		return d, nil
	}

	parts := reverse(strings.Split(strings.TrimSpace(string(d)), ":"))

	var hour, minute, second int
	var err error

	if len(parts) > 2 {
		hour, err = strconv.Atoi(parts[2])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %q as hour", parts[0])
		}
	}

	if len(parts) > 1 {
		minute, err = strconv.Atoi(parts[1])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %q as minute", parts[1])
		}
	}

	if len(parts) > 0 {
		second, err = strconv.Atoi(parts[0])
		if err != nil {
			return 0, fmt.Errorf("invalid duration: %q as second", parts[second])
		}
	}

	duration := time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(second)*time.Second
	return duration, nil
}

func reverse[T any](s []T) []T {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}
