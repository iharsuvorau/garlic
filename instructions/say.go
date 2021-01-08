package instructions

import (
	"fmt"
	"path/filepath"

	"github.com/google/uuid"
)

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
