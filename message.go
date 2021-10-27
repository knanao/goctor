package goctor

var (
	startedMessage = &Started{emptyMessage: emptyMessage{id: "started"}}
	stoppedMessage = &Stopped{emptyMessage: emptyMessage{id: "stopped"}}
	timerMessage   = &Timer{emptyMessage: emptyMessage{id: "timer"}}
)

type Message interface {
	ID() string
}

type emptyMessage struct {
	id string
}

func (e *emptyMessage) ID() string {
	return e.id
}

type Started struct {
	emptyMessage
}

type Stopped struct {
	emptyMessage
}

type Timer struct {
	emptyMessage
}
