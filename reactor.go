package reactor

type EventType int

const (
	Submit      EventType = iota
	Execute     EventType = iota
	Wait        EventType = iota
	Close       EventType = iota
	StateChange EventType = iota
	TaskStart   EventType = iota
	TaskExit    EventType = iota
)

func (e EventType) String() string {
	switch e {
	case Submit:
		return "Submit"
	case Execute:
		return "Execute"
	case Wait:
		return "Wait"
	case Close:
		return "Close"
	case StateChange:
		return "StateChange"
	case TaskStart:
		return "TaskStart"
	case TaskExit:
		return "TaskExit"
	default:
		return "Unknown"
	}
}

type ErrCycleDetected struct {
	Path []string
}

type ErrMissingDependency struct {
	Ids []string
}

type Event struct {
	Type             EventType
	NodeId           string
	NodeIds          []string
	PrevState, State string
	Err              error
	node             node
	channel          chan error
}

type Reactor interface {
	Submit(id string, task func() error, dependencies ...string)
	ExecuteAndWait(ids ...string) error
	Execute(ids ...string)
	// AddDependency(id, dependencies ...string)
	Listen(chan<- Event)
	Wait(ids ...string) error
	Start()
	Close()
}
