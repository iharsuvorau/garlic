package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"path/filepath"
	"sort"
	"time"
)

// Instruction interface

type Instruction interface {
	Command() Command
	Content() []byte
	GetID() uuid.UUID
	SetID(uuid.UUID)
	String() string
	IsValid() bool
	IsNil() bool
}

// sendInstruction sends an instruction via a web socket.
func sendInstruction(instruction Instruction, connection *websocket.Conn) error {
	if connection == nil {
		return fmt.Errorf("websocket connection is nil, Pepper must initiate it first")
	}

	type payload struct {
		command Command
		content []byte
	}

	send := func(p payload, connection *websocket.Conn) error {
		b, err := json.Marshal(p)
		if err != nil {
			return err
		}
		return connection.WriteMessage(websocket.TextMessage, b)
	}

	if instruction.Command() == SayAndMoveCommand { // unpacking the wrapper and sending two actions sequentially
		cmd := instruction.(*SayAndMoveAction)

		say := payload{
			command: cmd.SayItem.Command(),
			content: cmd.SayItem.Content(),
		}

		move := payload{
			command: cmd.MoveItem.Command(),
			content: cmd.MoveItem.Content(),
		}

		if err := send(say, connection); err != nil {
			return err
		}

		if err := send(move, connection); err != nil {
			return err
		}
	} else { // just sending the instruction
		return send(payload{
			command: instruction.Command(),
			content: instruction.Content(),
		}, connection)
	}

	return nil
}

type Command int

func (c Command) String() string {
	switch c {
	case SayCommand:
		return "say"
	case MoveCommand:
		return "move"
	case SayAndMoveCommand:
		return "sayAndMove"
	}
	return ""
}

// possible Pepper commands
const (
	SayCommand Command = iota
	MoveCommand
	SayAndMoveCommand
)

// SayAndMoveAction implements Instruction

// SayAndMoveAction is a wrapper around other elemental actions. This type is never sent over
// a web socket on itself. sendInstruction should take care about it.
type SayAndMoveAction struct {
	ID       uuid.UUID
	SayItem  *SayAction
	MoveItem *MoveAction
}

func (item *SayAndMoveAction) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if item.SayItem.Phrase == "" || item.SayItem.FilePath == "" {
		return false
	}
	return true
}

func (item *SayAndMoveAction) Command() Command {
	return SayAndMoveCommand
}

func (item *SayAndMoveAction) Content() []byte {
	return []byte{}
}

func (item *SayAndMoveAction) String() string {
	if item == nil {
		return ""
	}
	return fmt.Sprintf("say %q and move %q", item.SayItem.Phrase, item.MoveItem.Name)
}

func (item *SayAndMoveAction) IsNil() bool {
	return item == nil
}

func (item *SayAndMoveAction) SetID(id uuid.UUID) {
	if item == nil {
		return
	}
	item.ID = id
}

func (item *SayAndMoveAction) GetID() uuid.UUID {
	if item == nil {
		return uuid.UUID{}
	}
	return item.ID
}

// SayAction implements Instruction

type SayAction struct {
	ID       uuid.UUID
	Phrase   string
	FilePath string
}

func (item *SayAction) Command() Command {
	return SayCommand
}

func (item *SayAction) Content() []byte {
	if item.IsNil() {
		return []byte{}
	}

	return []byte(filepath.Base(item.FilePath))
}

func (item *SayAction) String() string {
	return fmt.Sprintf("say %s from %s", item.Phrase, item.FilePath)
}

func (item *SayAction) IsValid() bool {
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if item.FilePath == "" {
		return false
	}
	return true
}

func (item *SayAction) IsNil() bool {
	return item == nil
}

func (item *SayAction) SetID(id uuid.UUID) {
	if item == nil {
		return
	}
	item.ID = id
}

func (item *SayAction) GetID() uuid.UUID {
	if item == nil {
		return uuid.UUID{}
	}
	return item.ID
}

// MoveAction implements Instruction

type Moves []*MoveAction

func (mm Moves) GetByID(id uuid.UUID) *MoveAction {
	for _, v := range mm {
		if v.ID == id {
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

type MoveAction struct {
	ID       uuid.UUID
	Name     string
	FilePath string
	Delay    time.Duration
	Group    string
}

func (item *MoveAction) Command() Command {
	return MoveCommand
}

func (item *MoveAction) Content() []byte {
	if item.IsNil() {
		return []byte{}
	}

	return []byte(filepath.Base(item.FilePath))
}

func (item *MoveAction) String() string {
	if item == nil {
		return ""
	}
	return fmt.Sprintf("move %s from %s", item.Name, item.FilePath)
}

func (item *MoveAction) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if item.FilePath == "" {
		return false
	}
	return true
}

func (item *MoveAction) IsNil() bool {
	return item == nil
}

func (item *MoveAction) SetID(id uuid.UUID) {
	if item == nil {
		return
	}
	item.ID = id
}

func (item *MoveAction) GetID() uuid.UUID {
	if item == nil {
		return uuid.UUID{}
	}
	return item.ID
}
