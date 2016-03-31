package reactor

import (
	"bytes"
	"fmt"
	"strings"
	"sync"

	"github.com/Sirupsen/logrus"
)

var (
	log = logrus.WithField("component", "backend")
)

func (e ErrCycleDetected) Error() string {
	return fmt.Sprintf("Cycle detected %s", strings.Join(e.Path, ","))
}

func (e ErrMissingDependency) Error() string {
	return fmt.Sprintf("Missing dependencies: %s", strings.Join(e.Ids, ", "))
}

type CompositeError struct {
	Errors []error
}

func (c CompositeError) Error() string {
	buf := bytes.Buffer{}
	for _, e := range c.Errors {
		if buf.Len() == 0 {
			buf.WriteString(e.Error())
		} else {
			buf.WriteString(", ")
			buf.WriteString(e.Error())
		}
	}

	return buf.String()
}

type engine struct {
	sync.Mutex
	started   bool
	events    chan Event
	watching  map[string]bool
	waiting   []Event
	listeners []chan<- Event
	pending   int
	nodes     map[string]node
}

func (e *engine) notify(event Event) {
	log.WithField("event", event).Debug("Notify")
	for _, l := range e.listeners {
		l <- event
	}
}

func (e *engine) stop() {
	e.Lock()
	defer e.Unlock()
	e.started = false
}

func (e *engine) loop() {
	for event := range e.events {
		switch event.Type {
		case Submit:
			e.nodes[event.node.id] = event.node
		case Close:
			e.stop()
			return
		case Execute:
			for _, id := range event.NodeIds {
				e.watching[id] = true
			}
		case TaskStart:
			e.setState(event.NodeId, running)
		case TaskExit:
			if event.Err == nil {
				e.setState(event.NodeId, done)
			} else {
				e.setState(event.NodeId, errored)
				e.setError(event.NodeId, event.Err)
			}
		case Wait:
			e.waiting = append(e.waiting, event)
		}

		e.walk()
		e.checkDone()

		e.notify(event)
	}
}

func (e *engine) checkDone() {
	filtered := []Event{}
	for _, waiting := range e.waiting {
		var errors []error
		isDone := true
		for _, id := range waiting.NodeIds {
			if node, ok := e.nodes[id]; ok {
				switch node.state {
				case missingDependency:
					missing := []string{}
					for _, dep := range node.dependencies {
						if _, ok := e.nodes[dep]; !ok {
							missing = append(missing, dep)
						}
					}
					errors = append(errors, ErrMissingDependency{missing})
				case errored:
					errors = append(errors, node.err)
				case blocked:
					errors = append(errors, node.err)
				case done:
				default:
					isDone = false
				}
			} else {
				errors = append(errors, fmt.Errorf("Failed to find task %s", id))
			}
		}

		if isDone {
			waiting.channel <- composeError(errors)
		} else {
			filtered = append(filtered, waiting)
		}
	}

	e.waiting = filtered
}

func composeError(errors []error) error {
	if len(errors) == 0 {
		return nil
	} else if len(errors) == 1 {
		return errors[0]
	} else {
		return CompositeError{errors}
	}
}

func (e *engine) walk() {
	visited := map[string]bool{}
	for id := range e.watching {
		if node, ok := e.nodes[id]; ok {
			e.walkChildren(node, visited, nil)
		}
	}
}

func (e *engine) setState(id string, state state) {
	n := e.nodes[id]
	prevState := n.state
	n.state = state
	e.nodes[id] = n

	e.notify(Event{
		Type:      StateChange,
		NodeId:    id,
		PrevState: prevState.String(),
		State:     state.String(),
	})
}

func (e *engine) setError(id string, err error) {
	n := e.nodes[id]
	n.err = err
	e.nodes[id] = n
}

func (e *engine) dispatch(node node) {
	e.setState(node.id, dispatched)
	go func() {
		e.events <- Event{
			NodeId: node.id,
			Type:   TaskStart,
		}
		e.events <- Event{
			NodeId: node.id,
			Type:   TaskExit,
			Err:    node.task(),
		}
	}()
}

func (e *engine) walkChildren(node node, visited map[string]bool, path []string) {
	visited[node.id] = true
	if !node.state.needsEvaluation() {
		return
	}

	missingDep := false
	isBlocked := false
	allDone := true
	blockingErrors := []error{}

	for _, depId := range node.dependencies {
		if depNode, ok := e.nodes[depId]; ok {
			if !visited[depId] {
				e.walkChildren(depNode, visited, append(path, node.id))
				// reload if changed
				depNode = e.nodes[depId]
			}

			if depNode.state.blocking() {
				isBlocked = true
				blockingErrors = append(blockingErrors, depNode.err)
			}
			if depNode.state != done {
				allDone = false
			}
		} else {
			allDone = false
			missingDep = true
		}
	}

	if missingDep {
		e.setState(node.id, missingDependency)
	} else if isBlocked {
		e.setState(node.id, blocked)
		e.setError(node.id, composeError(blockingErrors))
	}

	if allDone {
		e.dispatch(node)
	}
}
