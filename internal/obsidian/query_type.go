package obsidian

import (
	"fmt"
	"strings"
)

// QueryType is the search_vault queryType argument.
type QueryType string

const (
	QueryTypeDataview  QueryType = "dataview"
	QueryTypeJSONLogic QueryType = "jsonlogic"
)

// ParseQueryType trims and lower-cases s and accepts only "dataview" and
// "jsonlogic". Anything else (including an empty value and "dql") is an error
// that names the valid values.
func ParseQueryType(s string) (QueryType, error) {
	switch qt := QueryType(strings.ToLower(strings.TrimSpace(s))); qt {
	case QueryTypeDataview, QueryTypeJSONLogic:
		return qt, nil
	case "dql":
		return "", fmt.Errorf("invalid queryType %q: valid values are %q and %q (case-insensitive); use %q for a Dataview DQL query", s, QueryTypeDataview, QueryTypeJSONLogic, QueryTypeDataview)
	default:
		return "", fmt.Errorf("invalid queryType %q: valid values are %q and %q (case-insensitive)", s, QueryTypeDataview, QueryTypeJSONLogic)
	}
}
