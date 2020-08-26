package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"sort"
	"sync"
)

type SayStore struct {
	Items []*SayAction

	filepath string
	mu       sync.RWMutex
}

func NewSayStore(fpath string) (*SayStore, error) {
	var file *os.File
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		file, err = os.Create(fpath)
		if err != nil {
			return nil, fmt.Errorf("can't create an audio store at %s: %v", fpath, err)
		}
	} else {
		file, err = os.Open(fpath)
	}
	defer file.Close()

	store := &SayStore{
		filepath: fpath,
		Items:    []*SayAction{},
	}
	if err = json.NewDecoder(file).Decode(&store.Items); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode audio items from %s: %v", fpath, err)
	}

	return store, store.dump()
}

func (s *SayStore) GetByUUID(id uuid.UUID) (*SayAction, error) {
	for _, s := range s.Items {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *SayStore) GetByPath(path string) (*SayAction, error) {
	for _, action := range s.Items {
		if action.FilePath == path {
			return action, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *SayStore) Get(id string) (*SayAction, error) {
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

func (s *SayStore) Create(m *SayAction) error {
	if (m.ID == uuid.UUID{}) {
		return fmt.Errorf("failed to create an audio: ID must be provided")
	}

	s.mu.Lock()
	s.Items = append(s.Items, m)
	s.mu.Unlock()
	return s.dump()
}

func (s *SayStore) Update(updatedAudio *SayAction) error {
	s.mu.Lock()
	for _, s := range s.Items {
		if s.ID == updatedAudio.ID {
			*s = *updatedAudio
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *SayStore) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	item, err := s.Get(id)
	if err != nil {
		return err
	}

	newItems := []*SayAction{}

	for _, s := range s.Items {
		if s.ID == uid {
			continue
		}
		newItems = append(newItems, s)
	}

	s.mu.Lock()
	if err = os.Remove(item.FilePath); err != nil {
		return fmt.Errorf("failed to remove a file: %v", err)
	}
	s.Items = newItems
	s.mu.Unlock()

	return s.dump()
}

func (s *SayStore) DeleteByPath(path string) error {
	item, err := s.GetByPath(path)
	if err != nil {
		return err
	}

	newItems := []*SayAction{}

	for _, s := range s.Items {
		if s.ID == item.ID {
			continue
		}
		newItems = append(newItems, s)
	}

	s.mu.Lock()
	if err = os.Remove(item.FilePath); err != nil {
		return fmt.Errorf("failed to remove a file: %v", err)
	}
	s.Items = newItems
	s.mu.Unlock()

	return s.dump()
}

func (s *SayStore) GetGroups() []string {
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

func (s *SayStore) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Items)
}
