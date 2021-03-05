package store

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"path"
	"strings"
	"sync"

	"github.com/google/uuid"

	"github.com/iharsuvorau/garlic/instruction"
)

// Session represents a session with a child, a set of questions and simple answers which
// are accompanied with moves by a robot to make the conversation a lively one.
type Session struct {
	ID          uuid.UUID      `json:"ID" form:"ID"`
	Name        string         `json:"Name" form:"Name" binding:"required"`
	Description string         `json:"Description" form:"Description"`
	Items       []*SessionItem `json:"Items" form:"Items"`
}

func (s *Session) initializeIDs() {
	if s == nil {
		return
	}

	if (s.ID == uuid.UUID{}) {
		s.ID = uuid.Must(uuid.NewRandom())
	}

	for _, item := range s.Items {
		if item == nil {
			continue
		}

		if (item.ID == uuid.UUID{}) {
			item.ID = uuid.Must(uuid.NewRandom())
		}

		for _, action := range item.Actions {
			if action == nil {
				continue
			}
			if (action.ID == uuid.UUID{}) {
				action.ID = uuid.Must(uuid.NewRandom())
			}
			if action.SayItem != nil {
				if (action.SayItem.ID == uuid.UUID{}) {
					action.SayItem.ID = uuid.Must(uuid.NewRandom())
				}
			}
			if action.MoveItem != nil {
				if (action.MoveItem.ID == uuid.UUID{}) {
					action.MoveItem.ID = uuid.Must(uuid.NewRandom())
				}
			}
			if action.ImageItem != nil {
				if (action.ImageItem.ID == uuid.UUID{}) {
					action.ImageItem.ID = uuid.Must(uuid.NewRandom())
				}
			}
		}
	}
}

func (s *Session) Export(dir string) (archivePath string, err error) {
	archiveFiles := []string{}

	// create subdirectory
	subDirName := path.Join(dir, s.ID.String())
	uploadsName := path.Join(subDirName, "uploads")
	err = os.MkdirAll(uploadsName, 0777)
	if err != nil {
		return
	}
	defer func() {
		err := os.RemoveAll(subDirName)
		if err != nil {
			log.Println("failed to remove sub directory after exporting the session", s.ID, err)
		}
	}() // cleaning up

	// export JSON data of the session
	const sessionFileName = "session.json"
	name := path.Join(subDirName, sessionFileName)
	f, err := os.Create(name)
	if err != nil {
		return
	}
	if err = json.NewEncoder(f).Encode(s); err != nil {
		return
	}
	if err := f.Close(); err != nil {
		log.Println("session export error on session.json close:", err)
	}
	archiveFiles = append(archiveFiles, name) // keeping track of archive files

	// export user data assets: uploads (images, sounds, moves)
	userAssets := []string{}
	for _, item := range s.Items {
		for _, assetPath := range item.LocateAssets() {
			if strings.HasPrefix(assetPath, "data/uploads") {
				userAssets = append(userAssets, assetPath)
			}
		}
	}

	// collect assets
	for _, asset := range userAssets {
		newAsset := strings.Replace(asset, "data", subDirName, 1)
		if err = copyFile(asset, newAsset); err != nil {
			return
		}
		archiveFiles = append(archiveFiles, newAsset) // keeping track of archive files
	}

	// archive files
	archivePath = subDirName + ".zip"
	f, err = os.Create(archivePath)
	if err != nil {
		return
	}
	w := zip.NewWriter(f)
	for _, asset := range archiveFiles {
		// expected path transformation: <subDirName>/uploads/<file-UUID>.qianim -> uploads/UUID.qianim
		assetInArchivePath := strings.Replace(asset, subDirName+"/", "", 1)
		var wf io.Writer
		wf, err = w.Create(assetInArchivePath)
		if err != nil {
			return
		}

		var b []byte
		b, err = ioutil.ReadFile(asset)
		if err != nil {
			return
		}

		if _, err = wf.Write(b); err != nil {
			return
		}
	}
	if err = w.Close(); err != nil {
		return
	}
	if err = f.Close(); err != nil {
		log.Println("session export error on archive file close:", err)
	}
	return
}

