package main

import (
	"flag"
	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
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

	r := gin.Default()
	r.LoadHTMLGlob("templates/*.html")

	// JSON: robot API
	r.GET("/pepper/initiate", initiateHandler)

	// HTML: user GUI
	r.POST("/voice", voiceHandler)
	r.GET("/", homeHandler)

	log.Fatal(r.Run(*addr))
}

func initiateHandler(c *gin.Context) {
	var err error
	wsConnection, err = upgrader.Upgrade(c.Writer, c.Request, nil)
	if err != nil {
		return
	}
}

type VoiceForm struct {
	Phrase string `form:"phrase" binding:"required"`
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

func homeHandler(c *gin.Context) {
	c.HTML(http.StatusOK, "index.html", gin.H{"title": "Garlic Home Page"})
}
