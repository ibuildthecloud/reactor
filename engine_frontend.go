package reactor

func NewDefault() Reactor {
	e := &engine{
		listeners: []chan<- Event{},
		events:    make(chan Event, 1024),
		nodes:     map[string]node{},
		watching:  map[string]bool{},
	}
	e.Start()
	return e
}

func (e *engine) Start() {
	e.Lock()
	defer e.Unlock()

	if !e.started {
		go e.loop()
	}
	e.started = true
}

func (e *engine) Execute(ids ...string) {
	e.events <- Event{
		Type:    Execute,
		NodeIds: ids,
	}
}

func (e *engine) Wait(ids ...string) error {
	c := make(chan error)
	e.events <- Event{
		Type:    Wait,
		NodeIds: ids,
		channel: c,
	}
	return <-c
}

func (e *engine) ExecuteAndWait(ids ...string) error {
	e.Execute(ids...)
	return e.Wait(ids...)
}

func (e *engine) Listen(listener chan<- Event) {
	e.listeners = append(e.listeners, listener)
}

func (e *engine) Submit(id string, task func() error, dependencies ...string) {
	e.events <- Event{
		Type:   Submit,
		NodeId: id,
		node: node{
			id:           id,
			task:         task,
			dependencies: dependencies,
		},
	}
}

func (e *engine) Close() {
	e.events <- Event{
		Type: Close,
	}
}
