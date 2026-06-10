package command

func shouldCheckLocalPorts(skipPortChecking bool) bool {
	return !skipPortChecking
}
