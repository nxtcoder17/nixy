package yaml_ast

import (
	"testing"
)

func TestJSONPatch(t *testing.T) {
	inputYAML := `
title: "Hello World"
views: 100
tags:
  - doc
  - code
author:
  name: "Jane"
  role: "admin"
`

	t.Run("replace and add mapping fields", func(t *testing.T) {
		parser, err := NewParser([]byte(inputYAML))
		if err != nil {
			t.Fatal(err)
		}

		patches := []PatchOp{
			{Op: "replace", Path: "/title", Value: "Hello JSON Patch"},
			{Op: "add", Path: "/analytics", Value: map[string]any{"viewCount": 200}},
		}

		if err := parser.ApplyPatches(patches); err != nil {
			t.Fatal(err)
		}

		var result map[string]any
		if err := parser.Decode(&result); err != nil {
			t.Fatal(err)
		}

		if result["title"] != "Hello JSON Patch" {
			t.Errorf("expected title to be 'Hello JSON Patch', got %v", result["title"])
		}

		analytics, ok := result["analytics"].(map[string]any)
		if !ok || analytics["viewCount"] != 200 {
			t.Errorf("expected analytics.viewCount to be 200, got %v", result["analytics"])
		}
	})

	t.Run("add and remove sequence items", func(t *testing.T) {
		parser, err := NewParser([]byte(inputYAML))
		if err != nil {
			t.Fatal(err)
		}

		patches := []PatchOp{
			{Op: "add", Path: "/tags/1", Value: "api"}, // Insert "api" at index 1
			{Op: "remove", Path: "/tags/0"},            // Remove "doc" (which is at index 0)
		}

		if err := parser.ApplyPatches(patches); err != nil {
			t.Fatal(err)
		}

		var result struct {
			Tags []string `yaml:"tags"`
		}
		if err := parser.Decode(&result); err != nil {
			t.Fatal(err)
		}

		expectedTags := []string{"api", "code"}
		if len(result.Tags) != 2 || result.Tags[0] != "api" || result.Tags[1] != "code" {
			t.Errorf("expected tags %v, got %v", expectedTags, result.Tags)
		}
	})

	t.Run("move and copy operations", func(t *testing.T) {
		parser, err := NewParser([]byte(inputYAML))
		if err != nil {
			t.Fatal(err)
		}

		patches := []PatchOp{
			{Op: "add", Path: "/analytics", Value: map[string]any{}},
			{Op: "move", From: "/views", Path: "/analytics/viewCount"},
			{Op: "copy", From: "/author/name", Path: "/analytics/createdBy"},
		}

		if err := parser.ApplyPatches(patches); err != nil {
			t.Fatal(err)
		}

		var result map[string]any
		if err := parser.Decode(&result); err != nil {
			t.Fatal(err)
		}

		if _, exists := result["views"]; exists {
			t.Error("expected 'views' to be moved (removed from original location)")
		}

		analytics, ok := result["analytics"].(map[string]any)
		if !ok {
			t.Fatalf("expected analytics section to exist")
		}

		if analytics["viewCount"] != 100 {
			t.Errorf("expected analytics.viewCount to be 100, got %v", analytics["viewCount"])
		}

		if analytics["createdBy"] != "Jane" {
			t.Errorf("expected analytics.createdBy to be Jane, got %v", analytics["createdBy"])
		}

		author := result["author"].(map[string]any)
		if author["name"] != "Jane" {
			t.Error("expected source of copy 'author.name' to still exist")
		}
	})

	t.Run("test operation success and failure", func(t *testing.T) {
		parser, err := NewParser([]byte(inputYAML))
		if err != nil {
			t.Fatal(err)
		}

		// Valid test should succeed
		if err := parser.ApplyPatches([]PatchOp{{Op: "test", Path: "/views", Value: 100}}); err != nil {
			t.Fatalf("expected test op to pass, got: %v", err)
		}

		// Invalid test should fail
		if err := parser.ApplyPatches([]PatchOp{{Op: "test", Path: "/views", Value: 999}}); err == nil {
			t.Fatal("expected test op to fail for value mismatch, but it succeeded")
		}
	})
}
