package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
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
	sessionsDir = flag.String("data", "data", "path to the folder with sessions data")
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

	//sessions, err = collectSessions(*sessionsDir, &moves)
	//if err != nil {
	//	log.Fatal(err)
	//}

	fileStore = NewFileStore("data/uploads")

	sessionsStore, err = NewSessionStore("data/sessions.json")
	if err != nil {
		log.Fatal(err)
	}

	// Routes

	r := gin.New()

	//r.SetFuncMap(template.FuncMap{
	//	"plus":      plus,
	//	"increment": increment,
	//	"basename":  basename,
	//})
	//r.LoadHTMLGlob("templates/*.html")

	r.Use(allowCORS)

	// JSON: robot API
	r.GET("/pepper/initiate", initiateHandler)
	r.POST("/pepper/send_command", sendCommandHandler)

	// Static assets
	r.Static("/assets/", "assets")
	//r.Static(fmt.Sprintf("/%s/", *sessionsDir), *sessionsDir)
	r.Static("/data", "data")

	// JSON: UI API
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

	//r.GET("/api/session_items/:id", getSessionItemJSONHandler)

	r.GET("/api/moves/", movesJSONHandler)
	r.GET("/api/moves/:id", getMoveJSONHandler)
	r.GET("/api/move_groups/", moveGroupsJSONHandler)
	//r.GET("/api/auth/", authJSONHandler)

	// HTML: user GUI
	//r.GET("/sessions/:id", sessionsHandler)
	//r.GET("/sessions/", sessionsHandler)
	////r.GET("/manual/", pageHandler("Manual"))
	//r.GET("/about/", pageHandler("About"))
	//r.GET("/", homeHandler)

	log.Fatal(r.Run(*servingAddr))
}

func allowCORS(c *gin.Context) {
	c.Writer.Header().Add("Access-Control-Allow-Origin", "*")
	c.Writer.Header().Add("Access-Control-Allow-Methods", "GET, PUT, POST, DELETE, OPTIONS")
	c.Writer.Header().Add("Access-Control-Allow-Headers", "Content-Type")
}

// Template Helpers

//func plus(a, b int) int {
//	return a + b
//}
//
//func increment(a int) int {
//	return a + 1
//}
//
//func basename(s string) string {
//	return path.Base(s)
//}

// Handlers

func sendCommandHandler(c *gin.Context) {
	var form SendCommandForm
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

	log.Printf("form: %+v", form)

	// We can receive two types of instructions: SayCommand and MoveCommand. In the first case,
	// we just respond with OK status and the web browser will play an audio file for the
	// instruction. If something is wrong, we reply with error and the sound won't be played. In the second case,
	// we push the motion command to a web socket for Pepper to execute.

	var curInstruction Instruction
	curInstruction = sessionsStore.GetInstruction(form.ItemID)
	if curInstruction.IsNil() {
		curInstruction = moves.GetByID(form.ItemID)
	}

	if curInstruction.IsNil() {
		c.JSON(http.StatusNotFound, gin.H{
			"error":  fmt.Sprintf("can't find an instruction with the ID %s", form.ItemID),
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

	log.Printf("curInstruction: %s", curInstruction)

	// TODO: process the command: execute moves remotely
	// TODO: implement delay for some moves

	if err = sendInstruction(curInstruction, wsConnection); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "the command has been sent"})
}

//func homeHandler(c *gin.Context) {
//	c.Redirect(http.StatusTemporaryRedirect, "/sessions/")
//}

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

//func sessionsHandler(c *gin.Context) {
//	activeMenu("Sessions", siteMenuItems)
//
//	var curSessionID uuid.UUID
//	var curSessionName string
//
//	curSessionID, _ = uuid.Parse(c.Param("id"))
//
//	for _, v := range sessionsStore.Sessions {
//		if v.ID == curSessionID {
//			curSessionName = v.Name
//			break
//		}
//	}
//
//	c.HTML(http.StatusOK, "index.html", gin.H{
//		"title":              "Sessions",
//		"currentSessionID":   curSessionID,
//		"currentSessionName": curSessionName,
//		"sessions":           sessionsStore.Sessions,
//		"moves":              moves,
//		"moveGroups":         moveGroups,
//		"siteMenu":           siteMenuItems,
//		"userMenu":           userMenuItems,
//	})
//}

func sessionsJSONHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"data": sessionsStore.Sessions,
	})
}

func updateSessionJSONHandler(c *gin.Context) {
	//id := c.Param("id")
	//session, err := sessionsStore.Get(id)
	//if err != nil {
	//	c.JSON(http.StatusNotFound, gin.H{
	//		"error": err.Error(),
	//	})
	//	return
	//}

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

//func getSessionItemJSONHandler(c *gin.Context) {
//	id := c.Param("id")
//	sessionItem, err := sessionsStore.Get(id)
//	if err != nil {
//		c.JSON(http.StatusNotFound, gin.H{
//			"error": err.Error(),
//		})
//		return
//	}
//
//	c.JSON(http.StatusOK, gin.H{
//		"data": sessionItem,
//	})
//}

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

// pageHandler is a handler for any simple HTML page.
// The templateName must match the base name of the HTML-template itself, it's case sensitive.
//func pageHandler(templateName string) func(*gin.Context) {
//	return func(c *gin.Context) {
//		activeMenu(templateName, siteMenuItems)
//		c.HTML(http.StatusOK, fmt.Sprintf("%s.html", templateName), gin.H{
//			"title":    templateName,
//			"siteMenu": siteMenuItems,
//			"userMenu": userMenuItems,
//		})
//	}
//}

// Website Menu

//func activeMenu(title string, items []*MenuItem) {
//	for _, item := range items {
//		if item.Title == title {
//			item.IsActive = true
//		} else {
//			item.IsActive = false
//		}
//	}
//}

type MenuItem struct {
	Title    string
	Link     string
	IsActive bool
}

//var siteMenuItems = []*MenuItem{
//	{
//		Title:    "Sessions",
//		Link:     "/sessions/",
//		IsActive: false,
//	},
//	//{
//	//	Title:    "Manual",
//	//	Link:     "/manual/",
//	//	IsActive: false,
//	//},
//	{
//		Title:    "About",
//		Link:     "/about/",
//		IsActive: false,
//	},
//}
//
//var userMenuItems = []*MenuItem{
//	{
//		Title:    "Log out",
//		Link:     "/logout/",
//		IsActive: false,
//	},
//}

// Forms and JSON requests the app needs to handle

type SendCommandForm struct {
	//SessionID uuid.UUID  `json:"session_id" binding:"required"`
	ItemID uuid.UUID `json:"item_id" binding:"required"`
	// ItemType specifies on of the possible values: question, positive-answer, negative-answer.
	//ItemType  string `json:"item_type" binding:"required"`
}

// Files Store

type FileStore struct {
	base string
}

func NewFileStore(base string) *FileStore {
	return &FileStore{
		base: base,
	}
}

func (s *FileStore) Save(name string, src io.Reader) (string, error) {
	dst := path.Join(s.base, name)

	f, err := os.Create(dst)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err = io.Copy(f, src); err != nil {
		return "", err
	}

	return dst, nil
}

func (s *FileStore) Get(name string) (*os.File, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	return f, nil
}
