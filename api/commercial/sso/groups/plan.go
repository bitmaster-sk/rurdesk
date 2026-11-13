package groups

import (
	"maps"
	"slices"
)

type Mapping struct {
	GroupValue string `db:"group_value"`
	IdTeam     int64  `db:"id_team"`
}

type SyncInput struct {
	Groups                  []string
	HasGroupsClaim          bool
	Mappings                []Mapping
	MemberTeams             []int64
	AdminGroup              string
	IsAdmin                 bool
	IsLastAdminWithPassword bool
}

type SyncPlan struct {
	AddTeams              []int64
	RemoveTeams           []int64
	SetAdmin              *bool
	IsAdminRemovalSkipped bool
}

func (p SyncPlan) IsEmpty() bool {
	return len(p.AddTeams) == 0 && len(p.RemoveTeams) == 0 && p.SetAdmin == nil
}

func PlanSync(in SyncInput) SyncPlan {
	var plan SyncPlan
	if !in.HasGroupsClaim {
		return plan
	}
	groups := map[string]bool{}
	for _, group := range in.Groups {
		groups[group] = true
	}
	wanted := map[int64]bool{}
	for _, mapping := range in.Mappings {
		wanted[mapping.IdTeam] = wanted[mapping.IdTeam] || groups[mapping.GroupValue]
	}
	isMember := map[int64]bool{}
	for _, idTeam := range in.MemberTeams {
		isMember[idTeam] = true
	}
	for _, idTeam := range slices.Sorted(maps.Keys(wanted)) {
		switch {
		case wanted[idTeam] && !isMember[idTeam]:
			plan.AddTeams = append(plan.AddTeams, idTeam)
		case !wanted[idTeam] && isMember[idTeam]:
			plan.RemoveTeams = append(plan.RemoveTeams, idTeam)
		}
	}
	if in.AdminGroup == "" {
		return plan
	}
	isWantedAdmin := groups[in.AdminGroup]
	switch {
	case isWantedAdmin && !in.IsAdmin:
		plan.SetAdmin = &isWantedAdmin
	case !isWantedAdmin && in.IsAdmin && in.IsLastAdminWithPassword:
		plan.IsAdminRemovalSkipped = true
	case !isWantedAdmin && in.IsAdmin:
		plan.SetAdmin = &isWantedAdmin
	}
	return plan
}
