package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"sync"
)

// Session represents a session with a child, a set of questions and simple answers which
// are accompanied with moves by a robot to make the conversation a lively one.
type Session struct {
	ID          uuid.UUID      `json:"ID" form:"ID"`
	Name        string         `json:"Name" form:"Name" binding:"required"`
	Description string         `json:"Description" form:"Description"`
	Items       []*SessionItem `json:"Items" form:"Items"`
}

// SessionItem represents a single unit of a session, it's a question and positive and negative
// answers accompanied with a robot's moves which are represented in the web UI as a set of buttons.
type SessionItem struct {
	ID      uuid.UUID
	Actions []*SayAndMoveAction // the first item of Actions is the main item, usually, it's the main question
	// of the session item, other actions are some kind of conversation supportive answers
}

type SessionStore struct {
	filepath string
	Sessions []*Session
	mu       sync.RWMutex
}

func NewSessionStore(fpath string) (*SessionStore, error) {
	var file *os.File
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		file, err = os.Create(fpath)
		if err != nil {
			return nil, fmt.Errorf("can't create a session store at %s: %v", fpath, err)
		}
	} else {
		file, err = os.Open(fpath)
	}
	defer file.Close()

	sessions := []*Session{}
	if err = json.NewDecoder(file).Decode(&sessions); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode sessions from %s: %v", fpath, err)
	}

	store := &SessionStore{
		filepath: fpath,
		Sessions: sessions,
	}

	return store, nil
}

// GetInstruction looks for a top level instruction, which unites Say and Move actions
// and presents them as a union of two actions, so both actions should be executed.
func (s *SessionStore) GetInstruction(id uuid.UUID) *SayAndMoveAction {
	for _, session := range s.Sessions {
		for _, item := range session.Items {
			for _, action := range item.Actions {
				if !action.IsNil() && action.GetID() == id {
					return action
				}
			}
		}
	}
	return nil
}

func (s *SessionStore) Get(id string) (*Session, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, s := range s.Sessions {
		if s.ID == uid {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *SessionStore) Create(newSession *Session) error {
	newSession.ID = uuid.Must(uuid.NewRandom())
	s.Sessions = append(s.Sessions, newSession)
	return s.dump()
}

func (s *SessionStore) Update(updatedSession *Session) error {
	for _, item := range updatedSession.Items {
		if (item.ID == uuid.UUID{}) {
			item.ID = uuid.Must(uuid.NewRandom())
		}

		for _, action := range item.Actions {
			if (action.ID == uuid.UUID{}) {
				action.ID = uuid.Must(uuid.NewRandom())
			}

			if action.SayItem != nil {
				if (action.SayItem.ID == uuid.UUID{}) {
					action.SayItem.ID = uuid.Must(uuid.NewRandom())
				}
			}

			if action.MoveItem != nil {
				if (action.MoveItem.ID == uuid.UUID{}) {
					action.MoveItem.ID = uuid.Must(uuid.NewRandom())
				}
			}
		}
	}

	for _, s := range s.Sessions {
		if s.ID == updatedSession.ID {
			*s = *updatedSession
		}
	}

	return s.dump()
}

func (s *SessionStore) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	_, err = s.Get(id)
	if err != nil {
		return err
	}

	newSessions := []*Session{}

	for _, s := range s.Sessions {
		if s.ID == uid {
			continue
		}
		newSessions = append(newSessions, s)
	}

	s.Sessions = newSessions

	return s.dump()
}

func (s *SessionStore) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Sessions)
}
