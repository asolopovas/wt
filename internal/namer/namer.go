package namer

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/asolopovas/wt/internal/llm"
)

const filenameGrammar = "root ::= \"{\" ws \"\\\"topic\\\":\" ws \"\\\"\" topic \"\\\"\" ws \"}\"\n" +
	"topic ::= slugChar slugChar slugChar slugChar slugChar (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)?\n" +
	"slugChar ::= [a-z0-9-]\n" +
	"ws ::= [ \\t\\n]*\n"

type Suggestion struct {
	Stamp string `json:"-"`
	Topic string `json:"topic"`
}

func Suggest(ctx context.Context, transcript string, fallbackDate time.Time) (Suggestion, error) {
	r, err := llm.NewRunner()
	if err != nil {
		return Suggestion{}, err
	}

	excerpt := transcript
	if len(excerpt) > 6000 {
		excerpt = excerpt[:6000]
	}

	prompt := buildPrompt(excerpt)
	out, err := r.Generate(ctx, llm.Options{
		Prompt:    prompt,
		Grammar:   filenameGrammar,
		MaxTokens: 80,
		Temp:      0.1,
	})
	if err != nil {
		return Suggestion{}, err
	}

	var s Suggestion
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		return Suggestion{}, fmt.Errorf("parsing LLM JSON %q: %w", out, err)
	}
	s.Stamp = fallbackDate.Format("060102-150405")
	s.Topic = sanitizeTopic(s.Topic)
	if s.Topic == "" {
		s.Topic = "untitled"
	}
	return s, nil
}

func (s Suggestion) Filename(ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		return fmt.Sprintf("%s_%s", s.Stamp, s.Topic)
	}
	return fmt.Sprintf("%s_%s.%s", s.Stamp, s.Topic, ext)
}

func ExtractTranscriptText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	if filepath.Ext(path) == ".json" {
		return extractJSON(data)
	}
	return string(data), nil
}

func extractJSON(data []byte) (string, error) {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return string(data), nil
	}
	var b strings.Builder
	if utts, ok := raw["utterances"].([]any); ok {
		for _, u := range utts {
			m, _ := u.(map[string]any)
			if m == nil {
				continue
			}
			if speaker, ok := m["speaker"].(string); ok && speaker != "" {
				b.WriteString(speaker)
				b.WriteString(": ")
			}
			if text, ok := m["text"].(string); ok {
				b.WriteString(text)
				b.WriteString("\n")
			}
		}
	}
	if b.Len() == 0 {
		return string(data), nil
	}
	return b.String(), nil
}

func buildPrompt(excerpt string) string {
	return fmt.Sprintf(`You are a filename topic generator. Read the conversation transcript below and respond with a single JSON object: {"topic": "<slug>"}.

The topic must be a kebab-case slug of 3-7 lowercase words joined with hyphens that captures the main subject, setting, or purpose of the conversation. Use only ASCII letters, digits, and hyphens. Max 60 characters. Be specific (e.g. "fulham-boys-school-admission-interview", not "interview"; "kitchen-renovation-quote", not "renovation"; "weekly-sales-team-standup", not "meeting"). Avoid generic single words like "sports", "talk", "meeting".

Output ONLY the JSON object, no prose, no commentary, no markdown.

Transcript:
%s

JSON:`, excerpt)
}

var (
	slugRE  = regexp.MustCompile(`[^a-z0-9-]+`)
	multiRE = regexp.MustCompile(`-+`)
)

func sanitizeTopic(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRE.ReplaceAllString(s, "-")
	s = multiRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 60 {
		s = s[:60]
		s = strings.Trim(s, "-")
	}
	return s
}
