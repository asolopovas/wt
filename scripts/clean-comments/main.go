package main

import (
	"bytes"
	"flag"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

var generatedMarker = regexp.MustCompile(`^// Code generated .* DO NOT EDIT\.$`)

var skipDirs = map[string]bool{
	".git":         true,
	"dist":         true,
	"node_modules": true,
	"third_party":  true,
	"vendor":       true,
}

func main() {
	dryRun := flag.Bool("n", false, "dry run (report files that would change, make no edits)")
	flag.Parse()

	roots := flag.Args()
	if len(roots) == 0 {
		roots = []string{"."}
	}

	var changed, scanned int
	for _, root := range roots {
		err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if skipDirs[filepath.Base(path)] {
					return fs.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") {
				return nil
			}
			scanned++
			modified, procErr := process(path, *dryRun)
			if procErr != nil {
				fmt.Fprintf(os.Stderr, "%s: %v\n", path, procErr)
				return nil
			}
			if modified {
				changed++
				if *dryRun {
					fmt.Printf("would rewrite %s\n", path)
				} else {
					fmt.Printf("rewrote %s\n", path)
				}
			}
			return nil
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "walk %s: %v\n", root, err)
			os.Exit(1)
		}
	}

	fmt.Fprintf(os.Stderr, "scanned %d .go files, %d %s\n",
		scanned, changed,
		map[bool]string{true: "would change", false: "changed"}[*dryRun])
}

func process(path string, dryRun bool) (bool, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return false, err
	}

	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, src, parser.ParseComments)
	if err != nil {
		return false, err
	}

	cgoPreambles := cgoPreambleGroups(file)

	keptGroups := make(map[*ast.CommentGroup]bool)
	var filteredAll []*ast.CommentGroup
	for _, cg := range file.Comments {
		if cgoPreambles[cg] {
			filteredAll = append(filteredAll, cg)
			keptGroups[cg] = true
			continue
		}
		var keepLines []*ast.Comment
		for _, c := range cg.List {
			if isDirective(c.Text) {
				keepLines = append(keepLines, c)
			}
		}
		if len(keepLines) > 0 {
			trimmed := &ast.CommentGroup{List: keepLines}
			filteredAll = append(filteredAll, trimmed)
			keptGroups[cg] = true
		}
	}
	file.Comments = filteredAll

	ast.Inspect(file, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.GenDecl:
			x.Doc = pickGroup(x.Doc, keptGroups)
		case *ast.FuncDecl:
			x.Doc = pickGroup(x.Doc, keptGroups)
		case *ast.Field:
			x.Doc = pickGroup(x.Doc, keptGroups)
			x.Comment = pickGroup(x.Comment, keptGroups)
		case *ast.ValueSpec:
			x.Doc = pickGroup(x.Doc, keptGroups)
			x.Comment = pickGroup(x.Comment, keptGroups)
		case *ast.TypeSpec:
			x.Doc = pickGroup(x.Doc, keptGroups)
			x.Comment = pickGroup(x.Comment, keptGroups)
		case *ast.ImportSpec:
			x.Doc = pickGroup(x.Doc, keptGroups)
			x.Comment = pickGroup(x.Comment, keptGroups)
		}
		return true
	})

	var buf bytes.Buffer
	if err := format.Node(&buf, fset, file); err != nil {
		return false, err
	}

	out, err := format.Source(buf.Bytes())
	if err != nil {
		out = buf.Bytes()
	}

	if bytes.Equal(src, out) {
		return false, nil
	}

	if dryRun {
		return true, nil
	}
	return true, os.WriteFile(path, out, 0o644)
}

func pickGroup(cg *ast.CommentGroup, kept map[*ast.CommentGroup]bool) *ast.CommentGroup {
	if cg == nil {
		return nil
	}
	if kept[cg] {
		return cg
	}
	var keepLines []*ast.Comment
	for _, c := range cg.List {
		if isDirective(c.Text) {
			keepLines = append(keepLines, c)
		}
	}
	if len(keepLines) == 0 {
		return nil
	}
	return &ast.CommentGroup{List: keepLines}
}

func cgoPreambleGroups(file *ast.File) map[*ast.CommentGroup]bool {
	preambles := map[*ast.CommentGroup]bool{}
	for _, decl := range file.Decls {
		gd, ok := decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.IMPORT {
			continue
		}
		importsC := false
		for _, spec := range gd.Specs {
			if is, ok := spec.(*ast.ImportSpec); ok && is.Path != nil && is.Path.Value == `"C"` {
				importsC = true
				if is.Doc != nil {
					preambles[is.Doc] = true
				}
			}
		}
		if !importsC {
			continue
		}
		if gd.Doc != nil {
			preambles[gd.Doc] = true
		}
	}
	return preambles
}

func isDirective(text string) bool {
	if !strings.HasPrefix(text, "//") {
		return false
	}
	if generatedMarker.MatchString(text) {
		return true
	}
	body := strings.TrimPrefix(text, "//")
	if strings.HasPrefix(body, "go:") {
		return true
	}
	if strings.HasPrefix(body, "+build") {
		return true
	}
	if strings.HasPrefix(body, "line ") {
		return true
	}
	if strings.HasPrefix(body, "export ") {
		return true
	}
	return false
}
