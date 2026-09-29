package fsm

import (
	"context"
)

type State string

type Event string

type Session struct {
	ID        string            `json:"id"`
	Type      string            `json:"type"`
	State     State             `json:"state"`
	Context   map[string]any    `json:"context"`
	Version   int               `json:"version"`
	Completed bool              `json:"completed"`
	CompanyID string            `json:"company_id"`
	UserID    string            `json:"user_id"`
	Warehouse *string           `json:"warehouse_id,omitempty"`
}

type Transition struct {
	From       State
	Event      Event
	To         State
	Permission string
	Guard      func(ctx context.Context, tx Tx, s *Session, payload map[string]any) error
	Action     func(ctx context.Context, tx Tx, s *Session, payload map[string]any) error
}

type ScreenBuilder func(ctx context.Context, s *Session) any

type Definition struct {
	Type      string
	Initial   State
	Terminal  map[State]bool
	Transitions []Transition
	Screens   map[State]ScreenBuilder
}

type Tx interface{}

type Result struct {
	Session *Session
	Screen  any
}

type Store interface {
	WithTx(ctx context.Context, fn func(Tx) error) error
}

type Engine struct {
	defs map[string]Definition
	store Store
}

func (d *Definition) Find(from State, ev Event) (Transition, bool) {
	for _, tr := range d.Transitions {
		if tr.From == from && tr.Event == ev {
			return tr, true
		}
	}
	return Transition{}, false
}

func (d *Definition) Allowed(from State) []Event {
	var events []Event
	for _, tr := range d.Transitions {
		if tr.From == from {
			events = append(events, tr.Event)
		}
	}
	return events
}

func NewEngine(defs []Definition, store Store) *Engine {
	m := map[string]Definition{}
	for _, def := range defs {
		m[def.Type] = def
	}
	return &Engine{defs: m, store: store}
}

func (e *Engine) Fire(ctx context.Context, session *Session, ev Event, payload map[string]any) (*Result, error) {
	def, ok := e.defs[session.Type]
	if !ok {
		return nil, nil
	}
	tr, ok := def.Find(session.State, ev)
	if !ok {
		return nil, nil
	}
	if tr.Guard != nil {
		if err := tr.Guard(ctx, nil, session, payload); err != nil {
			return nil, err
		}
	}
	if tr.Action != nil {
		if err := tr.Action(ctx, nil, session, payload); err != nil {
			return nil, err
		}
	}
	session.State = tr.To
	session.Version++
	if def.Terminal[session.State] {
		session.Completed = true
	}
	res := &Result{Session: session}
	if builder, ok := def.Screens[session.State]; ok {
		res.Screen = builder(ctx, session)
	}
	return res, nil
}
