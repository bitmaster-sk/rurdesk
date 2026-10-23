package model

import "encoding/json"

type WikiPromptPage struct {
	Slug    string `json:"slug"`
	Title   string `json:"title"`
	Version int    `json:"version"`
	Reason  string `json:"reason"`
	Body    string `json:"body"`
}

type WikiPromptIndexEntry struct {
	Slug      string `json:"slug"`
	Title     string `json:"title"`
	Summary   string `json:"summary"`
	OverLimit bool   `json:"overLimit"`
}

type WikiPromptContext struct {
	Pages     []json.RawMessage `json:"pages"`
	Index     []json.RawMessage `json:"index"`
	IndexMore int               `json:"indexMore"`
}
