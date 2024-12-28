package commands

import (
	"errors"
	"github.com/Despire/ff-tools/crypto"
	"github.com/spf13/cobra"
)

func pngToPdfCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "pngToPdf <key> <png> <pdf>",
		Short: "encrypt png such that it decrypts to pdf",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return errors.New("need 3 arguments for pngToPdf")
			}

			return crypto.PngToPdf(args)
		},
	}

	return cmd, nil
}

func pdfToWasmCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "pdfToWasm <key> <pdf> <wasm>",
		Short: "encrypt pdf such that it decrypts to wasm",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 3 {
				return errors.New("need 3 arguments for pdfToWasm")
			}

			return crypto.PdfToWasm(args)
		},
	}

	return cmd, nil
}
