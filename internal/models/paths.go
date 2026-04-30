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
	return filepath.Join(familyRoot(e.Family), e.RelPath)
}
