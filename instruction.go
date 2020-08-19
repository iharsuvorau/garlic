package main

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"time"
)

// Instruction interface

// TODO: Instruction interface looks overloaded, need to check the usage in shrink it.

type Instruction interface {
	Command() Command
	Content() ([]byte, error)
	DelayMillis() int64
	GetName() string
	GetID() uuid.UUID
	SetID(uuid.UUID)
	String() string
	IsValid() bool
	IsNil() bool
}

// PepperIncomingMessage is used to parse requests from the Android application on the Pepper's side.
// It sends available built-in motions when starts itself, so the webserver can register these motions
// and give a user an option to use built-in motions.
type PepperIncomingMessage struct {
	Moves []string `json:"moves"`
}

type PepperMessage struct {
	Command Command `json:"command"`
	Content []byte  `json:"content"`
	Name    string  `json:"name"`
	Delay   int64   `json:"delay"`
}

func (pm PepperMessage) MarshalJSON() ([]byte, error) {
	v := map[string]interface{}{
		"command": pm.Command.String(),
		"content": string(pm.Content),
		"name":    pm.Name,
		"delay":   pm.Delay,
	}
	return json.Marshal(v)
}

func MustBytes(b []byte, err error) []byte {
	if err != nil {
		panic(err)
	}
	return b
}

// sendInstruction sends an instruction via a web socket.
func sendInstruction(instruction Instruction, connection *websocket.Conn) error {
	if connection == nil {
		return fmt.Errorf("websocket connection is nil, Pepper must initiate it first")
	}

	send := func(p PepperMessage, connection *websocket.Conn) error {
		b, err := json.Marshal(p)
		if err != nil {
			return fmt.Errorf("can't marshal PepperMessage: %v", err)
		}
		return connection.WriteMessage(websocket.TextMessage, b)
	}

	if instruction.Command() == SayAndMoveCommand { // unpacking the wrapper and sending two actions sequentially
		// NOTE: actually, we send only a motion now, because audio is played via a speaker from a local computer
		cmd := instruction.(*SayAndMoveAction)

		// first, trying to get the content of a file
		content, err := cmd.MoveItem.Content()
		if err != nil && cmd.MoveItem.Name == "" {
			// second, checking on the name presence and sending just a name,
			// the move should be located on the Android app's side then
			return fmt.Errorf("can't get content out of MoveItem and Name is missing: %v", err)
		} else {
			err = nil
		}

		move := PepperMessage{
			Command: cmd.MoveItem.Command(),
			Name:    cmd.MoveItem.Name,
			Content: content,
			Delay:   cmd.MoveItem.DelayMillis(),
		}

		if err := send(move, connection); err != nil {
			return err
		}

		// if now file path, we don't have an audio locally, then send the phrase to the robot
		if cmd.SayItem.FilePath == "" {
			say := PepperMessage{
				Command: cmd.SayItem.Command(),
				Content: []byte(cmd.SayItem.Phrase),
				Name:    "",
				Delay:   0,
			}

			if err := send(say, connection); err != nil {
				return err
			}

			return fmt.Errorf("sayItem doesn't have FilePath, command has been sent to the robot")
		}
	} else { // just sending the instruction
		name := instruction.GetName()
		content, err := instruction.Content()
		if err != nil && name == "" {
			return fmt.Errorf("can't get content out of MoveItem and Name is missing: %v", err)
		} else {
			err = nil
		}

		move := PepperMessage{
			Command: instruction.Command(),
			Name:    name,
			Content: content,
			Delay:   instruction.DelayMillis(),
		}
		return send(move, connection)
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
	if item.SayItem.Phrase == "" {
		return false
	}
	return true
}

func (item *SayAndMoveAction) Command() Command {
	return SayAndMoveCommand
}

func (item *SayAndMoveAction) Content() (b []byte, err error) {
	return
}

func (item *SayAndMoveAction) DelayMillis() int64 {
	return 0
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

func (item *SayAndMoveAction) GetName() string {
	return item.MoveItem.Name
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

func (item *SayAction) Content() (b []byte, err error) {
	if item.IsNil() {
		return b, fmt.Errorf("nil item")
	}

	return []byte(filepath.Base(item.Phrase)), nil
}

func (item *SayAction) DelayMillis() int64 {
	return 0
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

func (item *SayAction) GetName() string {
	return ""
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

func (item *MoveAction) Content() (b []byte, err error) {
	if item.IsNil() {
		return b, fmt.Errorf("nil item")
	}

	if item.FilePath == "" {
		return b, fmt.Errorf("FilePath is missing")
	}

	f, err := os.Open(item.FilePath)
	defer f.Close()
	if err != nil {
		return b, err
	}

	return ioutil.ReadAll(f)
}

func (item *MoveAction) DelayMillis() int64 {
	return item.Delay.Milliseconds()
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

func (item *MoveAction) GetName() string {
	return item.Name
}
