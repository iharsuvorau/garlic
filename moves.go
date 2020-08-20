package main

import (
	"fmt"
	"github.com/google/uuid"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

// moves represent all ready-made moves located somewhere on the disk. This presented in the
// web UI as a library of moves which can be called any time by a user.
var moves Moves

// moveGroups is a helper variable for the "/sessions/" route to list moves by a group.
var moveGroups []string

func collectMoves(dataDir string) ([]*MoveAction, error) {
	query := path.Join(dataDir, "**/*.qianim")
	matches, err := filepath.Glob(query)
	if err != nil {
		return nil, err
	}

	var items = make([]*MoveAction, len(matches))
	for i := range matches {
		// parsing the parent folder as a motion group name
		parts := strings.Split(matches[i], "/")
		parent := parts[len(parts)-2] // TODO: windows error here

		// parsing the basename as a motion name
		basename := parts[len(parts)-1]
		name := strings.Replace(basename, filepath.Ext(basename), "", 1)

		// appending a motion
		items[i] = &MoveAction{
			ID:       uuid.Must(uuid.NewRandom()),
			FilePath: matches[i],
			Group:    parent,
			Name:     name,
		}
	}

	return items, err
}

type Moves []*MoveAction

func (mm Moves) GetByID(id uuid.UUID) *MoveAction {
	for _, v := range mm {
		if v.ID == id {
			return v
		}
	}
	return nil
}

func (mm Moves) GetByStringID(id string) (*MoveAction, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, v := range mm {
		if v.ID == uid {
			return v, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (mm Moves) GetByName(name string) *MoveAction {
	for _, v := range mm {
		if v.Name == name {
			return v
		}
	}
	return nil
}

func (mm Moves) GetGroups() []string {
	var groupsMap = map[string]interface{}{}

	for _, v := range moves {
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

func (mm *Moves) AddMoves(groupName string, names []string) {
	for _, n := range names {
		*mm = append(*mm, &MoveAction{
			ID:       uuid.Must(uuid.NewRandom()),
			Name:     n,
			FilePath: "",
			Delay:    0,
			Group:    groupName,
		})
	}
}
