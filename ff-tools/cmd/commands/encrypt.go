package commands

import (
	"crypto/aes"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"github.com/Despire/ff-tools/crypto"
	"github.com/spf13/cobra"
	"io"
)

func encryptCmd() (*cobra.Command, error) {
	cmd := &cobra.Command{
		Use:   "encrypt <key> <data> [IV]",
		Short: "encrypt using CBC-AES without pkcs7 padding",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) < 2 {
				return errors.New("need atleast 2 arguments for encryption")
			}

			if _, err := hex.DecodeString(args[0]); err != nil {
				return err
			}

			iv := make([]byte, aes.BlockSize)
			if _, err := io.ReadFull(rand.Reader, iv); err != nil {
				return err
			}

			if len(args) > 2 {
				prefix, err := hex.DecodeString(args[3])
				if err != nil {
					return err
				}

				copy(iv, prefix)
			}

			return crypto.Encrypt(args, iv)
		},
	}

	return cmd, nil
}
