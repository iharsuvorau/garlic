package instruction

import (
	"bytes"
	"encoding/json"
	"net/url"
	"strconv"

	"github.com/google/uuid"
)

// Action is a wrapper around other primary actions. This type is never sent over a web socket on itself.
// SendInstruction takes an Action and sends containing instruction one by one.
type Action struct {
	ID        uuid.UUID  `json:"ID" form:"ID"`
	Name      string     `json:"Name" form:"Name" binding:"required"` // NOTE: not used in sessions
	Group     string     `json:"Group" form:"Group"`                  // NOTE: not used in sessions
	SayItem   *Say       `json:"SayItem" form:"SayItem"`
	MoveItem  *Move      `json:"MoveItem" form:"MoveItem"`
	ImageItem *ShowImage `json:"ImageItem" form:"ImageItem"`
	URLItem   *ShowURI   `json:"URLItem" form:"URLItem"`
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
	a.SayItem = &Say{
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
	a.MoveItem = &Move{
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
	a.ImageItem = &ShowImage{
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
	a.URLItem = &ShowURI{
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
