package instructions

import (
	"fmt"
	"io/ioutil"
	"os"

	"github.com/google/uuid"
)

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
