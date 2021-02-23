package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"

	"github.com/iharsuvorau/garlic/instruction"
	"github.com/iharsuvorau/garlic/store"
)

// TODO: communicate over WSS
// TODO: is there a need to keep WS connection live using ping-pong?
// TODO: implement basic auth and logout
// TODO: make abstraction separation clearer between Session and Images, Audio and Files
// TODO: make so that SayItem is not compulsory, any Action could be there
// NOTE: another approach to files: don't send them with each request, but serve as static files and send only URL
// TODO: check for duplicated IDs for any item on create step, creating a store

// App-wide variables
var (
	// wsUpgrader is needed to use WebSocket
	wsUpgrader = websocket.Upgrader{}

	// wsConnection keeps the current WebSocket connection.
	// Only one connection can be at the moment with the Pepper robot.
	wsConnection *websocket.Conn
	wsMu         sync.Mutex

	fileStore     *store.Files
	sessionsStore *store.Sessions
	moveStore     *store.Moves
	audioStore    *store.Audio
	actionsStore  *store.Actions

	pepperStatus uint8 // 0 -- disconnected, 1 -- connected
)

// CLI arguments
var (
	servingAddr = flag.String("addr", "0.0.0.0:8080", "http service address")
	motionsDir  = flag.String("moves", "data/pepper-core-anims-master", "path to the folder with moves")
	// TODO: data dir flag instead of motionsDir
)

func main() {
	flag.Parse()

	// initialization of essential variables
	var err error
	if ip, err := externalIP(); err == nil {
		log.Printf("IP of the machine: %v", ip)
	} else {
		log.Printf("failed to get IP of the machine: %v", err)
	}
	wsUpgrader.CheckOrigin = func(r *http.Request) bool { return true }

	// creating new stores or loading ones which exist
	fileStore = store.NewFileStore("data/uploads")
	sessionsStore, err = store.NewSessionStore("data/sessions.json")
	if err != nil {
		log.Fatal(err)
	}
	moveStore, err = store.NewMoveStore("data/moves.json", *motionsDir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("%v moves in the database", len(moveStore.Moves))
	audioStore, err = store.NewAudioStore("data/audio.json")
	if err != nil {
		log.Fatal(err)
	}
	actionsStore, err = store.NewActionsStore("data/actions.json")
	if err != nil {
		log.Fatal(err)
	}

	engine := newEngine()
	log.Fatal(engine.Run(*servingAddr))
}

func newEngine() *gin.Engine {
	// main router
	r := gin.New()
	// middleware
	r.Use(allowCORS, logRequest)
	// static assets
	r.Static("/data", "data")
	r.GET("/tmp/:name", serveCleanlyHandler)

	// pepper communication
	r.GET("/api/pepper/initiate", initiateHandler)
	r.GET("/api/pepper/status", pepperStatusJSONHandler)
	r.POST("/api/pepper/send_command", sendCommandHandler)
	r.OPTIONS("/api/pepper/send_command", emptyResponseOK)

	// sessions management
	r.GET("/api/sessions/", sessionsJSONHandler)
	r.POST("/api/sessions/", createSessionJSONHandler)
	r.OPTIONS("/api/sessions/", emptyResponseOK)
	r.GET("/api/sessions/:id/export", exportSessionJSONHandler)
	r.OPTIONS("/api/sessions/:id/export", emptyResponseOK)
	r.GET("/api/sessions/:id", getSessionJSONHandler)
	r.PUT("/api/sessions/:id", updateSessionJSONHandler)
	r.DELETE("/api/sessions/:id", deleteSessionJSONHandler)
	r.OPTIONS("/api/sessions/:id", emptyResponseOK)
	r.GET("/api/session_items/:id", getSessionItemJSONHandler)
	r.OPTIONS("/api/session_items/:id", emptyResponseOK)

	// ?
	r.GET("/api/instruction/:id", getInstructionJSONHandler)
	r.DELETE("/api/instruction/:id", deleteInstructionJSONHandler)
	r.OPTIONS("/api/instruction/:id", emptyResponseOK)

	// general upload API
	r.POST("/api/upload/audio", audioUploadJSONHandler)
	r.OPTIONS("/api/upload/audio", emptyResponseOK)
	r.DELETE("/api/upload/audio", deleteUploadJSONHandler)
	r.POST("/api/upload/image", imageUploadJSONHandler)
	r.OPTIONS("/api/upload/image", emptyResponseOK)
	r.DELETE("/api/upload/image", deleteUploadJSONHandler)
	r.POST("/api/upload/move", moveUploadJSONHandler)
	r.OPTIONS("/api/upload/move", emptyResponseOK)

	// serving moveStore
	r.GET("/api/moves/", movesJSONHandler)
	r.GET("/api/moves/:id", getMoveJSONHandler)
	r.DELETE("/api/moves/:id", deleteMoveJSONHandler)
	r.OPTIONS("/api/moves/:id", emptyResponseOK)

	// serving audioStore
	r.GET("/api/audio/", audioJSONHandler)
	r.POST("/api/audio/", createAudioJSONHandler)
	r.OPTIONS("/api/audio/", emptyResponseOK)
	r.GET("/api/audio/:id", getAudioJSONHandler)
	r.DELETE("/api/audio/:id", deleteAudioJSONHandler)
	r.OPTIONS("/api/audio/:id", emptyResponseOK)

	// serving actionsStore
	r.GET("/api/actions/", actionsJSONHandler)
	r.POST("/api/actions/", createActionJSONHandler)
	r.OPTIONS("/api/actions/", emptyResponseOK)
	r.DELETE("/api/actions/:id", deleteActionJSONHandler)
	r.OPTIONS("/api/actions/:id", emptyResponseOK)

	// utilities: helpful endpoints for the client application or other
	r.GET("/api/move_groups/", moveGroupsJSONHandler)
	r.GET("/api/server_ip", getServerIPJSONHandler)

	// main fallback: nothing to serve here at the moment, but could be a place for API documentation
	r.GET("/", emptyResponseOK)

	return r
}

// Middleware

func allowCORS(c *gin.Context) {
	c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Add("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
	c.Writer.Header().Add("Access-Control-Allow-Headers", "Content-Type")
}

func logRequest(c *gin.Context) {
	log.Printf("%s %s %s", c.Request.Method, c.Request.RequestURI, c.Request.RemoteAddr)
}

// Handlers

func emptyResponseOK(c *gin.Context) {
	c.String(http.StatusOK, "")
}

func deleteUploadJSONHandler(c *gin.Context) {
	form := struct {
		Filepath string `json:"filepath"`
	}{}
	err := c.ShouldBindJSON(&form)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	err = fileStore.Delete(form.Filepath)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "file has been deleted"})
}

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

	// We can receive three types of instruction: SayCommand, MoveCommand, ShowImageCommand.
	// In the first case, we just respond with OK status and the web browser will play an audio file for the instruction.
	// If something is wrong, we reply with error and the sound won't be played.
	// In the second and third cases, we push the command to a web socket for Pepper to execute.
	var action instruction.Instruction
	action = sessionsStore.GetAction(form.ItemID)
	if action.IsNil() {
		action, _ = moveStore.GetByUUID(form.ItemID)
	}
	if action.IsNil() {
		action, _ = actionsStore.GetByUUID(form.ItemID)
	}
	if action.IsNil() {
		action, _ = audioStore.GetByUUID(form.ItemID)
	}
	if action.IsNil() {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  fmt.Sprintf("can't find the instruction with the ID %s", form.ItemID),
			"method": "sendCommandHandler",
		})
		return
	}
	if !action.IsValid() || action.IsNil() {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":  "got an invalid or empty instruction",
			"method": "sendCommandHandler",
		})
		return
	}
	if err = instruction.SendInstruction(action, wsConnection, &wsMu); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "the command has been sent"})
}

