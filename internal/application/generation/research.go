package generation

import "strings"

func promptNeedsRealTimeResearch(prompt string) bool {
	normalized := strings.ToLower(strings.TrimSpace(prompt))
	if normalized == "" {
		return false
	}
	keywords := []string{
		"ultimas noticias",
		"últimas noticias",
		"noticias de",
		"semana pasada",
		"esta semana",
		"hoy",
		"ayer",
		"latest news",
		"recent news",
		"last week",
		"this week",
		"today",
		"yesterday",
	}
	for _, keyword := range keywords {
		if strings.Contains(normalized, keyword) {
			return true
		}
	}
	return false
}
