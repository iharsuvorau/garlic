package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io/ioutil"
	"log"
	"net/http"
	"path/filepath"
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

	moves, err = collectMoves(*motionsDir)
	if err != nil {
		log.Fatal(err)
	}

	moveGroups = moves.GetGroups()

	fileStore = NewFileStore("data/uploads")

	sessionsStore, err = NewSessionStore("data/sessions.json")
	if err != nil {
		log.Fatal(err)
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

	r.POST("/api/upload/audio", audioUploadJSONHandler)
	r.OPTIONS("/api/upload/audio", func(c *gin.Context) {
		c.String(http.StatusOK, "")
	})

	r.GET("/api/moves/", movesJSONHandler)
	r.GET("/api/moves/:id", getMoveJSONHandler)
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
	var status int8
	if wsConnection != nil {
		status = 1
	}
	c.JSON(http.StatusOK, gin.H{"status": status})
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
		log.Printf("no ID in sessions store: %v", form.ItemID)
		curInstruction = moves.GetByID(form.ItemID)
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
	var err error
	wsConnection, err = wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		log.Println(err)
	}
	defer wsConnection.Close()
	for {
		_, message, err := wsConnection.ReadMessage()
		if err != nil {
			log.Println("read:", err)
			break
		}

		m := PepperIncomingMessage{}
		err = json.Unmarshal(message, &m)
		if err != nil {
			log.Println("unmarshal:", err)
		}
		if len(m.Moves) > 0 {
			log.Println("moves before", len(moves))
			moves.AddMoves("Remote", m.Moves)
			log.Println("new moves has been added")
			log.Println("moves after", len(moves))
			moveGroups = append(moveGroups, "Remote")
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

	c.JSON(http.StatusOK, gin.H{
		"message":  "file has been uploaded successfully",
		"id":       uid,
		"filepath": dst,
	})
}

func movesJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": moves,
	})
}

func getMoveJSONHandler(c *gin.Context) {
	id := c.Param("id")
	move, err := Moves(moves).GetByStringID(id)
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

func moveGroupsJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": moveGroups,
	})
}
