package main

import (
	"fmt"
	"log"
	"os"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/exploit/dumpmem"
)

var dumpCmd = &cobra.Command{
	Use:   "dump [offset] [size] [file]",
	Short: "Dump memory to file",
	Long:  "Read memory from a connected device and write results to a file. Not very fast.",
	Args:  cobra.ExactArgs(3),
	RunE: func(cmd *cobra.Command, args []string) error {
		app, err := newApp()
		if err != nil {
			return err
		}
		defer app.close()

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

		for i := uint32(0); i < size; i += 0x40 {
			o := offset + i
			log.Printf("Dumping %x...", o)
			data, err := dumpmem.Trigger(app.usb, app.ep, o)
			if err != nil {
				return fmt.Errorf("failed to run wInd3x exploit: %w", err)
			}
			if _, err := f.Write(data); err != nil {
				return fmt.Errorf("failed to write: %w", err)
			}
		}
		log.Printf("Done!")

		return nil
	},
}
