package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/hex"
	"errors"
	"io"
	"os"
)

func EncryptRaw(plaintext []byte, key []byte, iv []byte) ([]byte, error) {
	//plaintext, err := pkcs7Pad(plaintext, aes.BlockSize)
	//if err != nil {
	//	return nil, err
	//}

	if len(plaintext)%aes.BlockSize != 0 {
		return nil, errors.New("plaintext is not a multiple of the block size")
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	ciphertext := make([]byte, len(plaintext))

	mode := cipher.NewCBCEncrypter(block, iv)
	mode.CryptBlocks(ciphertext, plaintext)

	return ciphertext, nil
}

func Encrypt(args []string, iv []byte) error {
	key, err := hex.DecodeString(args[0])
	if err != nil {
		return err
	}

	file := args[1]

	fd, err := os.Open(file)
	if errors.Is(err, os.ErrNotExist) {
		fd, err = os.Create(file)
		if err != nil {
			return err
		}
	}

	plaintext, err := io.ReadAll(fd)
	if err != nil {
		return err
	}

	ciphertext, err := EncryptRaw(plaintext, key, iv)
	if err != nil {
		return err
	}

	fd, err = os.Create(file + ".enc")
	if err != nil {
		return err
	}

	if _, err := io.Copy(fd, bytes.NewReader(ciphertext)); err != nil {
		return err
	}

	return fd.Close()
}

//// pkcs7Pad right-pads the given byte slice with 1 to n bytes, where
//// n is the block size. The size of the result is x times n, where x
//// is at least 1.
//func pkcs7Pad(b []byte, blocksize int) ([]byte, error) {
//	if blocksize <= 0 {
//		return nil, ErrInvalidBlockSize
//	}
//	if b == nil || len(b) == 0 {
//		return nil, ErrInvalidPKCS7Data
//	}
//	n := blocksize - (len(b) % blocksize)
//	pb := make([]byte, len(b)+n)
//	copy(pb, b)
//	copy(pb[len(b):], bytes.Repeat([]byte{byte(n)}, n))
//	return pb, nil
//}
