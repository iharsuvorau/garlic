package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// moves represent all ready-made moves located somewhere on the disk. This presented in the
// web UI as a library of moves which can be called any time by a user.
//var moves Moves

// moveGroups is a helper variable for the "/sessions/" route to list moves by a group.
//var moveGroups []string

func collectMoves(dataDir string) ([]*MoveAction, error) {
	query := filepath.Join(dataDir, "**/*.qianim")
	matches, err := filepath.Glob(query)
	if err != nil {
		return nil, err
	}

	var items = make([]*MoveAction, len(matches))
	for i := range matches {
		// parsing the parent folder as a motion group name
		dir, basename := filepath.Split(matches[i])
		group := filepath.Base(dir)

		// parsing the basename as a motion name
		name := strings.Replace(basename, filepath.Ext(basename), "", 1)

		// appending a motion
		items[i] = &MoveAction{
			ID:       uuid.Must(uuid.NewRandom()),
			FilePath: matches[i],
			Group:    group,
			Name:     name,
		}
	}

	return items, err
}

//type Moves []*MoveAction
//
//func (mm Moves) GetByID(id uuid.UUID) *MoveAction {
//	for _, v := range mm {
//		if v.ID == id {
//			return v
//		}
//	}
//	return nil
//}
//
//func (mm Moves) GetByStringID(id string) (*MoveAction, error) {
//	uid, err := uuid.Parse(id)
//	if err != nil {
//		return nil, err
//	}
//
//	for _, v := range mm {
//		if v.ID == uid {
//			return v, nil
//		}
//	}
//
//	return nil, fmt.Errorf("not found")
//}
//
//func (mm Moves) GetByName(name string) *MoveAction {
//	for _, v := range mm {
//		if v.Name == name {
//			return v
//		}
//	}
//	return nil
//}
//
//func (mm Moves) GetGroups() []string {
//	var groupsMap = map[string]interface{}{}
//
//	for _, v := range moves {
//		if v == nil {
//			continue
//		}
//
//		groupsMap[v.Group] = nil
//	}
//
//	var groups = make([]string, len(groupsMap))
//	var i int64 = 0
//	for k := range groupsMap {
//		groups[i] = k
//		i++
//	}
//	sort.Strings(groups)
//
//	return groups
//}
//
//func (mm *Moves) AddMoves(groupName string, names []string) {
//	for _, n := range names {
//		*mm = append(*mm, &MoveAction{
//			ID:       uuid.Must(uuid.NewRandom()),
//			Name:     n,
//			FilePath: "",
//			Delay:    0,
//			Group:    groupName,
//		})
//	}
//}

//

type MoveStore struct {
	Moves []*MoveAction

	filepath string
	mu       sync.RWMutex
}

func NewMoveStore(fpath, providedMoves string) (*MoveStore, error) {
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

	moves := []*MoveAction{}
	if err = json.NewDecoder(file).Decode(&moves); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode moves from %s: %v", fpath, err)
	}

	store := &MoveStore{
		filepath: fpath,
		Moves:    moves,
	}

	// collect provided moves
	collected, err := collectMoves(providedMoves)
	if err != nil {
		return nil, fmt.Errorf("failed to collect provided moves: %v", err)
	}
	store.Moves = append(store.Moves, collected...)

	return store, store.dump()
}

func (s *MoveStore) GetByUUID(id uuid.UUID) (*MoveAction, error) {
	for _, s := range s.Moves {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *MoveStore) Get(id string) (*MoveAction, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, s := range s.Moves {
		if s.ID == uid {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *MoveStore) AddMany(groupName string, names []string) {
	s.mu.Lock()
	for _, n := range names {
		s.Moves = append(s.Moves, &MoveAction{
			ID:       uuid.Must(uuid.NewRandom()),
			Name:     n,
			FilePath: "",
			Delay:    0,
			Group:    groupName,
		})
	}
	s.mu.Unlock()
}

func (s *MoveStore) Create(m *MoveAction) error {
	m.ID = uuid.Must(uuid.NewRandom())

	_, err := s.GetByUUID(m.ID)
	if err == nil {
		return fmt.Errorf("the move with such ID already exists: %v", m.ID)
	}

	s.mu.Lock()
	s.Moves = append(s.Moves, m)
	s.mu.Unlock()
	return s.dump()
}

func (s *MoveStore) Update(updatedMove *MoveAction) error {
	s.mu.Lock()
	for _, s := range s.Moves {
		if s.ID == updatedMove.ID {
			*s = *updatedMove
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *MoveStore) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	move, err := s.Get(id)
	if err != nil {
		return err
	}

	newMoves := []*MoveAction{}

	for _, s := range s.Moves {
		if s.ID == uid {
			continue
		}
		newMoves = append(newMoves, s)
	}

	s.mu.Lock()
	if err = os.Remove(move.FilePath); err != nil {
		return fmt.Errorf("failed to remove a file: %v", err)
	}
	s.Moves = newMoves
	s.mu.Unlock()

	return s.dump()
}

func (s *MoveStore) GetGroups() []string {
	var groupsMap = map[string]interface{}{}

	for _, v := range s.Moves {
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

func (s *MoveStore) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Moves)
}
