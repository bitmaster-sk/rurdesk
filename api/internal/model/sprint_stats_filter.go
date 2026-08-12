package model

type SprintStatsFilter struct {
	IdSprint  *int64
	IdProject *int64
	IdsFinal  []int64
	IdsStart  []int64
}
