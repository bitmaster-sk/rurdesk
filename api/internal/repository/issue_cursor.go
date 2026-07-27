package repository

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"time"
)

// issueCursor is the opaque keyset cursor: sort key + id of the last returned row.
type issueCursor struct {
	Col string `json:"c"`
	Dir string `json:"d"`
	Val any    `json:"v"` // last row's sort value; nil when that value was SQL NULL
	Id  int64  `json:"i"`
}

func encodeCursor(cur issueCursor) (string, error) {
	b, err := json.Marshal(cur)
	if err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(b), nil
}

func decodeCursor(s string) (*issueCursor, error) {
	b, err := base64.URLEncoding.DecodeString(s)
	if err != nil {
		return nil, err
	}
	var cur issueCursor
	if err := json.Unmarshal(b, &cur); err != nil {
		return nil, err
	}
	return &cur, nil
}

// sortColumn maps a public sort key to its SQL expression and cursor-value decoding.
type sortColumn struct {
	expr     string
	nullable bool
	toArg    func(v any) (any, error)
}

func argTime(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("time cursor value not a string: %T", v)
	}
	return time.Parse(time.RFC3339Nano, s)
}

func argString(v any) (any, error) {
	s, ok := v.(string)
	if !ok {
		return nil, fmt.Errorf("string cursor value not a string: %T", v)
	}
	return s, nil
}

func argInt(v any) (any, error) {
	f, ok := v.(float64) // JSON numbers decode to float64
	if !ok {
		return nil, fmt.Errorf("int cursor value not a number: %T", v)
	}
	return int64(f), nil
}

// issueSortColumns is the allow-list of sortable columns + their cursor decoding.
var issueSortColumns = map[string]sortColumn{
	"idIssue":        {expr: "iss.id_issue", nullable: false, toArg: argInt},
	"updateAt":       {expr: "iss.update_at", nullable: false, toArg: argTime},
	"createAt":       {expr: "iss.create_at", nullable: false, toArg: argTime},
	"scheduledAt":    {expr: "iss.scheduled_at", nullable: true, toArg: argTime},
	"title":          {expr: "iss.title", nullable: false, toArg: argString},
	"tracked":        {expr: "iss.tracked", nullable: false, toArg: argInt},
	"estimated":      {expr: "iss.estimated", nullable: false, toArg: argInt},
	"severity":       {expr: "pis.order_rank", nullable: true, toArg: argInt},
	"state":          {expr: "pit.order_rank", nullable: true, toArg: argInt},
	"qualityScore":   {expr: "iq.score", nullable: true, toArg: argInt},
	"assignedToName": {expr: "ass.name", nullable: true, toArg: argString},
}

// sortColumnFor resolves a sort key, falling back to the default (updateAt) for unknown keys.
func sortColumnFor(key string) (string, sortColumn) {
	if sc, ok := issueSortColumns[key]; ok {
		return key, sc
	}
	return "updateAt", issueSortColumns["updateAt"]
}

// buildKeysetPredicate produces the "rows after the cursor" predicate (NULLS LAST,
// id_issue tiebreaker). startIdx is the next $ placeholder number.
func buildKeysetPredicate(cur *issueCursor, startIdx int) (string, []any, int, error) {
	_, sc := sortColumnFor(cur.Col)
	asc := cur.Dir == "asc"
	idCmp, valCmp := "<", "<"
	if asc {
		idCmp, valCmp = ">", ">"
	}
	idx := startIdx
	var args []any

	if cur.Val == nil {
		pred := fmt.Sprintf("(%s IS NULL AND iss.id_issue %s $%d)", sc.expr, idCmp, idx)
		args = append(args, cur.Id)
		return pred, args, idx + 1, nil
	}

	valArg, err := sc.toArg(cur.Val)
	if err != nil {
		return "", nil, startIdx, err
	}
	valPh, idPh := idx, idx+1
	args = append(args, valArg, cur.Id)
	pred := fmt.Sprintf("(%s %s $%d OR (%s = $%d AND iss.id_issue %s $%d)",
		sc.expr, valCmp, valPh, sc.expr, valPh, idCmp, idPh)
	if sc.nullable {
		// NULLS LAST: nulls always sort after a non-null cursor value, both directions.
		pred += fmt.Sprintf(" OR %s IS NULL", sc.expr)
	}
	pred += ")"
	return pred, args, idx + 2, nil
}

// normalizeSortVal makes a captured DB value JSON-stable: time.Time -> RFC3339Nano string.
// Nano precision matters — truncating to seconds breaks the keyset comparison when rows
// share a second (e.g. bulk-created issues), dropping/skipping rows.
func normalizeSortVal(v any) any {
	if t, ok := v.(time.Time); ok {
		return t.Format(time.RFC3339Nano)
	}
	return v
}
