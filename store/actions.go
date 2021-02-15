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

type Actions struct {
	Items []*instruction.Action

	filepath string
	mu       sync.RWMutex
}

func NewActionsStore(fpath string) (*Actions, error) {
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

	store := &Actions{
		filepath: fpath,
		Items:    []*instruction.Action{},
	}
	if err = json.NewDecoder(file).Decode(&store.Items); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode audio items from %s: %v", fpath, err)
	}

	for _, a := range store.Items {
		a.InitiateItemsIDs()
	}

	return store, store.dump()
}

func (s *Actions) GetByUUID(id uuid.UUID) (*instruction.Action, error) {
	for _, s := range s.Items {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *Actions) Get(id string) (*instruction.Action, error) {
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

func (s *Actions) Create(a *instruction.Action) error {
	if (a.ID == uuid.UUID{}) {
		a.ID = uuid.Must(uuid.NewRandom())
	}
	if !a.IsValid() {
		return fmt.Errorf("action is not valid")
	}
	if a.IsNil() {
		return fmt.Errorf("action is nil")
	}

	a.InitiateItemsIDs()

	s.mu.Lock()
	s.Items = append(s.Items, a)
	s.mu.Unlock()
	return s.dump()
}

func (s *Actions) Update(updatedAction *instruction.Action) error {
	s.mu.Lock()
	for _, s := range s.Items {
		if s.ID == updatedAction.ID {
			*s = *updatedAction
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *Actions) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	action, err := s.Get(id)
	if err != nil {
		return err
	}

	newItems := []*instruction.Action{}

	for _, s := range s.Items {
		if s.ID == uid {
			continue
		}
		newItems = append(newItems, s)
	}

	s.mu.Lock()

	// removing resources
	if action.SayItem != nil && len(action.SayItem.FilePath) > 0 {
		if err = removeFile(action.SayItem.FilePath); err != nil {
			return err
		}
	}
	if action.ImageItem != nil && len(action.ImageItem.FilePath) > 0 {
		if err = removeFile(action.ImageItem.FilePath); err != nil {
			return err
		}
	}
	// TODO: we're not removing motions, some of them might be in the built-in data folder

	s.Items = newItems
	s.mu.Unlock()

	return s.dump()
}

func (s *Actions) GetGroups() []string {
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

func (s *Actions) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Items)
}
