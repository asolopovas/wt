package gui

import (
	"sort"
	"strings"
)

var languageNames = map[string]string{
	"auto": "Auto-detect",
	"en":   "English",
	"zh":   "Chinese",
	"de":   "German",
	"es":   "Spanish",
	"ru":   "Russian",
	"ko":   "Korean",
	"fr":   "French",
	"ja":   "Japanese",
	"pt":   "Portuguese",
	"tr":   "Turkish",
	"pl":   "Polish",
	"ca":   "Catalan",
	"nl":   "Dutch",
	"ar":   "Arabic",
	"sv":   "Swedish",
	"it":   "Italian",
	"id":   "Indonesian",
	"hi":   "Hindi",
	"fi":   "Finnish",
	"vi":   "Vietnamese",
	"he":   "Hebrew",
	"uk":   "Ukrainian",
	"el":   "Greek",
	"ms":   "Malay",
	"cs":   "Czech",
	"ro":   "Romanian",
	"da":   "Danish",
	"hu":   "Hungarian",
	"ta":   "Tamil",
	"no":   "Norwegian",
	"th":   "Thai",
	"ur":   "Urdu",
	"hr":   "Croatian",
	"bg":   "Bulgarian",
	"lt":   "Lithuanian",
	"la":   "Latin",
	"mi":   "Maori",
	"ml":   "Malayalam",
	"cy":   "Welsh",
	"sk":   "Slovak",
	"te":   "Telugu",
	"fa":   "Persian",
	"lv":   "Latvian",
	"bn":   "Bengali",
	"sr":   "Serbian",
	"az":   "Azerbaijani",
	"sl":   "Slovenian",
	"kn":   "Kannada",
	"et":   "Estonian",
	"mk":   "Macedonian",
	"br":   "Breton",
	"eu":   "Basque",
	"is":   "Icelandic",
	"hy":   "Armenian",
	"ne":   "Nepali",
	"mn":   "Mongolian",
	"bs":   "Bosnian",
	"kk":   "Kazakh",
	"sq":   "Albanian",
	"sw":   "Swahili",
	"gl":   "Galician",
	"mr":   "Marathi",
	"pa":   "Punjabi",
	"si":   "Sinhala",
	"km":   "Khmer",
	"sn":   "Shona",
	"yo":   "Yoruba",
	"so":   "Somali",
	"af":   "Afrikaans",
	"oc":   "Occitan",
	"ka":   "Georgian",
	"be":   "Belarusian",
	"tg":   "Tajik",
	"sd":   "Sindhi",
	"gu":   "Gujarati",
	"am":   "Amharic",
	"yi":   "Yiddish",
	"lo":   "Lao",
	"uz":   "Uzbek",
	"fo":   "Faroese",
	"ht":   "Haitian Creole",
	"ps":   "Pashto",
	"tk":   "Turkmen",
	"nn":   "Nynorsk",
	"mt":   "Maltese",
	"sa":   "Sanskrit",
	"lb":   "Luxembourgish",
	"my":   "Myanmar",
	"bo":   "Tibetan",
	"tl":   "Tagalog",
	"mg":   "Malagasy",
	"as":   "Assamese",
	"tt":   "Tatar",
	"haw":  "Hawaiian",
	"ln":   "Lingala",
	"ha":   "Hausa",
	"ba":   "Bashkir",
	"jw":   "Javanese",
	"su":   "Sundanese",
	"yue":  "Cantonese",
}

func languageDisplayName(code string) string {
	if code == "" {
		return languageNames["auto"]
	}
	if name, ok := languageNames[code]; ok {
		return name
	}
	return code
}

func languageCodeFromName(name string) string {
	if name == "" || name == languageNames["auto"] {
		return ""
	}
	for code, n := range languageNames {
		if n == name {
			if code == "auto" {
				return ""
			}
			return code
		}
	}

	if _, ok := languageNames[name]; ok {
		return name
	}
	return name
}

func allLanguageCodes() []string {
	codes := make([]string, 0, len(languageNames))
	for code := range languageNames {
		if code == "auto" {
			continue
		}
		codes = append(codes, code)
	}
	sort.Slice(codes, func(i, j int) bool {
		return strings.ToLower(languageNames[codes[i]]) <
			strings.ToLower(languageNames[codes[j]])
	})
	return append([]string{"auto"}, codes...)
}

func allLanguageNames() []string {
	codes := allLanguageCodes()
	names := make([]string, len(codes))
	for i, c := range codes {
		names[i] = languageDisplayName(c)
	}
	return names
}

func codesToNames(codes []string) []string {
	out := make([]string, len(codes))
	for i, c := range codes {
		out[i] = languageDisplayName(c)
	}
	return out
}
