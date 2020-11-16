package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Instruction interface

type Instruction interface {
	Command() Command
	Content() ([]byte, error)
	DelayMillis() int64
	GetName() string
	IsValid() bool
	IsNil() bool
}

type PepperMessage struct {
	Command Command `json:"command"`
	Content string  `json:"content"`
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
		wsMu.Lock()
		defer func() {
			wsMu.Unlock()
		}()
		return connection.WriteMessage(websocket.TextMessage, b)
	}

	if instruction.Command() == ActionCommand { // unpacking the wrapper and sending three actions sequentially
		// NOTE: actually, we send only a motion and image now, because audio is played via a speaker from a local computer
		cmd := instruction.(*Action)

		// first, trying to get the content of a file
		if cmd.MoveItem != nil {
			content, err := cmd.MoveItem.Content()
			if err != nil && cmd.MoveItem.Name == "" {
				// second, checking on the name presence and sending just a name,
				// the move should be located on the Android app's side then
				log.Printf("no motion content: %v", err)
				err = nil // TODO: empty MoveItem shouldn't be created in the first place, need to change the frontend behaviour
			} else {
				err = nil
			}

			if len(content) > 0 || cmd.MoveItem.Name != "" {
				move := PepperMessage{
					Command: cmd.MoveItem.Command(),
					Name:    cmd.MoveItem.Name,
					Content: base64.StdEncoding.EncodeToString(content),
					Delay:   cmd.MoveItem.DelayMillis(),
				}

				if err := send(move, connection); err != nil {
					return err
				}
			}
		}

		// second, trying to get an image
		if cmd.ImageItem != nil {
			content, err := cmd.ImageItem.Content()
			if err != nil {
				log.Println("no image content")
				err = nil
			} else {
				image := PepperMessage{
					Command: cmd.ImageItem.Command(),
					Content: base64.StdEncoding.EncodeToString(content),
					Name:    cmd.ImageItem.Name,
					Delay:   cmd.ImageItem.DelayMillis(),
				}

				if err := send(image, connection); err != nil {
					return err
				}
			}
		}
	} else { // just sending the instruction
		// TODO: this is never called probably
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
			Content: base64.StdEncoding.EncodeToString(content),
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
	case ActionCommand:
		return "sayAndMove"
	case ShowImageCommand:
		return "show_image"
	}
	return ""
}

// possible Pepper commands
const (
	ActionCommand Command = iota
	SayCommand
	MoveCommand
	ShowImageCommand
)

// Action implements Instruction

// Action is a wrapper around other elemental actions. This type is never sent over
// a web socket on itself. sendInstruction should take care about it.
type Action struct {
	ID        uuid.UUID    `json:"ID" form:"ID"`
	Name      string       `json:"Name" form:"Name" binding:"required"`
	Group     string       `json:"Group" form:"Group"`
	SayItem   *SayAction   `json:"SayItem" form:"SayItem"`
	MoveItem  *MoveAction  `json:"MoveItem" form:"MoveItem"`
	ImageItem *ImageAction `json:"ImageItem" form:"ImageItem"`
}

