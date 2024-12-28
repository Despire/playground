package crypto

import (
	"bytes"
	"crypto/aes"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/Despire/ff-tools/algo"
	"io"
	"os"
)

func PdfToWasm(args []string) error {
	key, err := hex.DecodeString(args[0])
	if err != nil {
		return err
	}
	pdf := args[1]
	wasm := args[2]

	pdffd, err := os.Open(pdf)
	if errors.Is(err, os.ErrNotExist) {
		return err
	}

	pdfdata, err := io.ReadAll(pdffd)
	if err != nil {
		return err
	}

	wasmfd, err := os.Open(wasm)
	if err != nil {
		return err
	}

	wasmData, err := io.ReadAll(wasmfd)
	if err != nil {
		return err
	}

	wasmData = append(wasmData, 0x0)
	//wasmData = append(wasmData, algo.ToLEB128(uint64(len("\nendstream\nendobj\n"))+uint64(len(pdfdata)))...)
	wasmData = append(wasmData, algo.ToLEB128(547)...) // this is added manually

	if len(wasmData)%aes.BlockSize != 0 {
		m := aes.BlockSize - (len(wasmData) % aes.BlockSize)
		for m > 0 {
			wasmData = append(wasmData, 0x0)
			m -= 1
		}
	}

	const marker = "%PDF-1obj\nstream"
	iv, err := DecryptPlainTextToIV(wasmData, key, marker)
	if err != nil {
		return err
	}

	fmt.Printf("iv: %x\n", iv)

	ciphertext, err := EncryptRaw(wasmData, key, iv)
	if err != nil {
		return err
	}

	ciphertext = append(ciphertext, "\nendstream\nendobj\n"...)
	ciphertext = append(ciphertext, pdfdata...)

	fd, err := os.Create(fmt.Sprintf("pdfToWasm") + ".stg")
	if err != nil {
		return err
	}

	if _, err := io.Copy(fd, bytes.NewReader(ciphertext)); err != nil {
		return err
	}

	fmt.Printf("doesn't work automatically need to adjust manually")

	return fd.Close()
}

func PngToPdf(args []string) error {
	key, err := hex.DecodeString(args[0])
	if err != nil {
		return err
	}
	png := args[1]
	pdf := args[2]

	pngfd, err := os.Open(png)
	if errors.Is(err, os.ErrNotExist) {
		return err
	}

	pngdata, err := io.ReadAll(pngfd)
	if err != nil {
		return err
	}

	pdffd, err := os.Open(pdf)
	if errors.Is(err, os.ErrNotExist) {
		return err
	}

	pdfdata, err := io.ReadAll(pdffd)
	if err != nil {
		return err
	}

	const marker = "%PDF-1obj\nstream"

	iv, err := DecryptPlainTextToIV(pngdata, key, marker)
	if err != nil {
		return err
	}

	fmt.Printf("iv: %x\n", iv)

	ciphertext, err := EncryptRaw(pngdata, key, iv)
	if err != nil {
		return err
	}

	ciphertext = append(ciphertext, "\nendstream\nendobj\n"...)
	ciphertext = append(ciphertext, pdfdata...)

	fd, err := os.Create(fmt.Sprintf("pngToPdf") + ".stg")
	if err != nil {
		return err
	}

	if _, err := io.Copy(fd, bytes.NewReader(ciphertext)); err != nil {
		return err
	}

	return fd.Close()
}

func xor(a, b []byte) []byte {
	out := make([]byte, aes.BlockSize)
	for i, val := range b {
		if i >= aes.BlockSize {
			break
		}

		out[i] = a[i] ^ val
	}

	return out
}

func DecryptPlainTextToIV(plaintext []byte, key []byte, target string) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	out := make([]byte, aes.BlockSize)
	block.Decrypt(out, []byte(target))
	return xor(out, plaintext[:aes.BlockSize]), nil
}
