package instruction

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"

	"github.com/google/uuid"
)

// Move implements Instruction
type Move struct {
	ID       uuid.UUID
	Name     string
	FilePath string
	Delay    int64 // in seconds
	Group    string
}

func (item *Move) Command() Command {
	return MoveCommand
}

func (item *Move) Content() (b []byte, err error) {
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

func (item *Move) DelayMillis() int64 {
	return item.Delay * 1000
}

func (item *Move) IsValid() bool {
	// nil action is valid, because an action can contain empty SayItem,
	// ImageItem but non-nil URLItem, for example
	if item == nil {
		return true
	}

	// if non-nil, check other fields
	if _, err := uuid.Parse(item.ID.String()); err != nil {
		log.Println("move UUID parsing failed")
		return false
	}
	if len(item.Name) == 0 && len(item.FilePath) == 0 {
		log.Println("move's Name or FilePath are empty")
		return false
	}
	return true
}

func (item *Move) IsNil() bool {
	return item == nil
}

func (item *Move) GetName() string {
	return item.Name
}