func initiateHandler(c *gin.Context) {
	// PepperIncomingMessage is used to parse requests from the Android application on the Pepper's side.
	// It sends available built-in motions when starts itself, so the webserver can register these motions
	// and give a user an option to use built-in motions.
	log.Printf("establishing a websocket connection")

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
	log.Printf("websocket connection has been established with %s", c.Request.RemoteAddr)

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
	var updatedSession store.Session
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

func exportSessionJSONHandler(c *gin.Context) {
	id := c.Param("id")
	session, err := sessionsStore.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	relativePath, err := session.Export("tmp")
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": err.Error(),
		})
		log.Println(err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "session has been exported",
		"relative_path": relativePath,
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
	var newSession *store.Session

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
	// NOTE: using form data instead of JSON because of file upload in this handler

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

	// NOTE: we don't create an object in Audio, Audio only for commonly used audio items,
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
	action := &instruction.Say{
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
	move := &instruction.Move{
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

func imageUploadJSONHandler(c *gin.Context) {
	// NOTE: using form data instead of JSON because of file upload in this handler

	f, fh, err := c.Request.FormFile("file_content")
	if err != nil {
		log.Printf("imageUploadJSONHandler: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	defer f.Close()

	uid := uuid.Must(uuid.NewRandom())
	ext := filepath.Ext(fh.Filename)
	name := uid.String() + ext
	dst, err := fileStore.Save(name, f)
	if err != nil {
		log.Printf("imageUploadJSONHandler, can't save the file %v: %v", name, err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	//imageName := c.DefaultPostForm("name", "")
	//if imageName == "" {
	//	imageName = strings.Replace(fh.Filename, filepath.Ext(fh.Filename), "", -1)
	//}
	//group := c.DefaultPostForm("group", "")
	//if group == "" {
	//	group = "Default"
	//}
	//image := &ShowImage{
	//	ID:       uid,
	//	Name:     imageName,
	//	FilePath: dst,
	//	Group:    group,
	//}
	//if err = imageStore.Create(image); err != nil {
	//	c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("failed to create an image: %v", err)})
	//	return
	//}

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
	instruction := sessionsStore.GetAction(uid)
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

func actionsJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": actionsStore.Items,
	})
}

func createActionJSONHandler(c *gin.Context) {
	newAction := new(instruction.Action)

	err := c.ShouldBindJSON(&newAction)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": err.Error(),
		})
		return
	}

	err = actionsStore.Create(newAction)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error": fmt.Errorf("failed to create an action: %v", err),
		})
		log.Print(fmt.Errorf("failed to create an action: %v", err))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "action has been created successfully",
	})
}

func deleteActionJSONHandler(c *gin.Context) {
	id := c.Param("id")
	err := actionsStore.Delete(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "action has been deleted successfully",
	})
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

func getServerIPJSONHandler(c *gin.Context) {
	ip, err := getOutboundIP()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "cannot get server IP address"})
	}

	c.JSON(http.StatusOK, gin.H{"data": ip})
}

func serveCleanlyHandler(c *gin.Context) {
	name := c.Param("name")
	fpath := path.Join("tmp", name)
	c.File(fpath)
	defer func() {
		err := os.RemoveAll("tmp")
		if err != nil {
			log.Printf("failed to remove ./tmp: %v", err)
		}
	}()
}

// Helpers

func makeMoveActionsFromNames(names []string, group string) []*instruction.Move {
	moves := []*instruction.Move{}
	for _, n := range names {
		moves = append(moves, &instruction.Move{
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

func getOutboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)
	return localAddr.IP.String(), err
}
