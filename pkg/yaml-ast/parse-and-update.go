// [AI Generated]
// This package is generated with help from AI.
// Only goal is to able to patch YAML AST nodes with JSON patches, because i want to preserver user comments in nixy.yml configuration

package yaml_ast

import (
	"fmt"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Parser handles parsing a YAML document into an AST node tree,
// allowing programmatic updates via JSON Patches.
type Parser struct {
	rootNode *yaml.Node
}

// PatchOp represents a single JSON Patch operation (RFC 6902).
type PatchOp struct {
	Op    string `json:"op"`
	Path  string `json:"path"`
	From  string `json:"from,omitempty"`
	Value any    `json:"value,omitempty"`
}

// NewParser parses the given YAML byte slice and returns a Parser instance.
func NewParser(b []byte) (*Parser, error) {
	var rootNode yaml.Node
	if err := yaml.Unmarshal(b, &rootNode); err != nil {
		return nil, err
	}
	return &Parser{rootNode: &rootNode}, nil
}

// Decode decodes the internal AST into the provided value structure.
func (p *Parser) Decode(v any) error {
	return p.rootNode.Decode(v)
}

// Root returns the underlying AST document node.
func (p *Parser) Root() *yaml.Node {
	return p.rootNode
}

// ApplyPatches applies a list of PatchOp operations sequentially to the YAML AST.
func (p *Parser) ApplyPatches(ops []PatchOp) error {
	for i, op := range ops {
		if err := p.applyOp(op); err != nil {
			return fmt.Errorf("patch op %d (%s at %s) failed: %w", i, op.Op, op.Path, err)
		}
	}
	return nil
}

func (p *Parser) applyOp(op PatchOp) error {
	parts, err := parseJSONPointer(op.Path)
	if err != nil {
		return err
	}

	// Helper to convert Go value to yaml.Node
	toNode := func(val any) (*yaml.Node, error) {
		if val == nil {
			return &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!null", Value: "null"}, nil
		}
		b, err := yaml.Marshal(val)
		if err != nil {
			return nil, err
		}
		var node yaml.Node
		if err := yaml.Unmarshal(b, &node); err != nil {
			return nil, err
		}
		if node.Kind == yaml.DocumentNode && len(node.Content) > 0 {
			return node.Content[0], nil
		}
		return &node, nil
	}

	switch op.Op {
	case "test":
		target, err := resolvePath(p.rootNode, parts)
		if err != nil || target.TargetNode == nil {
			return fmt.Errorf("path %q not found for test", op.Path)
		}
		expected, err := toNode(op.Value)
		if err != nil {
			return err
		}
		if !equalNodes(target.TargetNode, expected) {
			return fmt.Errorf("value mismatch at %q", op.Path)
		}

	case "remove":
		target, err := resolvePath(p.rootNode, parts)
		if err != nil || target.Parent == nil {
			return fmt.Errorf("path %q not found for remove", op.Path)
		}
		return removeNode(target)

	case "add", "replace":
		target, err := resolvePath(p.rootNode, parts)
		if err != nil {
			return err
		}
		if op.Op == "replace" && target.TargetNode == nil {
			return fmt.Errorf("cannot replace non-existent path %q", op.Path)
		}
		valNode, err := toNode(op.Value)
		if err != nil {
			return err
		}
		return addOrReplaceNode(target, valNode)

	case "move":
		fromParts, err := parseJSONPointer(op.From)
		if err != nil {
			return err
		}
		fromTarget, err := resolvePath(p.rootNode, fromParts)
		if err != nil || fromTarget.TargetNode == nil {
			return fmt.Errorf("from path %q not found for move", op.From)
		}
		valNode := cloneNode(fromTarget.TargetNode)
		if err := removeNode(fromTarget); err != nil {
			return err
		}
		toTarget, err := resolvePath(p.rootNode, parts)
		if err != nil {
			return err
		}
		return addOrReplaceNode(toTarget, valNode)

	case "copy":
		fromParts, err := parseJSONPointer(op.From)
		if err != nil {
			return err
		}
		fromTarget, err := resolvePath(p.rootNode, fromParts)
		if err != nil || fromTarget.TargetNode == nil {
			return fmt.Errorf("from path %q not found for copy", op.From)
		}
		valNode := cloneNode(fromTarget.TargetNode)
		toTarget, err := resolvePath(p.rootNode, parts)
		if err != nil {
			return err
		}
		return addOrReplaceNode(toTarget, valNode)

	default:
		return fmt.Errorf("unsupported op: %q", op.Op)
	}

	return nil
}

// JSON Pointer Path Resolution

type resolvedPath struct {
	Parent     *yaml.Node
	Key        string
	Index      int
	TargetNode *yaml.Node
}

func parseJSONPointer(path string) ([]string, error) {
	if path == "" {
		return nil, nil
	}
	if path[0] != '/' {
		return nil, fmt.Errorf("invalid JSON pointer: must start with '/'")
	}
	parts := strings.Split(path[1:], "/")
	for i, part := range parts {
		part = strings.ReplaceAll(part, "~1", "/")
		part = strings.ReplaceAll(part, "~0", "~")
		parts[i] = part
	}
	return parts, nil
}

func resolvePath(root *yaml.Node, parts []string) (*resolvedPath, error) {
	curr := root
	if curr.Kind == yaml.DocumentNode {
		if len(curr.Content) == 0 {
			return nil, fmt.Errorf("empty document node")
		}
		curr = curr.Content[0]
	}

	var parent *yaml.Node
	var targetKey string
	var targetIndex int = -1

	for i, part := range parts {
		parent = curr
		isLast := i == len(parts)-1

		switch curr.Kind {
		case yaml.MappingNode:
			targetKey = part
			targetIndex = -1
			found := false
			for j := 0; j < len(curr.Content)-1; j += 2 {
				if curr.Content[j].Value == part {
					curr = curr.Content[j+1]
					found = true
					break
				}
			}
			if !found {
				if isLast {
					return &resolvedPath{Parent: parent, Key: targetKey, Index: -1, TargetNode: nil}, nil
				}
				return nil, fmt.Errorf("key %q not found", part)
			}

		case yaml.SequenceNode:
			var idx int
			if part == "-" {
				idx = len(curr.Content)
			} else {
				var err error
				idx, err = strconv.Atoi(part)
				if err != nil || idx < 0 || idx > len(curr.Content) {
					return nil, fmt.Errorf("invalid sequence index %q", part)
				}
			}
			targetIndex = idx
			targetKey = ""

			if idx < len(curr.Content) {
				curr = curr.Content[idx]
			} else {
				if isLast && part == "-" {
					return &resolvedPath{Parent: parent, Key: "", Index: targetIndex, TargetNode: nil}, nil
				}
				return nil, fmt.Errorf("index %d out of bounds", idx)
			}

		default:
			return nil, fmt.Errorf("cannot traverse node of kind %v", curr.Kind)
		}
	}

	return &resolvedPath{Parent: parent, Key: targetKey, Index: targetIndex, TargetNode: curr}, nil
}

// AST Modification Primitives

func addOrReplaceNode(target *resolvedPath, valNode *yaml.Node) error {
	parent := target.Parent
	if parent == nil {
		return fmt.Errorf("no parent node resolved")
	}

	if parent.Kind == yaml.MappingNode {
		// Replace if exists
		for j := 0; j < len(parent.Content)-1; j += 2 {
			if parent.Content[j].Value == target.Key {
				parent.Content[j+1] = valNode
				return nil
			}
		}
		// Append if new key
		parent.Content = append(parent.Content,
			&yaml.Node{Kind: yaml.ScalarNode, Value: target.Key},
			valNode,
		)
		return nil
	}

	if parent.Kind == yaml.SequenceNode {
		idx := target.Index
		if idx == len(parent.Content) {
			parent.Content = append(parent.Content, valNode)
			return nil
		}
		newContent := make([]*yaml.Node, 0, len(parent.Content)+1)
		newContent = append(newContent, parent.Content[:idx]...)
		newContent = append(newContent, valNode)
		newContent = append(newContent, parent.Content[idx:]...)
		parent.Content = newContent
		return nil
	}

	return fmt.Errorf("cannot add/replace on node kind %v", parent.Kind)
}

func removeNode(target *resolvedPath) error {
	parent := target.Parent
	if parent.Kind == yaml.MappingNode {
		for j := 0; j < len(parent.Content)-1; j += 2 {
			if parent.Content[j].Value == target.Key {
				parent.Content = append(parent.Content[:j], parent.Content[j+2:]...)
				return nil
			}
		}
		return fmt.Errorf("key %q not found for removal", target.Key)
	}

	if parent.Kind == yaml.SequenceNode {
		idx := target.Index
		if idx < 0 || idx >= len(parent.Content) {
			return fmt.Errorf("index %d out of bounds for removal", idx)
		}
		parent.Content = append(parent.Content[:idx], parent.Content[idx+1:]...)
		return nil
	}

	return fmt.Errorf("cannot remove from node kind %v", parent.Kind)
}

// Deep cloning and equivalence helpers

func cloneNode(node *yaml.Node) *yaml.Node {
	if node == nil {
		return nil
	}
	res := *node
	if len(node.Content) > 0 {
		res.Content = make([]*yaml.Node, len(node.Content))
		for i, child := range node.Content {
			res.Content[i] = cloneNode(child)
		}
	}
	return &res
}

func equalNodes(n1, n2 *yaml.Node) bool {
	if n1 == nil || n2 == nil {
		return n1 == n2
	}
	if n1.Kind != n2.Kind || n1.Value != n2.Value || n1.Tag != n2.Tag {
		return false
	}
	if len(n1.Content) != len(n2.Content) {
		return false
	}
	for i := range n1.Content {
		if !equalNodes(n1.Content[i], n2.Content[i]) {
			return false
		}
	}
	return true
}
