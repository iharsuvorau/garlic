package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/google/uuid"
)

type ActionsStore struct {
	Items []*Action

	filepath string
	mu       sync.RWMutex
}

func NewActionsStore(fpath string) (*ActionsStore, error) {
	var file *os.File
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		file, err = os.Create(fpath)
		if err != nil {
			return nil, fmt.Errorf("can't create an actions store at %s: %v", fpath, err)
		}
	} else {
		file, err = os.Open(fpath)
	}
	defer file.Close()

	store := &ActionsStore{
		filepath: fpath,
		Items:    []*Action{},
	}
	if err = json.NewDecoder(file).Decode(&store.Items); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode audio items from %s: %v", fpath, err)
	}

	return store, store.dump()
}

func (s *ActionsStore) GetByUUID(id uuid.UUID) (*Action, error) {
	for _, s := range s.Items {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *ActionsStore) Get(id string) (*Action, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}
	for _, s := range s.Items {
		if s.ID == uid {
			return s, nil
		}
	}
	return nil, fmt.Errorf("not found: %v", id)
}

func (s *ActionsStore) Create(a *Action) error {
	if (a.ID == uuid.UUID{}) {
		a.ID = uuid.Must(uuid.NewRandom())
	}
	s.mu.Lock()
	s.Items = append(s.Items, a)
	s.mu.Unlock()
	return s.dump()
}

func (s *ActionsStore) Update(updatedAction *Action) error {
	s.mu.Lock()
	for _, s := range s.Items {
		if s.ID == updatedAction.ID {
			*s = *updatedAction
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *ActionsStore) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	_, err = s.Get(id)
	if err != nil {
		return err
	}

	newItems := []*Action{}

	for _, s := range s.Items {
		if s.ID == uid {
			continue
		}
		newItems = append(newItems, s)
	}

	s.mu.Lock()
	// TODO: remove resources
	//if err = os.Remove(item.FilePath); err != nil {
	//	return fmt.Errorf("failed to remove a file: %v", err)
	//}
	s.Items = newItems
	s.mu.Unlock()

	return s.dump()
}

func (s *ActionsStore) GetGroups() []string {
	var groupsMap = map[string]interface{}{}

	for _, v := range s.Items {
		if v == nil {
			continue
		}

		groupsMap[v.Group] = nil
	}

	var groups = make([]string, len(groupsMap))
	var i int64 = 0
	for k := range groupsMap {
		groups[i] = k
		i++
	}
	sort.Strings(groups)

	return groups
}

func (s *ActionsStore) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Items)
}
