/*
Package instruction provides types and functions for communication with Pepper. Instruction interface provides
the interface for implementation of a more specific instruction, e.g. "Say Phrase", "Show Image", "Move", etc.
This instruction can be sent to Pepper using SendInstruction(). Command type helps to enumerate actions implemented
in the Android application and provide an explicit naming for it.
*/
package instruction

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"github.com/gorilla/websocket"
)

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
	// IsNil is true, when an instruction is empty and uninitialized or when all its members are nil.
	IsNil() bool
}

// Command is a type which helps to enumerate and make clear all possible commands for a robot.
type Command int

// Pepper commands enumeration
const (
	// TODO: ActionCommand should be removed from commands without changing other values, because they are stored in files.
	ActionCommand Command = iota // ActionCommand is not, actually, a command for Pepper, it's a container of commands
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

//

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

func (pm PepperMessage) SendWS(connection *websocket.Conn, mu *sync.Mutex) error {
	b, err := json.Marshal(pm)
	if err != nil {
		return fmt.Errorf("can't marshal PepperMessage: %v", err)
	}
	mu.Lock()
	defer func() {
		mu.Unlock()
	}()
	return connection.WriteMessage(websocket.TextMessage, b)
}

// SendInstruction sends an instruction to a robot via a web socket.
func SendInstruction(instr Instruction, connection *websocket.Conn, mu *sync.Mutex) (err error) {
	if connection == nil {
		return fmt.Errorf("websocket connection is nil, Pepper must initiate it first")
	}

	switch instr.Command() {
	case ActionCommand:
		err = handleAction(instr, connection, mu)
	case SayCommand:
		err = handleSay(instr, connection, mu)
	default:
		err = handleAny(instr, connection, mu)
	}

	return err
}

func handleAction(instr Instruction, connection *websocket.Conn, mu *sync.Mutex) error {
	// unpacking the wrapper and sending three actions sequentially
	// NOTE: actually, we send only a motion and image now, because audio is played via a speaker from a local computer
	action := instr.(*Action)

	// NOTE: order of processing matters: image with URL go last

	// getting the phrase content--must come before image processing, otherwise a phony phrase can disrupt
	// image showing on a robot, because the image is cancelled when any other command is sent
	if action.SayItem != nil {
		phrase := PepperMessage{
			Command: action.SayItem.Command(),
			// NOTE: we send a phony string, because the phrase is being played in the client JS app currently
			Content: base64.StdEncoding.EncodeToString([]byte("")),
			Name:    action.SayItem.GetName(),
			Delay:   action.SayItem.DelayMillis(),
		}

		if err := phrase.SendWS(connection, mu); err != nil {
			return err
		}
	}

	// trying to get the content of a file
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

			if err := move.SendWS(connection, mu); err != nil {
				return err
			}
		}
	}

	// trying to get an image
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

			if err := image.SendWS(connection, mu); err != nil {
				return err
			}
		}
	}

	// trying to get an URL
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

			if err := webURL.SendWS(connection, mu); err != nil {
				return err
			}
		}
	}

	return nil
}

func handleSay(instr Instruction, connection *websocket.Conn, mu *sync.Mutex) error {
	msg := PepperMessage{
		Command: instr.Command(),
		Name:    instr.GetName(),
		// NOTE: we send a phony string, because the phrase is being played in the client JS app currently
		Content: base64.StdEncoding.EncodeToString([]byte("")),
		Delay:   instr.DelayMillis(),
	}

	return msg.SendWS(connection, mu)
}

func handleAny(instr Instruction, connection *websocket.Conn, mu *sync.Mutex) error {
	name := instr.GetName()
	content, err := instr.Content()
	if err != nil && name == "" {
		return fmt.Errorf("can't get content out of an instruction and Name is missing, which makes the instruction ambiguous: %v", err)
	} else {
		err = nil
	}

	msg := PepperMessage{
		Command: instr.Command(),
		Name:    name,
		Content: base64.StdEncoding.EncodeToString(content),
		Delay:   instr.DelayMillis(),
	}

	return msg.SendWS(connection, mu)
}
