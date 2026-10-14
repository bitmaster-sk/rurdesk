package githost

type CiCommitStatus struct {
	State  string `json:"state"`
	Status string `json:"status"`
}

func (s CiCommitStatus) githubState() string { return s.State }

func (s CiCommitStatus) giteaState() string { return s.Status }
