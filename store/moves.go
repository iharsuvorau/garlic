package store

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/iharsuvorau/garlic/instruction"
)

func collectMoves(dataDir string) ([]*instruction.Move, error) {
	query := filepath.Join(dataDir, "**/*.qianim")
	matches, err := filepath.Glob(query)
	if err != nil {
		return nil, err
	}

	var items = make([]*instruction.Move, len(matches))
	for i := range matches {
		// parsing the parent folder as a motion group name
		dir, basename := filepath.Split(matches[i])
		group := filepath.Base(dir)

		// parsing the basename as a motion name
		name := strings.Replace(basename, filepath.Ext(basename), "", -1)

		// appending a motion
		items[i] = &instruction.Move{
			ID:       uuid.Must(uuid.NewRandom()),
			FilePath: matches[i],
			Group:    group,
			Name:     name,
		}
	}

	return items, err
}

type Moves struct {
	Moves []*instruction.Move

	filepath string
	mu       sync.RWMutex
}

func NewMoveStore(fpath, providedMoves string) (*Moves, error) {
	var isFreshDatabase bool

	var file *os.File
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		isFreshDatabase = true
		file, err = os.Create(fpath)
		if err != nil {
			return nil, fmt.Errorf("can't create a move store at %s: %v", fpath, err)
		}
	} else {
		file, err = os.Open(fpath)
	}
	defer file.Close()

	store := &Moves{filepath: fpath}
	if err = json.NewDecoder(file).Decode(&store.Moves); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode moves from %s: %v", fpath, err)
	}

	// collect provided moves only if there is no yet database created
	if isFreshDatabase {
		collected, err := collectMoves(providedMoves)
		if err != nil {
			return nil, fmt.Errorf("failed to collect provided moves: %v", err)
		}
		store.AddMany(collected)
	}

	return store, store.dump()
}

func (s *Moves) GetByUUID(id uuid.UUID) (*instruction.Move, error) {
	for _, s := range s.Moves {
		if s.ID == id {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *Moves) GetByName(name string) (*instruction.Move, error) {
	for _, s := range s.Moves {
		if s.Name == name {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *Moves) Get(id string) (*instruction.Move, error) {
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

// AddMany appends a move only if there is no another move with the same name and ID.
func (s *Moves) AddMany(moves []*instruction.Move) {
	s.mu.Lock()
	for _, m := range moves {
		shouldAppend := true
		for _, mm := range s.Moves {
			if m.ID == mm.ID || m.Name == mm.Name {
				shouldAppend = false
				break
			}
		}
		if shouldAppend {
			s.Moves = append(s.Moves, m)
		}
	}
	s.mu.Unlock()
}

func (s *Moves) Create(m *instruction.Move) error {
	if (m.ID == uuid.UUID{}) {
		return fmt.Errorf("failed to create a move: ID must be provided")
	}

	if _, err := s.GetByName(m.Name); err == nil {
		return fmt.Errorf("the move with such name already exists: %v", m.Name)
	}

	s.mu.Lock()
	s.Moves = append(s.Moves, m)
	s.mu.Unlock()
	return s.dump()
}

func (s *Moves) Update(updatedMove *instruction.Move) error {
	s.mu.Lock()
	for _, s := range s.Moves {
		if s.ID == updatedMove.ID {
			*s = *updatedMove
		}
	}
	s.mu.Unlock()

	return s.dump()
}

func (s *Moves) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	move, err := s.Get(id)
	if err != nil {
		return err
	}

	newMoves := []*instruction.Move{}

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

func (s *Moves) GetGroups() []string {
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

func (s *Moves) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Moves)
}
