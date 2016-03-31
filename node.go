package reactor

type state int

const (
	submitted = iota
	dispatched
	running
	done
	missingDependency
	errored
	blocked
)

func (s state) String() string {
	switch s {
	case submitted:
		return "Submitted"
	case dispatched:
		return "Dispatched"
	case running:
		return "Running"
	case done:
		return "Done"
	case missingDependency:
		return "MissDependency"
	case errored:
		return "Errored"
	case blocked:
		return "Blocked"
	}
	return "Unknown"
}

type node struct {
	id           string
	state        state
	err          error
	task         func() error
	dependencies []string
}

func (s state) needsEvaluation() bool {
	return s == submitted || s == missingDependency
}

func (s state) blocking() bool {
	return s == errored ||
		s == blocked
}
