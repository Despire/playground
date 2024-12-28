package commands

import "github.com/spf13/cobra"

func rootCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "ff-tools",
		Short: "creating polyglot files/hiding information inside files",
	}

	mergeCmd, err := mergeCmd()
	if err != nil {
		return nil, err
	}

	encryptCmd, err := encryptCmd()
	if err != nil {
		return nil, err
	}

	decryptCmd, err := decryptCmd()
	if err != nil {
		return nil, err
	}

	pngToPdf, err := pngToPdfCmd()
	if err != nil {
		return nil, err
	}

	pdfToWasm, err := pdfToWasmCmd()
	if err != nil {
		return nil, err
	}

	cmd.AddCommand(mergeCmd)
	cmd.AddCommand(encryptCmd)
	cmd.AddCommand(decryptCmd)
	cmd.AddCommand(pngToPdf)
	cmd.AddCommand(pdfToWasm)

	return cmd, nil
}
