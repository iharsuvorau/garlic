package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"sync"

	"github.com/google/uuid"

	"github.com/iharsuvorau/garlic/instruction"
)

type Audio struct {
	Items []*instruction.Say

	filepath string
	mu       sync.RWMutex
}

func NewAudioStore(fpath string) (*Audio, error) {
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

	store := &Audio{
		filepath: fpath,
		Items:    []*instruction.Say{},
	}
	if err = json.NewDecoder(file).Decode(&store.Items); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode audio items from %s: %v", fpath, err)
	}

	return store, store.dump()
}

func (s *Audio) GetByUUID(id uuid.UUID) (*instruction.Say, error) {
	for _, s := range s.Items {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *Audio) GetByPath(path string) (*instruction.Say, error) {
	for _, action := range s.Items {
		if action.FilePath == path {
			return action, nil
		}
	}
	return nil, fmt.Errorf("not found")
}

func (s *Audio) Get(id string) (*instruction.Say, error) {
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

func (s *Audio) Create(m *instruction.Say) error {
	if (m.ID == uuid.UUID{}) {
		return fmt.Errorf("failed to create an audio: ID must be provided")
	}

	s.mu.Lock()
	s.Items = append(s.Items, m)
	s.mu.Unlock()
	return s.dump()
}

func (s *Audio) Update(updatedAudio *instruction.Say) error {
	s.mu.Lock()
	for _, s := range s.Items {
		if s.ID == updatedAudio.ID {
			*s = *updatedAudio
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *Audio) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	item, err := s.Get(id)
	if err != nil {
		return err
	}

	newItems := []*instruction.Say{}

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

func (s *Audio) DeleteByPath(path string) error {
	item, err := s.GetByPath(path)
	if err != nil {
		return err
	}

	newItems := []*instruction.Say{}

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

func (s *Audio) GetGroups() []string {
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

func (s *Audio) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Items)
}