// SessionItem represents a single unit of a session, it's a question and positive and negative
// answers accompanied with a robot's moves which are represented in the web UI as a set of buttons.
type SessionItem struct {
	ID      uuid.UUID
	Actions []*instruction.Action // the first item of Actions is the main item, usually, it's the main question
	// of the session item, other actions are some kind of conversation supportive answers
}

func (si *SessionItem) LocateAssets() []string {
	paths := []string{}
	for _, action := range si.Actions {
		paths = append(paths, action.LocateAssets()...)
	}
	return paths
}

type Sessions struct {
	Sessions []*Session

	filepath string
	mu       sync.RWMutex
}

func NewSessionStore(fpath string) (*Sessions, error) {
	var file *os.File
	_, err := os.Stat(fpath)
	if os.IsNotExist(err) {
		file, err = os.Create(fpath)
		if err != nil {
			return nil, fmt.Errorf("can't create a session store at %s: %v", fpath, err)
		}
	} else {
		file, err = os.Open(fpath)
	}
	defer file.Close()

	sessions := []*Session{}
	if err = json.NewDecoder(file).Decode(&sessions); err != nil && err != io.EOF {
		return nil, fmt.Errorf("can't decode sessions from %s: %v", fpath, err)
	}

	// create empty show-image actions for existing items
	for _, s := range sessions {
		for _, item := range s.Items {
			for _, action := range item.Actions {
				if action.ImageItem == nil {
					action.ImageItem = &instruction.ShowImage{}
				}
			}
		}
	}

	store := &Sessions{
		filepath: fpath,
		Sessions: sessions,
	}

	return store, nil
}

// GetAction looks for a top level instruction, which unites Say and Move actions
// and presents them as a union of two actions, so both actions should be executed.
func (s *Sessions) GetAction(id uuid.UUID) *instruction.Action {
	for _, session := range s.Sessions {
		for _, item := range session.Items {
			for _, action := range item.Actions {
				if action == nil {
					continue
				}

				if action.ID == id {
					return action
				}

				if action.SayItem != nil && action.SayItem.ID == id {
					return action
				}
				if action.MoveItem != nil && action.MoveItem.ID == id {
					return action
				}
				if action.ImageItem != nil && action.ImageItem.ID == id {
					return action
				}
			}
		}
	}
	return nil
}

func (s *Sessions) Get(id string) (*Session, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, s := range s.Sessions {
		if s.ID == uid {
			return s, nil
		}
	}

	return nil, fmt.Errorf("not found")
}

func (s *Sessions) GetItem(id string) (*SessionItem, error) {
	uid, err := uuid.Parse(id)
	if err != nil {
		return nil, err
	}

	for _, s := range s.Sessions {
		for _, item := range s.Items {
			if item.ID == uid {
				return item, nil
			}
		}
	}

	return nil, fmt.Errorf("not found")
}

//func (s *Sessions) FindAction(id string) (interface{}, error) {
//	uid, err := uuid.Parse(id)
//	if err != nil {
//		return nil, err
//	}
//	for _, session := range s.Sessions {
//		for _, item := range session.Items {
//			if item == nil {
//				continue
//			}
//			for _, action := range item.Actions {
//				if action == nil {
//					continue
//				}
//				if action.SayItem != nil {
//					if action.SayItem.ID == uid {
//						return action.SayItem, nil
//					}
//				}
//				if action.MoveItem != nil {
//					if action.MoveItem.ID == uid {
//						return action.MoveItem, nil
//					}
//				}
//			}
//		}
//	}
//	return nil, fmt.Errorf("not found")
//}

func (s *Sessions) Create(newSession *Session) error {
	newSession.initializeIDs()
	if s.isDuplicate(newSession) {
		return fmt.Errorf("cannot create a new session, duplicated ID: %v", newSession.ID)
	}
	s.mu.Lock()
	s.Sessions = append(s.Sessions, newSession)
	s.mu.Unlock()
	return s.dump()
}

func (s *Sessions) Update(updatedSession *Session) error {
	updatedSession.initializeIDs()
	for _, s := range s.Sessions {
		if s.ID == updatedSession.ID {
			*s = *updatedSession
		}
	}
	return s.dump()
}

