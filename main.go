package main

import (
	"errors"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"path/filepath"
	"strings"
)

// TODO: communicate over WSS
// TODO: is there a need to keep WS connection live using ping-pong?
// TODO: implement basic auth and logout

// App-wide variables
var (
	// wsUpgrader is needed to use WebSocket
	wsUpgrader = websocket.Upgrader{}

	// wsConnection keeps the current WebSocket connection.
	// Only one connection can be at the moment with the Pepper robot.
	wsConnection *websocket.Conn

	fileStore     *FileStore
	sessionsStore *SessionStore
	moveStore     *MoveStore
	audioStore    *SayStore
	pepperStatus  uint8 // 0 -- disconnected, 1 -- connected
)

// CLI arguments
var (
	servingAddr = flag.String("addr", ":8080", "http service address")
	motionsDir  = flag.String("moves", "data/pepper-core-anims-master", "path to the folder with moves")
)

func main() {
	flag.Parse()

	// Initialization of essential variables

	var err error

	wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	fileStore = NewFileStore("data/uploads")

	sessionsStore, err = NewSessionStore("data/sessions.json")
	if err != nil {
		log.Fatal(err)
	}

	moveStore, err = NewMoveStore("data/moves.json", *motionsDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v moves in the database", len(moveStore.Moves))

	audioStore, err = NewSayStore("data/audio.json")
	if err != nil {
		log.Fatal(err)
	}

	if ip, err := externalIP(); err == nil {
		log.Printf("IP of the machine: %v", ip)
	} else {
		log.Printf("failed to get IP of the machine: %v", err)
	}

	// Routes

	r := gin.New()

	// Middleware
	r.Use(allowCORS)

	// JSON: robot API
	r.GET("/pepper/initiate", initiateHandler)
	//r.POST("/pepper/send_command", sendCommandHandler)
	//r.OPTIONS("/pepper/send_command", func(c *gin.Context) {
	//	c.String(http.StatusOK, "")
	//})

	// Static assets
	r.Static("/data", "data")

	// JSON: UI API

	r.GET("/api/pepper/status", pepperStatusJSONHandler)
	r.POST("/api/pepper/send_command", sendCommandHandler)
	r.OPTIONS("/api/pepper/send_command", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/sessions/", sessionsJSONHandler)
	r.POST("/api/sessions/", createSessionJSONHandler)
	r.OPTIONS("/api/sessions/", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/sessions/:id", getSessionJSONHandler)
	r.PUT("/api/sessions/:id", updateSessionJSONHandler)
	r.DELETE("/api/sessions/:id", deleteSessionJSONHandler)
	r.OPTIONS("/api/sessions/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/session_items/:id", getSessionItemJSONHandler)
	r.OPTIONS("/api/session_items/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/instructions/:id", getInstructionJSONHandler)
	r.DELETE("/api/instructions/:id", deleteInstructionJSONHandler)
	r.OPTIONS("/api/instructions/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.POST("/api/upload/audio", audioUploadJSONHandler)
	r.OPTIONS("/api/upload/audio", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.POST("/api/upload/move", moveUploadJSONHandler)
	r.OPTIONS("/api/upload/move", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/moves/", movesJSONHandler)
	r.GET("/api/moves/:id", getMoveJSONHandler)
	r.DELETE("/api/moves/:id", deleteMoveJSONHandler)
	r.OPTIONS("/api/moves/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/audio/", audioJSONHandler)
	r.POST("/api/audio/", createAudioJSONHandler)
	r.OPTIONS("/api/audio/", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})
	r.GET("/api/audio/:id", getAudioJSONHandler)
	r.DELETE("/api/audio/:id", deleteAudioJSONHandler)
	r.OPTIONS("/api/audio/:id", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/move_groups/", moveGroupsJSONHandler)
	//r.GET("/api/auth/", authJSONHandler)

	r.GET("/", func(c *gin.Context) {
		c.String(http.StatusOK, "Pepper webserver")
	})

	log.Fatal(r.Run(*servingAddr))
}

// Middleware

func allowCORS(c *gin.Context) {
	c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Add("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
	c.Writer.Header().Add("Access-Control-Allow-Headers", "Content-Type")
}

// Handlers

func pepperStatusJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": pepperStatus})
}

func sendCommandHandler(c *gin.Context) {
	form := struct {
		ItemID uuid.UUID `json:"item_id" binding:"required"`
	}{}
	err := c.BindJSON(&form)
	if err != nil {
		defer c.Request.Body.Close()

		b, e := ioutil.ReadAll(c.Request.Body)
		if e != nil {
			c.JSON(http.StatusBadRequest, gin.H{"method": "sendCommandHandler", "error": e.Error()})
			return
		}

		c.JSON(http.StatusBadRequest, gin.H{
			"method":  "sendCommandHandler",
			"error":   err.Error(),
			"request": fmt.Sprintf("%s", b),
		})
		return
	}

	// We can receive two types of instructions: SayCommand and MoveCommand. In the first case,
	// we just respond with OK status and the web browser will play an audio file for the
	// instruction. If something is wrong, we reply with error and the sound won't be played. In the second case,
	// we push the motion command to a web socket for Pepper to execute.
	var curInstruction Instruction
	curInstruction = sessionsStore.GetInstruction(form.ItemID)
	if curInstruction.IsNil() {
		curInstruction, _ = moveStore.GetByUUID(form.ItemID)
	}
	if curInstruction.IsNil() {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  fmt.Sprintf("can't find the instruction with the ID %s", form.ItemID),
			"method": "sendCommandHandler",
		})
		return
	}
	if !curInstruction.IsValid() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "got an invalid instruction",
			"method": "sendCommandHandler",
		})
		return
	}
	if err = sendInstruction(curInstruction, wsConnection); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "the command has been sent"})
}

