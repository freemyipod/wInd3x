package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/golang/glog"
	"github.com/spf13/cobra"

	"github.com/freemyipod/wInd3x/pkg/mse"
)

var extractDir string

var mseCmd = &cobra.Command{
	Use:   "mse",
	Short: "Manipulate .mse firmware files",
}

var mseExtractCmd = &cobra.Command{
	Use:   "extract",
	Short: "Extract an .mse firmware flie into images",
	Long:  "Split an .mse file into individual images like osos, disk, etc.",
	Args:  cobra.ExactArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		f, err := os.Open(args[0])
		if err != nil {
			return fmt.Errorf("could not read input: %w", err)
		}

		defer f.Close()
		m, err := mse.Parse(f)
		if err != nil {
			return fmt.Errorf("could not parse .mse: %w", err)
		}

		dir := extractDir
		if dir == "" {
			dir, err = os.Getwd()
			if err != nil {
				return fmt.Errorf("could not get working directory: %w", err)
			}
		}
		for _, file := range m.Files {
			if !file.Header.Valid() {
				continue
			}
			path := filepath.Join(dir, file.Header.Name.String())
			glog.Infof("Extracting %s ...", path)
			if err := os.WriteFile(path, file.Data, 0666); err != nil {
				return err
			}
		}

		return nil
	},
}
