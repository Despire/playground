package main

import (
	"crypto/sha512"
	"fmt"
	"github.com/Despire/merkle-tree/merkle"
)

func main() {
	tree := merkle.NewTree([][]byte{
		[]byte("x1"),
		[]byte("x2"),
		[]byte("x3"),
		[]byte("x4"),
	})

	// obtain proof for x2
	block := sha512.Sum512([]byte("x2"))

	proof, err := tree.Proof(block[:])
	if err != nil {
		panic(err)
	}

	fmt.Println(tree.Verify())
	fmt.Println(tree.VerifyProof(block[:], proof))

	// obtain proof for x5
	block = sha512.Sum512([]byte("x5"))
	_, err = tree.Proof(block[:])
	fmt.Println(err) // no node with such hash
}
