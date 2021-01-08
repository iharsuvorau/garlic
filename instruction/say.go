package instruction

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

// Say implements Instruction
type Say struct {
	ID       uuid.UUID
	Phrase   string
	FilePath string
	Group    string
	Delay    int64 // in seconds
}

func (item *Say) Command() Command {
	return SayCommand
}

func (item *Say) Content() (b []byte, err error) {
	if item.IsNil() {
		return b, fmt.Errorf("nil item")
	}

	return []byte(filepath.Base(item.Phrase)), nil
}

func (item *Say) DelayMillis() int64 {
	return item.Delay * 1000
}

func (item *Say) IsValid() bool {
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

func (item *Say) IsNil() bool {
	return item == nil
}

func (item *Say) GetName() string {
	return ""
}
