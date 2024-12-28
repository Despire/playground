package merkle

import (
	"bytes"
	"crypto/sha512"
	"errors"
)

// Tree is an implementation of https://en.wikipedia.org/wiki/Merkle_tree
type Tree struct {
	Root   *Node
	Leaves []*Node
}

// Node represents a node in the merkle tree.
type Node struct {
	// Pointers to respective siblings.
	Parent *Node
	Left   *Node
	Right  *Node
	// Hash is the hash of the children if the type is not Leaf, otherwise is the hash of the contents.
	Hash []byte
}

// NewTree constructs a new MerkleTree from the given values.
func NewTree(values [][]byte) *Tree {
	root, leaves := construct(values)
	return &Tree{
		Root:   root,
		Leaves: leaves,
	}
}

func construct(values [][]byte) (*Node, []*Node) {
	leaves := make([]*Node, len(values))

	for i := range leaves {
		h := sha512.Sum512(values[i])

		leaves[i] = &Node{
			Hash: h[:],
		}
	}

	if len(leaves)%2 == 1 {
		h := sha512.Sum512(values[len(values)-1])

		leaves = append(leaves, &Node{
			Hash: h[:],
		})
	}

	return root(leaves), leaves
}

func root(queue []*Node) *Node {
	for len(queue) != 1 {
		left, right := queue[0], queue[1]
		queue = queue[2:]

		h := sha512.Sum512(append(left.Hash, right.Hash...))

		node := &Node{
			Parent: nil,
			Left:   left,
			Right:  right,
			Hash:   h[:],
		}

		left.Parent = node
		right.Parent = node

		queue = append(queue, node)
	}

	return queue[0]
}

// Verify rebuilds the tree and verifies the integrity.
func (tree *Tree) Verify() bool {
	if tree == nil {
		return false
	}

	root := root(append([]*Node(nil), tree.Leaves...))
	return bytes.Equal(tree.Root.Hash, root.Hash)
}

// VerifyProof verifies that h part of the merkle tree.
func (tree *Tree) VerifyProof(h []byte, path []PathPoint) bool {
	result := h

	for _, point := range path {
		if point.Appended {
			tmp := sha512.Sum512(append(result, point.Hash...))
			result = tmp[:]
		} else {
			tmp := sha512.Sum512(append(point.Hash, result...))
			result = tmp[:]
		}
	}

	return bytes.Equal(tree.Root.Hash, result)
}

type PathPoint struct {
	Hash     []byte
	Appended bool
}

// Proof builds a proof for a MerkleTree
func (tree *Tree) Proof(h []byte) ([]PathPoint, error) {
	if tree == nil {
		return nil, errors.New("empty tree")
	}

	current := findNodeWithHash(tree.Leaves, h)
	if current == nil {
		return nil, errors.New("no node with such hash")
	}

	var path []PathPoint

	// collect all the siblings until the root is reached.
	parent := current.Parent
	for parent != nil {
		if current == parent.Left {
			path = append(path, PathPoint{
				Hash:     parent.Right.Hash,
				Appended: true,
			})
		} else {
			path = append(path, PathPoint{
				Hash:     parent.Left.Hash,
				Appended: false,
			})
		}

		current = parent
		parent = current.Parent
	}

	return path, nil
}

func findNodeWithHash(nodes []*Node, h []byte) *Node {
	for _, n := range nodes {
		if bytes.Equal(n.Hash, h) {
			return n
		}
	}
	return nil
}
