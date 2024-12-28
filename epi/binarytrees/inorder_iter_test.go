// Copyright (c) 2019 Matúš Mrekaj.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//    http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License

package binarytrees

import (
	"testing"
)

func TestIterativeInOrder(t *testing.T) {
	tests := []struct {
		root *Node
		want []int
	}{
		{
			root: &Node{
				Data: 2,
				Left: &Node{
					Data: 1,
					Left: &Node{
						Data:  1,
						Left:  nil,
						Right: nil,
					},
					Right: &Node{
						Data:  2,
						Left:  nil,
						Right: nil,
					},
				},
				Right: &Node{
					Data:  2,
					Left:  nil,
					Right: nil,
				},
			},
			want: []int{1, 1, 2, 2, 2},
		},
		{
			root: &Node{
				Data:  3,
				Left:  nil,
				Right: nil,
			},
			want: []int{3},
		},
		{
			root: nil,
			want: nil,
		},
		{
			root: &Node{
				Data: 3,
				Left: &Node{
					Data: 5,
					Left: &Node{
						Data: 10,
						Left: &Node{
							Data: 15,
						},
					},
				},
			},
			want: []int{15, 10, 5, 3},
		},
		{
			root: &Node{
				Data: 0,
				Left: &Node{
					Data: -3,
					Left: &Node{
						Data: -12,
					},
					Right: &Node{
						Data: -8,
					},
				},
				Right: &Node{
					Data: -5,
					Left: &Node{
						Data: -1,
						Left: &Node{
							Data: -7,
						},
					},
					Right: &Node{
						Data: -4,
						Left: &Node{
							Data: 5,
						},
						Right: &Node{
							Data: 3,
							Left: &Node{
								Data: 10,
							},
						},
					},
				},
			},
			want: []int{-12, -3, -8, 0, -7, -1, -5, 5, -4, 10, 3},
		},
		{
			root: &Node{
				Data: 1,
				Left: &Node{
					Data: -2,
				},
				Right: &Node{
					Data: 0,
				},
			},
			want: []int{-2, 1, 0},
		},
		{
			root: &Node{
				Data: -3,
			},
			want: []int{-3},
		},
		{
			root: &Node{
				Data: 6,
				Left: &Node{
					Data: -9,
					Left: &Node{
						Data: 0,
					},
					Right: &Node{
						Data: -5,
					},
				},
				Right: &Node{
					Data: 6,
				},
			},
			want: []int{0, -9, -5, 6, 6},
		},
		{
			root: &Node{
				Data: 2,
				Left: &Node{
					Data: 1516,
					Left: &Node{
						Data: 0,
						Left: &Node{
							Data: -6,
						},
					},
				},
			},
			want: []int{-6, 0, 1516, 2},
		},
		{
			root: &Node{
				Data: -2,
				Left: &Node{
					Data: -3,
					Left: &Node{
						Data: 1,
					},
				},
				Right: &Node{
					Data: -4,
					Left: &Node{
						Data: 0,
					},
				},
			},
			want: []int{1, -3, -2, 0, -4},
		},
	}

	for _, test := range tests {
		got := IterativeInorder(test.root)
		for i, node := range got {
			if node.Data.(int) != test.want[i] {
				t.Errorf("result mismatch. got:%v; want:%v", node.Data.(int), test.want[i])
			}
		}
	}
}
