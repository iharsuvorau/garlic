package main

import (
	"github.com/google/uuid"
	"path"
	"path/filepath"
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
