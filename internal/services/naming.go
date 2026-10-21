package services

import (
	"regexp"
	"strings"

	"ssh2rclone/internal/model"
)

var tokenRegex = regexp.MustCompile(`\{([a-zA-Z0-9_]+)(?::-([^}]*))?\}`)

// RemoteNameFormatter formats remote section names based on user templates.
type RemoteNameFormatter interface {
	Format(entry model.SSHEntry, tmpl string) string
}

type remoteNameFormatter struct{}

// NewRemoteNameFormatter creates a new RemoteNameFormatter instance.
func NewRemoteNameFormatter() RemoteNameFormatter {
	return &remoteNameFormatter{}
}

func (f *remoteNameFormatter) Format(entry model.SSHEntry, tmpl string) string {
	return FormatRemoteName(entry, tmpl)
}

// FormatRemoteName renders an SSHEntry into a remote name using the given template.
func FormatRemoteName(entry model.SSHEntry, tmpl string) string {
	if strings.TrimSpace(tmpl) == "" {
		tmpl = "{alias}"
	}

	rendered := tokenRegex.ReplaceAllStringFunc(tmpl, func(match string) string {
		submatches := tokenRegex.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		varName := strings.ToLower(submatches[1])
		fallback := ""
		if len(submatches) >= 3 {
			fallback = submatches[2]
		}

		var val string
		switch varName {
		case "alias", "name":
			val = entry.Alias
		case "user", "username":
			val = entry.User
		case "host", "hostname":
			val = entry.HostName
		case "port":
			val = entry.Port
			if val == "" {
				val = "22"
			}
		default:
			return match
		}

		if val != "" {
			return val
		}
		return fallback
	})

	sanitized := SanitizeRemoteName(rendered)
	if sanitized == "" {
		return entry.Alias
	}
	return sanitized
}

// SanitizeRemoteName removes invalid rclone INI characters and normalizes whitespace.
func SanitizeRemoteName(name string) string {
	// Strip characters that break INI section parsing: [ and ]
	replacer := strings.NewReplacer("[", "", "]", "")
	cleaned := replacer.Replace(name)

	// Replace all whitespace (spaces, tabs, newlines, carriage returns) with single dashes
	spaceRegex := regexp.MustCompile(`\s+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, "-")

	// Collapse multiple dashes
	dashRegex := regexp.MustCompile(`-+`)
	cleaned = dashRegex.ReplaceAllString(cleaned, "-")

	// Trim leading and trailing dashes
	return strings.Trim(cleaned, "-")
}
