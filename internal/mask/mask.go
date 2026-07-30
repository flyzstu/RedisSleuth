package mask

import (
	"regexp"
	"strings"
)

var numeric = regexp.MustCompile(`^\d+$`)

func Key(key string) string {
	parts := strings.Split(key, ":")
	for i, p := range parts {
		clean := strings.Trim(p, "{}")
		if numeric.MatchString(clean) {
			replacement := "******"
			if strings.HasPrefix(p, "{") && strings.HasSuffix(p, "}") {
				replacement = "{******}"
			}
			parts[i] = replacement
		}
	}
	return strings.Join(parts, ":")
}
