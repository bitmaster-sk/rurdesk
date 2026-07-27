package model

type Team struct {
	IdTeam int64  `json:"idTeam" db:"id_team"`
	Name   string `json:"name" db:"name"`
	Color  string `json:"color" db:"color"`
}

type CreateTeamReq struct {
	Name  string `json:"name" binding:"required,max=250"`
	Color string `json:"color" binding:"required,max=32"`
}

type EditTeamReq struct {
	IdTeam int64  `json:"idTeam" binding:"required"`
	Name   string `json:"name" binding:"required,max=250"`
	Color  string `json:"color" binding:"required,max=32"`
}

type DeleteTeamReq struct {
	IdTeam int64 `json:"idTeam"`
}

type AddTeamMemberReq struct {
	IdTeam int64 `json:"idTeam" binding:"required"`
	IdUser int64 `json:"idUser" binding:"required"`
}

type DeleteTeamMemberReq struct {
	IdTeam int64 `json:"idTeam"`
	IdUser int64 `json:"idUser"`
}
