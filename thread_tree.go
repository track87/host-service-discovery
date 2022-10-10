// Package host_service_discovery declare thread tree
// MarsDong 2022/10/10
package host_service_discovery

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
		tree.Roots = append(tree.Roots, root)
		tree.Nodes[root.ID] = root
		rootSet[root.ID] = struct{}{}
	}

	for _, node := range nodes {
		if _, exists := rootSet[node.ID]; exists {
			continue
		}
		if _, exists := tree.Nodes[node.ID]; !exists {
			tree.Nodes[node.ID] = NewNode(node.ID, node.Parent)
		}
		if _, exists := tree.Nodes[node.Parent]; !exists {
			tree.Nodes[node.Parent] = NewNode(node.ID, node.Parent)
		}
		tree.Nodes[node.Parent].Children = append(tree.Nodes[node.Parent].Children, tree.Nodes[node.ID])
	}
	return tree
}

func (t *Tree) Traverse() Nodes {
	ret := make(Nodes, 0)
	for _, root := range t.Roots {
		ret = append(ret, t.traverseByRoot(root)...)
	}
	return ret
}

func (t *Tree) traverseByRoot(root *Node) (children Nodes) {
	children = append(children, root)
	if len(root.Children) == 0 {
		return children
	}
	for _, node := range root.Children {
		for _, n := range t.traverseByRoot(node) {
			children = append(children, n)
		}

	}
	return children
}
