package main

import (
	"fmt"
	"os"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/app"
	"github.com/freemyipod/wInd3x/pkg/exploit/dumpmem"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [offset] [size] [file]",
	Short: "Dump memory to file",
	Long:  "Read memory from a connected device and write results to a file. Not very fast.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := app.New()
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
			glog.Infof("Dumping %x...", o)
			data, err := dumpmem.Trigger(app.Usb, app.Ep, o)
			if err != nil {
				return fmt.Errorf("failed to run wInd3x exploit: %w", err)
			}
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("failed to write: %w", err)
			}
		}
		took := time.Since(start)
		glog.Infof("Done! %d bytes in %d seconds (%d bytes per second)", size, int(took.Seconds()), int(float64(size)/took.Seconds()))

		return nil
	},
}