func (a *Action) UnmarshalJSON(b []byte) error {
	m := map[string]interface{}{}
	err := json.NewDecoder(bytes.NewReader(b)).Decode(&m)
	if err != nil {
		return err
	}

	var id, name, phrase, fpath, group string
	var delay time.Duration // in nanoseconds
	var uid uuid.UUID
	var ok bool

	// decoding general fields

	if id, ok = m["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	name, _ = m["Name"].(string)
	group, ok = m["Group"].(string)
	if !ok || group == "" {
		group = "Default"
	}

	a.ID = uid
	a.Name = name
	a.Group = group

	// decoding internal structs

	sayItem, _ := m["SayItem"].(map[string]interface{})
	moveItem, _ := m["MoveItem"].(map[string]interface{})
	imageItem, _ := m["ImageItem"].(map[string]interface{})

	if id, ok = sayItem["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	phrase, _ = sayItem["Phrase"].(string)
	fpath, _ = sayItem["FilePath"].(string)
	group, _ = sayItem["Group"].(string)
	if delay, err = castDelay(sayItem["Delay"]); err != nil {
		return err
	}
	a.SayItem = &SayAction{
		ID:       uid,
		Phrase:   phrase,
		FilePath: fpath,
		Group:    group,
		Delay:    delay,
	}

	if id, ok = moveItem["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	name, _ = moveItem["Name"].(string)
	fpath, _ = moveItem["FilePath"].(string)
	group, _ = moveItem["Group"].(string)
	if delay, err = castDelay(moveItem["Delay"]); err != nil {
		return err
	}
	a.MoveItem = &MoveAction{
		ID:       uid,
		Name:     name,
		FilePath: fpath,
		Delay:    delay,
		Group:    group,
	}

	if id, ok = imageItem["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	name, _ = imageItem["Name"].(string)
	fpath, _ = imageItem["FilePath"].(string)
	group, _ = imageItem["Group"].(string)
	if delay, err = castDelay(imageItem["Delay"]); err != nil {
		return err
	}
	a.ImageItem = &ImageAction{
		ID:       uid,
		Name:     name,
		FilePath: fpath,
		Delay:    delay,
		Group:    group,
	}

	return nil
}

func castDelay(delay interface{}) (delayNanoseconds time.Duration, err error) {
	switch v := delay.(type) { // for some reason, incoming value can by any of these types
	case string:
		delayNanoseconds, err = time.ParseDuration(v + "s")
		if err != nil {
			return
		}
	case int:
		delayNanoseconds = time.Duration(v * 1000000000) // because it must be in nanoseconds and incoming is in seconds
	case float64:
		delayNanoseconds = time.Duration(int64(v) * 1000000000) // because it must be in nanoseconds and incoming is in seconds
	default:
		delayNanoseconds = 0
	}
	return
}

func NewAction() *Action {
	return &Action{
		ID:    uuid.UUID{},
		Name:  "",
		Group: "Default",
		SayItem: &SayAction{
			ID:       uuid.UUID{},
			Phrase:   "",
			FilePath: "",
			Group:    "",
			Delay:    0,
		},
		MoveItem: &MoveAction{
			ID:       uuid.UUID{},
			Name:     "",
			FilePath: "",
			Delay:    0,
			Group:    "",
		},
		ImageItem: &ImageAction{
			ID:       uuid.UUID{},
			Name:     "",
			FilePath: "",
			Delay:    0,
			Group:    "",
		},
	}
}

func (item *Action) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if !item.SayItem.IsValid() && !item.MoveItem.IsValid() && !item.ImageItem.IsValid() {
		return false
	}
	return true
}

func (item *Action) Command() Command {
	return ActionCommand
}

func (item *Action) Content() (b []byte, err error) {
	return
}

func (item *Action) DelayMillis() int64 {
	return 0
}

func (item *Action) IsNil() bool {
	return item == nil
}

func (item *Action) GetName() string {
	return item.MoveItem.Name
}

// SayAction implements Instruction

type SayAction struct {
	ID       uuid.UUID
	Phrase   string
	FilePath string
	Group    string
	Delay    time.Duration
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
	return item.Delay.Milliseconds()
}

//func (item *SayAction) String() string {
//	return fmt.Sprintf("say %s from %s", item.Phrase, item.FilePath)
//}

func (item *SayAction) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if item.FilePath == "" && item.Phrase == "" {
		return false
	}
	return true
}

func (item *SayAction) IsNil() bool {
	return item == nil
}

func (item *SayAction) GetName() string {
	return ""
}

// MoveAction implements Instruction

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

//func (item *MoveAction) String() string {
//	if item == nil {
//		return ""
//	}
//	return fmt.Sprintf("move %s from %s", item.Name, item.FilePath)
//}

func (item *MoveAction) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if len(item.Name) == 0 || len(item.FilePath) == 0 {
		return false
	}
	return true
}

func (item *MoveAction) IsNil() bool {
	return item == nil
}

func (item *MoveAction) GetName() string {
	return item.Name
}

// ImageAction implements Instruction

type ImageAction struct {
	ID       uuid.UUID
	Name     string
	FilePath string
	Delay    time.Duration
	Group    string
}

func (item *ImageAction) Command() Command {
	return ShowImageCommand
}

func (item *ImageAction) Content() (b []byte, err error) {
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

func (item *ImageAction) DelayMillis() int64 {
	if item == nil {
		return 0
	}
	return item.Delay.Milliseconds()
}

//func (item *ImageAction) String() string {
//	if item == nil {
//		return ""
//	}
//	return fmt.Sprintf("move %s from %s", item.Name, item.FilePath)
//}

func (item *ImageAction) IsValid() bool {
	if item == nil {
		return false
	}
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if len(item.FilePath) == 0 {
		return false
	}
	return true
}

func (item *ImageAction) IsNil() bool {
	return item == nil
}

func (item *ImageAction) GetName() string {
	return item.Name
}
