package main

import (
	"fmt"
	"log/slog"
	"os"
	"time"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/exploit/dumpmem"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [offset] [size] [file]",
	Short: "Dump memory to file",
	Long:  "Read memory from a connected device and write results to a file.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newDFU()
		if err != nil {
			return err
		}
		defer app.Close()

		offset, err := parseNumber(args[0])
		if err != nil {
			return fmt.Errorf("invalid offset")
		}
		size, err := parseNumber(args[1])
		if err != nil {
			return fmt.Errorf("invalid size")
		}

		f, err := os.Create(args[2])
		if err != nil {
			return fmt.Errorf("could not open file for writing: %w", err)
		}
		defer f.Close()

		start := time.Now()
		lastReport := start
		progress := func(p float32) {
			now := time.Now()
			if now.Sub(lastReport) < 500*time.Millisecond && p < 1.0 {
				return
			}
			lastReport = now
			elapsed := now.Sub(start)
			done := uint32(float32(size) * p)
			if p > 0 && p < 1.0 {
				eta := time.Duration(float64(elapsed) / float64(p) * float64(1.0-p))
				slog.Info("Dumping...", "offset", fmt.Sprintf("0x%x", offset+done),
					"percent", fmt.Sprintf("%.1f%%", p*100),
					"bps", int(float64(done)/elapsed.Seconds()),
					"eta", fmt.Sprintf("%ds", int(eta.Seconds())))
			}
		}

		if err := dumpmem.DumpRange(app.Usb, app.Ep, offset, size, f, progress); err != nil {
			return err
		}

		took := time.Since(start)
		slog.Info("Done!", "bytes", size, "seconds", int(took.Seconds()), "bps", int(float64(size)/took.Seconds()))

		return nil
	},
}