func initiateHandler(c *gin.Context) {
	// PepperIncomingMessage is used to parse requests from the Android application on the Pepper's side.
	// It sends available built-in motions when starts itself, so the webserver can register these motions
	// and give a user an option to use built-in motions.
	type PepperIncomingMessage struct {
		Moves []string `json:"moves"`
	}

	var err error
	wsConnection, err = wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
	}
	defer wsConnection.Close()

	pepperStatus = 1

	wsConnection.SetCloseHandler(func(code int, text string) error {
		log.Printf("websocket connection close handler: %v, %v", code, text)
		pepperStatus = 0
		return nil
	})

	for {
		m := PepperIncomingMessage{}
		if err := wsConnection.ReadJSON(&m); err != nil {
			log.Printf("failed to read a message from Pepper: %v", err)
			pepperStatus = 0
			break
		}
		if len(m.Moves) > 0 {
			remoteMoves := makeMoveActionsFromNames(m.Moves, "Remote")
			moveStore.AddMany(remoteMoves)
		}
	}
}

func sessionsJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": sessionsStore.Sessions,
	})
}

func updateSessionJSONHandler(c *gin.Context) {
	var updatedSession Session
	err := c.BindJSON(&updatedSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	err = sessionsStore.Update(&updatedSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "session has been saved successfully",
	})
}

func getSessionJSONHandler(c *gin.Context) {
	id := c.Param("id")
	session, err := sessionsStore.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": session,
	})
}

func deleteSessionJSONHandler(c *gin.Context) {
	id := c.Param("id")

	err := sessionsStore.Delete(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "session has been deleted",
	})
}

func createSessionJSONHandler(c *gin.Context) {
	var newSession *Session

	err := c.ShouldBindJSON(&newSession)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		log.Printf("session: %+v", newSession)
		return
	}

	err = sessionsStore.Create(newSession)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Errorf("failed to create a session: %v", err),
		})
		log.Print(fmt.Errorf("failed to create a session: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "session has been created successfully",
	})
}

