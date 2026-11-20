package control

func validProjectEnvironmentScope(projectID, environmentID int64) bool {
	return projectID >= 0 && environmentID >= 0 && ((projectID == 0) == (environmentID == 0))
}
