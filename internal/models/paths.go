package models

import (
	"path/filepath"

	shared "github.com/asolopovas/wt/internal"
)

func familyRoot(f Family) string {
	switch f {
	case FamilyLLM:
		return filepath.Join(externalRoot(), "llm")
	default:
		return shared.ModelsDir()
	}
}

func PathFor(e Entry) string {
	specs := e.FileSpecs()
	return filepath.Join(familyRoot(e.Family), specs[0].RelPath)
}

func PathsFor(e Entry) []string {
	root := familyRoot(e.Family)
	specs := e.FileSpecs()
	out := make([]string, len(specs))
	for i, s := range specs {
		out[i] = filepath.Join(root, s.RelPath)
	}
	return out
}