func audioUploadJSONHandler(c *gin.Context) {
	f, fh, err := c.Request.FormFile("file_content")
	if err != nil {
		log.Printf("audioUploadJSONHandler: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	uid := uuid.Must(uuid.NewRandom())
	ext := filepath.Ext(fh.Filename)
	name := uid.String() + ext
	dst, err := fileStore.Save(name, f)
	if err != nil {
		log.Printf("audioUploadJSONHandler, can't save the file %v: %v", name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// NOTE: we don't create an object in SayStore, SayStore only for commonly used audio items,
	// session audio items should belong only to a session.

	c.JSON(http.StatusOK, gin.H{
		"message":  "file has been uploaded successfully",
		"id":       uid,
		"filepath": dst,
	})
}

func createAudioJSONHandler(c *gin.Context) {
	f, fh, err := c.Request.FormFile("file_content")
	if err != nil {
		log.Printf("createAudioJSONHandler: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	uid := uuid.Must(uuid.NewRandom())
	ext := filepath.Ext(fh.Filename)
	name := uid.String() + ext
	dst, err := fileStore.Save(name, f)
	if err != nil {
		log.Printf("createAudioJSONHandler, can't save the file %v: %v", name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	phrase := c.DefaultPostForm("phrase", "")
	if phrase == "" {
		phrase = strings.Replace(fh.Filename, filepath.Ext(fh.Filename), "", -1)
	}
	group := c.DefaultPostForm("group", "")
	if group == "" {
		group = "Default"
	}
	action := &SayAction{
		ID:       uid,
		Phrase:   phrase,
		FilePath: dst,
		Group:    group,
		Delay:    0,
	}
	if err = audioStore.Create(action); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create an audio: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "audio has been created successfully",
		"id":       uid,
		"filepath": dst,
	})
}

func moveUploadJSONHandler(c *gin.Context) {
	// NOTE: using form data instead of JSON because of file upload in this handler

	f, fh, err := c.Request.FormFile("file_content")
	if err != nil {
		log.Printf("moveUploadJSONHandler: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	uid := uuid.Must(uuid.NewRandom())
	ext := filepath.Ext(fh.Filename)
	name := uid.String() + ext
	dst, err := fileStore.Save(name, f)
	if err != nil {
		log.Printf("moveUploadJSONHandler, can't save the file %v: %v", name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	moveName := c.DefaultPostForm("name", "")
	if moveName == "" {
		moveName = strings.Replace(fh.Filename, filepath.Ext(fh.Filename), "", -1)
	}
	moveGroup := c.DefaultPostForm("group", "")
	if moveGroup == "" {
		moveGroup = "Default"
	}
	move := &MoveAction{
		ID:       uid,
		Name:     moveName,
		FilePath: dst,
		Group:    moveGroup,
	}
	if err = moveStore.Create(move); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create a move: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":  "file has been uploaded successfully",
		"id":       uid,
		"filepath": dst,
	})
}

func movesJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": moveStore.Moves,
	})
}

func getMoveJSONHandler(c *gin.Context) {
	id := c.Param("id")
	move, err := moveStore.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": move,
	})
}

func deleteMoveJSONHandler(c *gin.Context) {
	id := c.Param("id")
	err := moveStore.Delete(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "motion has been deleted successfully",
	})
}

func moveGroupsJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": moveStore.GetGroups(),
	})
}

func audioJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": audioStore.Items,
	})
}

func getAudioJSONHandler(c *gin.Context) {
	id := c.Param("id")
	item, err := audioStore.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data": item,
	})
}

func deleteAudioJSONHandler(c *gin.Context) {
	id := c.Param("id")
	err := audioStore.Delete(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "audio file has been deleted successfully",
	})
}

func getInstructionJSONHandler(c *gin.Context) {
	id := c.Param("id")
	uid, err := uuid.Parse(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}
	instruction := sessionsStore.GetInstruction(uid)
	c.JSON(http.StatusOK, gin.H{"data": instruction})
}

func deleteInstructionJSONHandler(c *gin.Context) {
	id := c.Param("id")
	err := sessionsStore.DeleteInstruction(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "action has been deleted"})
}

func getSessionItemJSONHandler(c *gin.Context) {
	id := c.Param("id")
	item, err := sessionsStore.GetItem(id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{"data": item})
}

// Helpers

func makeMoveActionsFromNames(names []string, group string) []*MoveAction {
	moves := []*MoveAction{}
	for _, n := range names {
		moves = append(moves, &MoveAction{
			ID:       uuid.Must(uuid.NewRandom()),
			Name:     n,
			FilePath: "",
			Delay:    0,
			Group:    group,
		})
	}
	return moves
}

func externalIP() (string, error) {
	ifaces, err := net.Interfaces()
	if err != nil {
		return "", err
	}
	for _, iface := range ifaces {
		if iface.Flags&net.FlagUp == 0 {
			continue // interface down
		}
		if iface.Flags&net.FlagLoopback != 0 {
			continue // loopback interface
		}
		addrs, err := iface.Addrs()
		if err != nil {
			return "", err
		}
		for _, addr := range addrs {
			var ip net.IP
			switch v := addr.(type) {
			case *net.IPNet:
				ip = v.IP
			case *net.IPAddr:
				ip = v.IP
			}
			if ip == nil || ip.IsLoopback() {
				continue
			}
			ip = ip.To4()
			if ip == nil {
				continue // not an ipv4 address
			}
			return ip.String(), nil
		}
	}
	return "", errors.New("are you connected to the network?")
}
