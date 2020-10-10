package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"log"
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

func (s *Session) initializeIDs() {
	if s == nil {
		return
	}

	if (s.ID == uuid.UUID{}) {
		s.ID = uuid.Must(uuid.NewRandom())
	}

	for _, item := range s.Items {
		if item == nil {
			continue
		}

		if (item.ID == uuid.UUID{}) {
			item.ID = uuid.Must(uuid.NewRandom())
		}

		for _, action := range item.Actions {
			if action == nil {
				continue
			}
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
			if action.ImageItem != nil {
				if (action.ImageItem.ID == uuid.UUID{}) {
					action.ImageItem.ID = uuid.Must(uuid.NewRandom())
				}
			}
		}
	}
}

// SessionItem represents a single unit of a session, it's a question and positive and negative
// answers accompanied with a robot's moves which are represented in the web UI as a set of buttons.
type SessionItem struct {
	ID      uuid.UUID
	Actions []*SayAndMoveAction // the first item of Actions is the main item, usually, it's the main question
	// of the session item, other actions are some kind of conversation supportive answers
}

type SessionStore struct {
	Sessions []*Session

	filepath string
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

	// create empty show-image actions for existing items
	for _, s := range sessions {
		for _, item := range s.Items {
			for _, action := range item.Actions {
				if action.ImageItem == nil {
					action.ImageItem = &ImageAction{}
				}
			}
		}
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
				if action == nil {
					continue
				}

				if action.ID == id {
					return action
				}

				if action.SayItem != nil && action.SayItem.ID == id {
					return action
				}
				if action.MoveItem != nil && action.MoveItem.ID == id {
					return action
				}
				if action.ImageItem != nil && action.ImageItem.ID == id {
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

func (s *SessionStore) GetItem(id string) (*SessionItem, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, s := range s.Sessions {
		for _, item := range s.Items {
			if item.ID == uid {
				return item, nil
			}
		}
	}

	return nil, fmt.Errorf("not found")
}

//func (s *SessionStore) FindAction(id string) (interface{}, error) {
//	uid, err := uuid.Parse(id)
//	if err != nil {
//		return nil, err
//	}
//	for _, session := range s.Sessions {
//		for _, item := range session.Items {
//			if item == nil {
//				continue
//			}
//			for _, action := range item.Actions {
//				if action == nil {
//					continue
//				}
//				if action.SayItem != nil {
//					if action.SayItem.ID == uid {
//						return action.SayItem, nil
//					}
//				}
//				if action.MoveItem != nil {
//					if action.MoveItem.ID == uid {
//						return action.MoveItem, nil
//					}
//				}
//			}
//		}
//	}
//	return nil, fmt.Errorf("not found")
//}

func (s *SessionStore) Create(newSession *Session) error {
	newSession.initializeIDs()
	s.Sessions = append(s.Sessions, newSession)
	return s.dump()
}

func (s *SessionStore) Update(updatedSession *Session) error {
	updatedSession.initializeIDs()
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

	session, err := s.Get(id)
	if err != nil {
		return err
	}

	// removing all audio files attached to the session
	for _, item := range session.Items {
		if item == nil {
			continue
		}
		for _, action := range item.Actions {
			if action.SayItem != nil && action.SayItem.FilePath != "" {
				if err = removeFile(action.SayItem.FilePath); err != nil {
					return err
				}
			}
			if action.ImageItem != nil && action.ImageItem.FilePath != "" {
				if err = removeFile(action.ImageItem.FilePath); err != nil {
					return err
				}
			}
		}
	}

	newSessions := []*Session{}

	for _, s := range s.Sessions {
		if s.ID == uid {
			continue
		}
		newSessions = append(newSessions, s)
	}

	s.mu.Lock()
	s.Sessions = newSessions
	s.mu.Unlock()

	return s.dump()
}

func (s *SessionStore) DeleteInstruction(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	// removing instruction's files
	instruction := s.GetInstruction(uid)
	if instruction != nil {
		if instruction.SayItem != nil && len(instruction.SayItem.FilePath) > 0 {
			err = os.Remove(instruction.SayItem.FilePath)
			if err != nil {
				log.Println(fmt.Errorf("failed to remove audio from the action: %v", err))
			}
		}
		if instruction.ImageItem != nil && instruction.ImageItem.FilePath != "" {
			err = os.Remove(instruction.ImageItem.FilePath)
			if err != nil {
				log.Println(fmt.Errorf("failed to remove image %s from the action: %v", instruction.ImageItem.FilePath, err))
			}
		}
	}

	// removing the instruction
	for _, session := range s.Sessions {
		for _, item := range session.Items {
			for _, instruction := range item.Actions {
				if instruction.ID == uid {
					newActions := []*SayAndMoveAction{}
					for _, action := range item.Actions {
						if action.ID == uid {
							continue
						}
						newActions = append(newActions, action)
					}
					s.mu.Lock()
					item.Actions = newActions
					s.mu.Unlock()
					break
				}
			}
		}
	}

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

func removeFile(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("tried to remove unexistent file: %v", filePath)
			err = nil
		} else {
			err = fmt.Errorf("failed to remove a file %s: %v", filePath, err)
		}
	}
	return err
}
