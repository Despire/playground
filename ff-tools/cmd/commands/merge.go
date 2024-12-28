package commands

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/Despire/ff-tools/formats"
	"github.com/spf13/cobra"
)

func mergeCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "merge",
		Short: "merge two files to create a polyglot",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 2 {
				return errors.New("need exactly two arguments where each of them is a file")
			}

			var file1 formats.FormatChecker
			{
				b, err := os.ReadFile(args[0])
				if err != nil {
					return err
				}

				file1, err = formats.Find(b)
				if err != nil {
					return err
				}
			}

			var file2 formats.FormatChecker
			{
				b, err := os.ReadFile(args[1])
				if err != nil {
					return err
				}

				file2, err = formats.Find(b)
				if err != nil {
					return err
				}
			}

			out, err := formats.Combine(file1, file2)
			if err != nil {
				return err
			}

			for i, o := range out {
				b, _ := io.ReadAll(o)
				if err := os.WriteFile(fmt.Sprintf("%d-combined.%s.%s", i, file1.Format().String(), file2.Format().String()), b, os.ModePerm); err != nil {
					return err
				}
			}

			return nil
		},
	}

	return cmd, nil
}
