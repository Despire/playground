package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
)

func DecryptRaw(ciphertext []byte, key []byte, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	if len(ciphertext) < aes.BlockSize {
		return nil, errors.New("ciphertext too short")
	}

	if len(ciphertext)%aes.BlockSize != 0 {
		return nil, errors.New("ciphertext is not a multiple of the block size")
	}

	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(ciphertext, ciphertext)
	//ciphertext, err = pkcs7Unpad(ciphertext, aes.BlockSize)
	//if err != nil {
	//	return nil, err
	//}

	return ciphertext, nil
}

func Decrypt(args []string, iv []byte) error {
	key, err := hex.DecodeString(args[0])
	if err != nil {
		return err
	}
	file := args[1]

	fd, err := os.Open(file)
	if err != nil {
		return err
	}

	ciphertext, err := io.ReadAll(fd)
	if err != nil {
		return err
	}

	ciphertext, err = DecryptRaw(ciphertext, key, iv)
	if err != nil {
		return err
	}

	fmt.Printf("%s\n", ciphertext)
	return nil
}

//// pkcs7Unpad validates and unpads data from the given bytes slice.
//// The returned value will be 1 to n bytes smaller depending on the
//// amount of padding, where n is the block size.
//func pkcs7Unpad(b []byte, blocksize int) ([]byte, error) {
//	if blocksize <= 0 {
//		return nil, ErrInvalidBlockSize
//	}
//	if b == nil || len(b) == 0 {
//		return nil, ErrInvalidPKCS7Data
//	}
//	if len(b)%blocksize != 0 {
//		return nil, ErrInvalidPKCS7Padding
//	}
//	c := b[len(b)-1]
//	n := int(c)
//	if n == 0 || n > len(b) {
//		return nil, ErrInvalidPKCS7Padding
//	}
//	for i := 0; i < n; i++ {
//		if b[len(b)-n+i] != c {
//			return nil, ErrInvalidPKCS7Padding
//		}
//	}
//	return b[:len(b)-n], nil
//}
