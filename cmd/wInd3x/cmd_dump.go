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
	Long:  "Read memory from a connected device and write results to a file. Not very fast.",
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
		for i := uint32(0); i < size; i += 0x40 {
			o := offset + i
			slog.Info("Dumping...", "offset", o)
			data, err := dumpmem.Trigger(app.Usb, app.Ep, o)
			if err != nil {
				return fmt.Errorf("failed to run wInd3x exploit: %w", err)
			}
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("failed to write: %w", err)
			}
		}
		took := time.Since(start)
		slog.Info("Done!", "bytes", size, "seconds", int(took.Seconds()), "bps", int(float64(size)/took.Seconds()))

		return nil
	},
}
