package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/url"
	"os"
	"path/filepath"
	"strconv"

	"github.com/google/uuid"
	"github.com/gorilla/websocket"
)

// Instruction interface

// Instruction is the main interface to a robot, which allows to send commands and necessary data.
type Instruction interface {
	// Command returns one of Pepper commands, it signifies what kind of this instruction is.
	Command() Command
	// Content returns a payload and an error for an instruction.
	Content() ([]byte, error)
	// DelayMillis returns time in milliseconds an instruction should be delayed for.
	DelayMillis() int64
	// GetName returns a name of the instruction for whatever reason one might need it.
	GetName() string
	// IsValid is true, when an instruction is not nil, but initialized incorrectly
	// or contains incorrect or insufficient data.
	IsValid() bool
	// IsNil is true, when an instruction is empty and uninitialized.
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
		"content": pm.Content,
		"name":    pm.Name,
		"delay":   pm.Delay,
	}
	return json.Marshal(v)
}

// sendInstruction sends an instruction to a robot via a web socket.
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
		action := instruction.(*Action)

		// first, trying to get the content of a file
		if action.MoveItem != nil {
			content, err := action.MoveItem.Content()
			if err != nil && action.MoveItem.Name == "" {
				// second, checking on the name presence and sending just a name,
				// the move should be located on the Android app's side then
				log.Printf("no motion content: %v", err)
				err = nil // TODO: empty MoveItem shouldn't be created in the first place, need to change the frontend behaviour
			} else {
				err = nil
			}

			if len(content) > 0 || action.MoveItem.Name != "" {
				move := PepperMessage{
					Command: action.MoveItem.Command(),
					Name:    action.MoveItem.Name,
					Content: base64.StdEncoding.EncodeToString(content),
					Delay:   action.MoveItem.DelayMillis(),
				}

				if err := send(move, connection); err != nil {
					return err
				}
			}
		}

		// second, trying to get an image
		if action.ImageItem != nil {
			content, err := action.ImageItem.Content()
			if err != nil {
				log.Println("no image content")
				err = nil
			} else {
				image := PepperMessage{
					Command: action.ImageItem.Command(),
					Content: base64.StdEncoding.EncodeToString(content),
					Name:    action.ImageItem.Name,
					Delay:   action.ImageItem.DelayMillis(),
				}

				if err := send(image, connection); err != nil {
					return err
				}
			}
		}

		// third, trying to get an URL
		if action.URLItem != nil {
			content, err := action.URLItem.Content()
			if err != nil {
				log.Println("no URL content")
				err = nil
			} else {
				webURL := PepperMessage{
					Command: action.URLItem.Command(),
					Content: base64.StdEncoding.EncodeToString(content),
					Name:    action.URLItem.Name,
					Delay:   action.URLItem.DelayMillis(),
				}

				log.Printf("sending url: %+v", webURL)
				if err := send(webURL, connection); err != nil {
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

// Command is a type which helps to enumerate and make clear all possible commands for a robot.
type Command int

// Pepper commands enumeration
const (
	ActionCommand Command = iota
	SayCommand
	MoveCommand
	ShowImageCommand
	ShowURLCommand
)

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
	case ShowURLCommand:
		return "show_url"
	}
	return ""
}

// Action implements Instruction

// Action is a wrapper around other elemental actions. This type is never sent over a web socket on itself.
// sendInstruction takes an Action and sends containing instructions one by one.
type Action struct {
	ID        uuid.UUID    `json:"ID" form:"ID"`
	Name      string       `json:"Name" form:"Name" binding:"required"` // NOTE: not used in sessions
	Group     string       `json:"Group" form:"Group"`                  // NOTE: not used in sessions
	SayItem   *SayAction   `json:"SayItem" form:"SayItem"`
	MoveItem  *MoveAction  `json:"MoveItem" form:"MoveItem"`
	ImageItem *ImageAction `json:"ImageItem" form:"ImageItem"`
	URLItem   *URLAction   `json:"URLItem" form:"URLItem"`
}

func (a *Action) UnmarshalJSON(b []byte) error {
	m := map[string]interface{}{}
	err := json.NewDecoder(bytes.NewReader(b)).Decode(&m)
	if err != nil {
		return err
	}

	var id, name, phrase, fpath, group string
	var delaySeconds int64
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
	urlItem, _ := m["URLItem"].(map[string]interface{})

	if id, ok = sayItem["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	phrase, _ = sayItem["Phrase"].(string)
	fpath, _ = sayItem["FilePath"].(string)
	group, _ = sayItem["Group"].(string)
	if delaySeconds, err = castDelay(sayItem["Delay"]); err != nil {
		return err
	}
	a.SayItem = &SayAction{
		ID:       uid,
		Phrase:   phrase,
		FilePath: fpath,
		Group:    group,
		Delay:    delaySeconds,
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
	if delaySeconds, err = castDelay(moveItem["Delay"]); err != nil {
		return err
	}
	a.MoveItem = &MoveAction{
		ID:       uid,
		Name:     name,
		FilePath: fpath,
		Delay:    delaySeconds,
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
	if delaySeconds, err = castDelay(imageItem["Delay"]); err != nil {
		return err
	}
	a.ImageItem = &ImageAction{
		ID:       uid,
		Name:     name,
		FilePath: fpath,
		Delay:    delaySeconds,
		Group:    group,
	}

	if id, ok = urlItem["ID"].(string); ok && len(id) > 0 {
		uid, err = uuid.Parse(id)
		if err != nil {
			return err
		}
	}
	name, _ = urlItem["Name"].(string)
	group, _ = urlItem["Group"].(string)
	var urlString string
	if s, ok := urlItem["URL"].(string); ok {
		u, err := url.Parse(s) // parsing in order to ensure that the provided string is URL
		if err != nil {
			return err
		}
		urlString = u.String() // but storing it back as string
	}
	if delaySeconds, err = castDelay(urlItem["Delay"]); err != nil {
		return err
	}
	a.URLItem = &URLAction{
		ID:    uid,
		Name:  name,
		URL:   urlString,
		Delay: delaySeconds,
		Group: group,
	}

	return nil
}

func castDelay(delay interface{}) (delaySeconds int64, err error) {
	// incoming value can by any of these types
	switch v := delay.(type) {
	case string:
		var delaySecondsInt int
		delaySecondsInt, err = strconv.Atoi(v)
		delaySeconds = int64(delaySecondsInt)
	case int:
		delaySeconds = int64(v)
	case float64:
		delaySeconds = int64(v)
	default:
		delaySeconds = 0
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

func (a *Action) IsValid() bool {
	if a == nil {
		return false
	}

	if _, err := uuid.Parse(a.ID.String()); err != nil {
		return false
	}
	if !a.SayItem.IsValid() &&
		!a.MoveItem.IsValid() &&
		!a.ImageItem.IsValid() &&
		!a.URLItem.IsValid() {
		return false
	}
	return true
}

func (a *Action) Command() Command {
	return ActionCommand
}

func (a *Action) Content() (b []byte, err error) {
	return
}

func (a *Action) DelayMillis() int64 {
	return 0
}

func (a *Action) IsNil() bool {
	if a == nil {
		return true
	}

	if a.SayItem.IsNil() &&
		a.MoveItem.IsNil() &&
		a.ImageItem.IsNil() &&
		a.URLItem.IsNil() {
		return true
	}

	return false
}

func (a *Action) GetName() string {
	return a.MoveItem.Name
}

// SayAction implements Instruction

type SayAction struct {
	ID       uuid.UUID
	Phrase   string
	FilePath string
	Group    string
	Delay    int64 // in seconds
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
	return item.Delay * 1000
}

func (item *SayAction) IsValid() bool {
	// nil action is valid, because an action can contain empty SayItem,
	// ImageItem but non-nil URLItem, for example
	if item == nil {
		return true
	}

	// if non-nil, check other fields
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
	Delay    int64 // in seconds
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
	return item.Delay * 1000
}

func (item *MoveAction) IsValid() bool {
	// nil action is valid, because an action can contain empty SayItem,
	// ImageItem but non-nil URLItem, for example
	if item == nil {
		return true
	}

	// if non-nil, check other fields
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
	Delay    int64 // in seconds
	Group    string
}

func (item *ImageAction) Command() Command {
	return ShowURLCommand
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
	return item.Delay * 1000
}

func (item *ImageAction) IsValid() bool {
	// nil action is valid, because an action can contain empty SayItem,
	// ImageItem but non-nil URLItem, for example
	if item == nil {
		return true
	}

	// if non-nil, check other fields
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

// URLAction implements Instruction

type URLAction struct {
	ID    uuid.UUID
	Name  string
	URL   string
	Delay int64 // in seconds
	Group string
}

func (item *URLAction) Command() Command {
	return ShowURLCommand
}

func (item *URLAction) Content() (b []byte, err error) {
	if item.IsNil() {
		return b, fmt.Errorf("nil item")
	}

	if item.URL == "" {
		return b, fmt.Errorf("URL is empty")
	}

	return []byte(item.URL), nil
}

func (item *URLAction) DelayMillis() int64 {
	if item == nil {
		return 0
	}
	return item.Delay * 1000
}

func (item *URLAction) IsValid() bool {
	// nil action is valid, because an action can contain empty SayItem,
	// ImageItem but non-nil URLItem, for example
	if item == nil {
		return true
	}

	// if non-nil, check other fields
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		return false
	}
	if item.URL == "" {
		return false
	}

	return true
}

func (item *URLAction) IsNil() bool {
	return item == nil
}

func (item *URLAction) GetName() string {
	return item.Name
}
