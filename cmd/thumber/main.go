package main

import (
	"context"
	"fmt"
	"image/jpeg"
	"io"
	"log"
	"os"
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
		kong.Vars{"version": version.GitVersion().String()},
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
	Version           kong.VersionFlag
	VideoPath         string   `arg:"" help:"Path to video"`
	OutputPath        string   `short:"o" help:"Output path to save JPEG, use - for stdout"`
	From              Duration `default:"10" help:"Starting point in seconds, 11h22m33s or mm:ss or hh:mm:ss format"`
	To                Duration `help:"Stopping point"`
	TileWidth         int      `default:"540" help:"Tile width in px"`
	TileHeight        int      `help:"Tile height in px, optional"`
	Columns           int      `default:"3" help:"Columns of tile grid"`
	IntervalSeconds   int      `default:"60" help:"Interval between tiles in seconds"`
	Padding           int      `help:"Padding around tiles in px"`
	OverlayTimestamps bool     `help:"Overlay timestamp on each tile"`
	Debug             bool     `help:"Enable verbose logging"`
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

	opts := thumber.ThumbOptions{
		From:              from,
		To:                to,
		TileColumns:       a.Columns,
		Interval:          time.Second * time.Duration(a.IntervalSeconds),
		TileWidth:         a.TileWidth,
		TileHeight:        a.TileHeight,
		Padding:           a.Padding,
		OverlayTimestamps: a.OverlayTimestamps,
	}
	slog.Debug("parsed options", "options", opts)

	img, err := thumber.Generate(context.Background(), a.VideoPath, opts)
	if err != nil {
		return fmt.Errorf("failed to generate thumbnails: %w", err)
	}

	var f io.Writer
	if a.OutputPath == "-" {
		f = os.Stdout
	} else {
		f, err = os.Create(a.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to create file: %w", err)
		}
	}

	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		return fmt.Errorf("failed to encode as jpeg: %w", err)
	}
	return nil
}

type Duration string

func reverse[T comparable](s []T) []T {
	for i, j := 0, len(s)-1; i < j; i, j = i+1, j-1 {
		s[i], s[j] = s[j], s[i]
	}
	return s
}

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
