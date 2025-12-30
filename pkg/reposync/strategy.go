package reposync

import (
	"fmt"
	"strings"
)

// ParseStrategy converts a user-supplied string into a Strategy enum.
func ParseStrategy(value string) (Strategy, error) {
	if value == "" {
		return StrategyReset, nil
	}

	switch strings.ToLower(value) {
	case "reset", "hard":
		return StrategyReset, nil
	case "pull":
		return StrategyPull, nil
	case "fetch":
		return StrategyFetch, nil
	default:
		return "", fmt.Errorf("unknown strategy %q", value)
	}
}

// String implements fmt.Stringer.
func (s Strategy) String() string {
	return string(s)
}
