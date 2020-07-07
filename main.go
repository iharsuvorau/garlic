package main

import (
	"flag"
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"strconv"
)

// TODO: communicate over WSS
// TODO: is there a need to keep WS connection live using ping-pong?

var upgrader = websocket.Upgrader{}
var wsConnection *websocket.Conn

// CLI arguments
var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()

	upgrader.CheckOrigin = func(r *http.Request) bool { return true }

	r := gin.New()
	r.LoadHTMLGlob("templates/*.html")

	// JSON: robot API
	r.GET("/pepper/initiate", initiateHandler)
	r.POST("/pepper/send_command", sendCommandHandler)

	// HTML: user GUI
	r.Static("/assets/", "assets")
	r.POST("/voice", voiceHandler)
	r.GET("/sessions/:id", sessionsHandler)
	r.GET("/sessions/", sessionsHandler)
	r.GET("/manual/", pageHandler("Manual"))
	r.GET("/about/", pageHandler("About"))
	r.GET("/", homeHandler)

	log.Fatal(r.Run(*addr))
}

// Handlers

func sendCommandHandler(c *gin.Context) {
	var form SendCommandForm
	err := c.BindJSON(&form)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	log.Printf("form: %+v", form)

	var sayPhrase, audioFileName, motion string
	for _, session := range sessions {
		if session.ID == form.SessionID {
			for _, sessionItem := range session.Items {
				if sessionItem.ID == form.ItemID {
					switch form.ItemType {
					case "question":
						sayPhrase = sessionItem.Question.Phrase
						audioFileName = sessionItem.Question.AudioFilePath
						motion = sessionItem.Question.MotionName
					case "positiveAnswer":
						sayPhrase = sessionItem.PositiveAnswer.Phrase
						audioFileName = sessionItem.PositiveAnswer.AudioFilePath
						motion = sessionItem.PositiveAnswer.MotionName
					case "negativeAnswer":
						sayPhrase = sessionItem.NegativeAnswer.Phrase
						audioFileName = sessionItem.NegativeAnswer.AudioFilePath
						motion = sessionItem.NegativeAnswer.MotionName
					default:
						c.JSON(http.StatusBadRequest, gin.H{"error": "unrecognized item type"})
						return
					}
					break
				}
			}
		}
	}
	log.Printf("command: say, phrase: %s, audio: %s, motion: %s",
		sayPhrase, audioFileName, motion)

	// TODO: process the command

	c.JSON(http.StatusOK, gin.H{"message": "the command has been sent"})
}

func homeHandler(c *gin.Context) {
	c.Redirect(http.StatusTemporaryRedirect, "/sessions/")
}

func initiateHandler(c *gin.Context) {
	var err error
	wsConnection, err = upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
}

func voiceHandler(c *gin.Context) {
	var form VoiceForm
	if err := c.Bind(&form); err != nil {
		c.String(http.StatusBadRequest, "Wrong user input")
		return
	}

	task := PepperTask{
		Command: "say",
		Content: form.Phrase,
	}
	if err := task.Send(); err != nil {
		c.String(http.StatusInternalServerError, "Task has failed: %s", err.Error())
		return
	}

	c.String(http.StatusOK, "Task has been sent")
}

func sessionsHandler(c *gin.Context) {
	activeMenu("Sessions", siteMenuItems)

	var curSessionID int64
	var curSessionName string

	curSessionID, _ = strconv.ParseInt(c.Param("id"), 10, 64)

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
		"siteMenu":           siteMenuItems,
		"userMenu":           userMenuItems,
	})
}

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

// Menu

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
	{
		Title:    "Manual",
		Link:     "/manual/",
		IsActive: false,
	},
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

// Forms

type SendCommandForm struct {
	SessionID int64  `json:"session_id" binding:"required"`
	ItemID    int64  `json:"item_id" binding:"required"`
	ItemType  string `json:"item_type" binding:"required"`
}

type VoiceForm struct {
	Phrase string `form:"phrase" binding:"required"`
}