func (s *Sessions) Delete(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	session, err := s.Get(id)
	if err != nil {
		return err
	}

	// removing all audio files attached to the session
	for _, item := range session.Items {
		if item == nil {
			continue
		}
		for _, action := range item.Actions {
			if action.SayItem != nil && action.SayItem.FilePath != "" {
				if err = removeFile(action.SayItem.FilePath); err != nil {
					return err
				}
			}
			if action.ImageItem != nil && action.ImageItem.FilePath != "" {
				if err = removeFile(action.ImageItem.FilePath); err != nil {
					return err
				}
			}
		}
	}

	newSessions := []*Session{}

	for _, s := range s.Sessions {
		if s.ID == uid {
			continue
		}
		newSessions = append(newSessions, s)
	}

	s.mu.Lock()
	s.Sessions = newSessions
	s.mu.Unlock()

	return s.dump()
}

func (s *Sessions) DeleteInstruction(id string) error {
	uid, err := uuid.Parse(id)
	if err != nil {
		return err
	}

	// removing instruction's files
	action := s.GetAction(uid)
	if action != nil {
		if action.SayItem != nil && len(action.SayItem.FilePath) > 0 {
			err = os.Remove(action.SayItem.FilePath)
			if err != nil {
				log.Println(fmt.Errorf("failed to remove audio from the action: %v", err))
			}
		}
		if action.ImageItem != nil && action.ImageItem.FilePath != "" {
			err = os.Remove(action.ImageItem.FilePath)
			if err != nil {
				log.Println(fmt.Errorf("failed to remove image %s from the action: %v", action.ImageItem.FilePath, err))
			}
		}
	}

	// removing the instruction
	for _, session := range s.Sessions {
		for _, item := range session.Items {
			for _, action := range item.Actions {
				if action.ID == uid {
					newActions := []*instruction.Action{}
					for _, action := range item.Actions {
						if action.ID == uid {
							continue
						}
						newActions = append(newActions, action)
					}
					s.mu.Lock()
					item.Actions = newActions
					s.mu.Unlock()
					break
				}
			}
		}
	}

	return s.dump()
}

func (s *Sessions) Import(fpath string, overwrite bool, fileStore *Files) error {
	r, err := zip.OpenReader(fpath)
	if err != nil {
		return fmt.Errorf("failed to open a reader for %s: %v", fpath, err)
	}
	defer r.Close()

	for _, f := range r.File {
		// creating a new or updating an existing session
		if f.Name == "session.json" {
			ff, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open %s at %s: %v", f.Name, fpath, err)
			}

			var session Session
			if err = json.NewDecoder(ff).Decode(&session); err != nil {
				return fmt.Errorf("failed to decode %s at %s: %v", f.Name, fpath, err)
			}
			err = ff.Close()
			if err != nil {
				return fmt.Errorf("failed to close %s at %s: %v", f.Name, fpath, err)
			}

			if overwrite && s.isDuplicate(&session) {
				err = s.Update(&session)
			} else {
				err = s.Create(&session)
			}
			if err != nil {
				return err
			}
		}

		// checking and uploading attached files
		if strings.HasPrefix(f.Name, "uploads/") {
			name := path.Base(f.Name)
			ff, err := f.Open()
			if err != nil {
				return fmt.Errorf("failed to open %s at %s: %v", f.Name, fpath, err)
			}

			if _, err = fileStore.Save(name, ff); err != nil {
				return fmt.Errorf("failed to save a file at %s: %v", name, err)
			}

			err = ff.Close()
			if err != nil {
				return fmt.Errorf("failed to close %s at %s: %v", f.Name, fpath, err)
			}
		}
	}

	return nil
}

func (s *Sessions) dump() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	f, err := os.Create(s.filepath)
	if err != nil {
		return fmt.Errorf("failed to open a file: %v", err)
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(s.Sessions)
}

func (s *Sessions) isDuplicate(ss *Session) bool {
	for _, v := range s.Sessions {
		if v.ID == ss.ID {
			return true
		}
	}
	return false
}

func removeFile(filePath string) error {
	err := os.Remove(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("tried to remove unexistent file: %v", filePath)
			err = nil
		} else {
			err = fmt.Errorf("failed to remove a file %s: %v", filePath, err)
		}
	}
	return err
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	if err != nil {
		return err
	}
	return out.Close()
}
