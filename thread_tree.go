// Package host_service_discovery declare thread tree
// MarsDong 2022/10/10
package host_service_discovery

import (
	"fmt"
)

type ITree interface {
	Traverse() Nodes
}

// Tree thread Tree
type Tree struct {
	Roots []*Node
	Nodes map[int32]*Node
}

// Node thread Tree Node
type Node struct {
	Parent   int32
	ID       int32
	Children []*Node
}

type Nodes []*Node

func (n Nodes) GetIDs() []int32 {
	ret := make([]int32, 0)
	for _, node := range n {
		ret = append(ret, node.ID)
	}
	return ret
}

func (n Nodes) Set() map[int32]*Node {
	ret := make(map[int32]*Node)
	for _, node := range n {
		ret[node.ID] = node
	}
	return ret
}

func NewNode(id, parent int32) *Node {
	return &Node{
		ID:       id,
		Parent:   parent,
		Children: make([]*Node, 0),
	}
}

func NewTree(roots Nodes, nodes Nodes) *Tree {
	tree := &Tree{
		Roots: make(Nodes, 0),
		Nodes: make(map[int32]*Node),
	}

	rootSet := make(map[int32]struct{})
	for _, root := range roots {
		rootNode := NewNode(root.ID, root.Parent)
		tree.Roots = append(tree.Roots, rootNode)
		tree.Nodes[root.ID] = rootNode
		rootSet[root.ID] = struct{}{}
	}

	nodeSet := nodes.Set()
	for _, node := range nodes {
		if _, exists := rootSet[node.ID]; exists {
			continue
		}
		if _, exists := tree.Nodes[node.ID]; !exists {
			tree.Nodes[node.ID] = NewNode(node.ID, node.Parent)
		}
		if _, exists := tree.Nodes[node.Parent]; !exists {
			parentNode, exists := nodeSet[node.Parent]
			if !exists {
				continue
			}
			tree.Nodes[parentNode.ID] = NewNode(parentNode.ID, parentNode.Parent)
		}
		tree.Nodes[node.Parent].Children = append(tree.Nodes[node.Parent].Children, tree.Nodes[node.ID])
	}
	return tree
}

func (t *Tree) Traverse() Nodes {
	ret := make(Nodes, 0)
	for _, root := range t.Roots {
		nodes := t.traverseByRoot(root)
		ret = append(ret, nodes...)
		debug := make([]int32, 0)
		for _, node := range nodes {
			debug = append(debug, node.ID)
		}
		fmt.Println(debug)
	}
	return ret
}

func (t *Tree) traverseByRoot(root *Node) (children Nodes) {
	children = append(children, root)
	if len(root.Children) == 0 {
		return children
	}
	for _, node := range root.Children {
		//for _, n := range t.traverseByRoot(node) {
		//	children = append(children, n)
		//}
		children = append(children, t.traverseByRoot(node)...)

	}
	return children
}
