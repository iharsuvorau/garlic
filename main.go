package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/gorilla/websocket"
	"html/template"
	"io/ioutil"
	"log"
	"net/http"
	"path"
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

	sessions, err = collectSessions(*sessionsDir, &moves)
	if err != nil {
		log.Fatal(err)
	}

	// Routes

	r := gin.New()
	r.SetFuncMap(template.FuncMap{
		"plus":      plus,
		"increment": increment,
		"basename":  basename,
	})
	r.LoadHTMLGlob("templates/*.html")

	// JSON: robot API
	r.GET("/pepper/initiate", initiateHandler)
	r.POST("/pepper/send_command", sendCommandHandler)

	// Static assets
	r.Static("/assets/", "assets")
	r.Static(fmt.Sprintf("/%s/", *sessionsDir), *sessionsDir)

	// HTML: user GUI
	r.GET("/sessions/:id", sessionsHandler)
	r.GET("/sessions/", sessionsHandler)
	//r.GET("/manual/", pageHandler("Manual"))
	r.GET("/about/", pageHandler("About"))
	r.GET("/", homeHandler)

	log.Fatal(r.Run(*servingAddr))
}

// Template Helpers

func plus(a, b int) int {
	return a + b
}

func increment(a int) int {
	return a + 1
}

func basename(s string) string {
	return path.Base(s)
}

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
	curInstruction = Sessions(sessions).GetInstructionByID(form.ItemID)
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

func homeHandler(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, "/sessions/")
}

func initiateHandler(c *gin.Context) {
	var err error
	wsConnection, err = wsUpgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
}

func sessionsHandler(c *gin.Context) {
	activeMenu("Sessions", siteMenuItems)

	var curSessionID uuid.UUID
	var curSessionName string

	curSessionID, _ = uuid.Parse(c.Param("id"))

	for _, v := range sessions {
		if v.ID == curSessionID {
			curSessionName = v.Name
			break
		}
	}

	c.HTML(http.StatusOK, "index.html", gin.H{
		"title":              "Sessions",
		"currentSessionID":   curSessionID,
		"currentSessionName": curSessionName,
		"sessions":           sessions,
		"moves":              moves,
		"moveGroups":         moveGroups,
		"siteMenu":           siteMenuItems,
		"userMenu":           userMenuItems,
	})
}

// pageHandler is a handler for any simple HTML page.
// The templateName must match the base name of the HTML-template itself, it's case sensitive.
func pageHandler(templateName string) func(*gin.Context) {
	return func(c *gin.Context) {
		activeMenu(templateName, siteMenuItems)
		c.HTML(http.StatusOK, fmt.Sprintf("%s.html", templateName), gin.H{
			"title":    templateName,
			"siteMenu": siteMenuItems,
			"userMenu": userMenuItems,
		})
	}
}

// Website Menu

func activeMenu(title string, items []*MenuItem) {
	for _, item := range items {
		if item.Title == title {
			item.IsActive = true
		} else {
			item.IsActive = false
		}
	}
}

type MenuItem struct {
	Title    string
	Link     string
	IsActive bool
}

var siteMenuItems = []*MenuItem{
	{
		Title:    "Sessions",
		Link:     "/sessions/",
		IsActive: false,
	},
	//{
	//	Title:    "Manual",
	//	Link:     "/manual/",
	//	IsActive: false,
	//},
	{
		Title:    "About",
		Link:     "/about/",
		IsActive: false,
	},
}

var userMenuItems = []*MenuItem{
	{
		Title:    "Log out",
		Link:     "/logout/",
		IsActive: false,
	},
}

// Forms and JSON requests the app needs to handle

type SendCommandForm struct {
	//SessionID uuid.UUID  `json:"session_id" binding:"required"`
	ItemID uuid.UUID `json:"item_id" binding:"required"`
	// ItemType specifies on of the possible values: question, positive-answer, negative-answer.
	//ItemType  string `json:"item_type" binding:"required"`
}
