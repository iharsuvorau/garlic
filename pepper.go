package main

import (
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
)

// PepperTask represents a task for the robot to execute.
type PepperTask struct {
	Command string
	Content string
}

// Send sends a JSON message over the websocket connection to the robot.
func (task *PepperTask) Send() error {
	if wsConnection == nil {
		return fmt.Errorf("websocket connection is nil, Pepper must initiate it first")
	}

	b, err := json.Marshal(task)
	if err != nil {
		return err
	}

	return wsConnection.WriteMessage(websocket.TextMessage, b)
}
