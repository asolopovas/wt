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

const filenameGrammar = "root ::= \"{\" ws \"\\\"date\\\":\" ws \"\\\"\" date \"\\\"\" ws \",\" ws \"\\\"time\\\":\" ws \"\\\"\" tm \"\\\"\" ws \",\" ws \"\\\"topic\\\":\" ws \"\\\"\" topic \"\\\"\" ws \"}\"\n" +
	"date ::= digit digit digit digit \"-\" digit digit \"-\" digit digit\n" +
	"tm   ::= digit digit \"-\" digit digit\n" +
	"topic ::= slugChar slugChar slugChar slugChar slugChar (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)? (slugChar)?\n" +
	"slugChar ::= [a-z0-9-]\n" +
	"digit ::= [0-9]\n" +
	"ws ::= [ \\t\\n]*\n"

type Suggestion struct {
	Date  string `json:"date"`
	Time  string `json:"time"`
	Topic string `json:"topic"`
}

func Suggest(ctx context.Context, transcript string, fallbackDate time.Time) (Suggestion, error) {
	r, err := llm.NewRunner()
	if err != nil {
		return Suggestion{}, err
	}

	excerpt := transcript
	if len(excerpt) > 2000 {
		excerpt = excerpt[:2000]
	}

	prompt := buildPrompt(excerpt, fallbackDate)
	out, err := r.Generate(ctx, llm.Options{
		Prompt:    prompt,
		Grammar:   filenameGrammar,
		MaxTokens: 96,
		Temp:      0.1,
	})
	if err != nil {
		return Suggestion{}, err
	}

	var s Suggestion
	if err := json.Unmarshal([]byte(out), &s); err != nil {
		return Suggestion{}, fmt.Errorf("parsing LLM JSON %q: %w", out, err)
	}
	s.normalize(fallbackDate)
	return s, nil
}

func (s *Suggestion) normalize(fallback time.Time) {
	if !validDate(s.Date) {
		s.Date = fallback.Format("2006-01-02")
	}
	if !validTime(s.Time) {
		s.Time = fallback.Format("15-04")
	}
	s.Topic = sanitizeTopic(s.Topic)
	if s.Topic == "" {
		s.Topic = "untitled"
	}
}

func (s Suggestion) Filename(ext string) string {
	ext = strings.TrimPrefix(ext, ".")
	if ext == "" {
		return fmt.Sprintf("%s_%s_%s", s.Date, s.Time, s.Topic)
	}
	return fmt.Sprintf("%s_%s_%s.%s", s.Date, s.Time, s.Topic, ext)
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

func buildPrompt(excerpt string, fallback time.Time) string {
	return fmt.Sprintf(`You are a filename generator. Read the transcript excerpt and respond with a single JSON object containing three fields:
- date: YYYY-MM-DD format
- time: HH-MM format (24-hour with hyphen, not colon)
- topic: kebab-case-slug summarizing the main subject

Rules:
- date and time: extract from transcript if explicitly mentioned, otherwise use fallback %s.
- topic: 2-6 lowercase words joined with hyphens, ASCII letters/digits/hyphen only, max 40 chars.
- Output ONLY the JSON object, no prose, no commentary.

Transcript:
%s

JSON:`, fallback.Format("2006-01-02 15-04"), excerpt)
}

var (
	dateRE  = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)
	timeRE  = regexp.MustCompile(`^\d{2}-\d{2}$`)
	slugRE  = regexp.MustCompile(`[^a-z0-9-]+`)
	multiRE = regexp.MustCompile(`-+`)
)

func validDate(s string) bool {
	if !dateRE.MatchString(s) {
		return false
	}
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

func validTime(s string) bool { return timeRE.MatchString(s) }

func sanitizeTopic(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = slugRE.ReplaceAllString(s, "-")
	s = multiRE.ReplaceAllString(s, "-")
	s = strings.Trim(s, "-")
	if len(s) > 40 {
		s = s[:40]
		s = strings.Trim(s, "-")
	}
	return s
}
