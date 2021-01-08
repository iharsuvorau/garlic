package instruction

import (
	"fmt"

	"github.com/google/uuid"
)

// ShowURI implements Instruction
type ShowURI struct {
	ID    uuid.UUID
	Name  string
	URL   string
	Delay int64 // in seconds
	Group string
}

func (item *ShowURI) Command() Command {
	return ShowURLCommand
}

func (item *ShowURI) Content() (b []byte, err error) {
	if item.IsNil() {
		return b, fmt.Errorf("nil item")
	}

	if item.URL == "" {
		return b, fmt.Errorf("URL is empty")
	}

	return []byte(item.URL), nil
}

func (item *ShowURI) DelayMillis() int64 {
	if item == nil {
		return 0
	}
	return item.Delay * 1000
}

func (item *ShowURI) IsValid() bool {
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

func (item *ShowURI) IsNil() bool {
	return item == nil
}

func (item *ShowURI) GetName() string {
	return item.Name
}
