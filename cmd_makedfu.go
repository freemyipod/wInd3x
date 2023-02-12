package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/devices"
	"github.com/freemyipod/wInd3x/pkg/image"
)

var (
	makeDFUEntrypoint string
	makeDFUDeviceKind string
)
var makeDFUCmd = &cobra.Command{
	Use:   "makedfu [input] [output]",
	Short: "Build 'haxed dfu' unsigned image from binary",
	Long:  "Wraps a flat binary (loadable at 0x2200_0000) into an unsigned and unencrypted DFU image, to use with haxdfu/run.",
	Args:  cobra.ExactArgs(2),
	RunE: func(cmd *cobra.Command, args []string) error {
		data, err := os.ReadFile(args[0])
		if err != nil {
			return fmt.Errorf("could not read input: %w", err)
		}

		var kind devices.Kind
		switch strings.ToLower(makeDFUDeviceKind) {
		case "":
			return fmt.Errorf("--kind must be set (one of: n3g, n4g, n5g)")
		case "n3g":
			kind = devices.Nano3
		case "n4g":
			kind = devices.Nano4
		case "n5g":
			kind = devices.Nano5
		default:
			return fmt.Errorf("--kind must be one of: n3g, n4g, n5g")
		}

		entrypoint, err := parseNumber(makeDFUEntrypoint)
		if err != nil {
			return fmt.Errorf("invalid entrypoint")
		}
		wrapped, err := image.MakeUnsigned(kind, entrypoint, data)
		if err != nil {
			return fmt.Errorf("could not make image: %w", err)
		}

		if err := os.WriteFile(args[1], wrapped, 0600); err != nil {
			return fmt.Errorf("could not write image: %w", err)
		}

		return nil
	},
}
