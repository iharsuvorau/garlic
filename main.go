package main

import (
	"encoding/json"
	"flag"
	"github.com/gorilla/websocket"
	"log"
	"net/http"
	"time"
)

var upgrader = websocket.Upgrader{}

var addr = flag.String("addr", ":8080", "http service address")

func main() {
	flag.Parse()
	upgrader.CheckOrigin = func(r *http.Request) bool { return true }
	http.HandleFunc("/", homeHandler)
	http.HandleFunc("/test", testWSHandler)
	log.Println("listening at", *addr)
	//log.Fatal(http.ListenAndServeTLS(*addr, "server.crt", "server.key", nil))
	log.Fatal(http.ListenAndServe(*addr, nil))
}

func homeHandler(w http.ResponseWriter, r *http.Request) {
	w.Write([]byte("hi there"))
}

type Message struct {
	Command string
	Content string
}

func testWSHandler(w http.ResponseWriter, r *http.Request) {
	log.Println("new request")
	log.Printf("%s, %s\n", r.RemoteAddr, r.Proto)

	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Println(err)
		return
	}

	sayHiMsg := Message{
		Command: "say",
		Content: "How're you doing?",
	}
	sayHiMsgBytes, err := json.Marshal(sayHiMsg)
	if err != nil {
		log.Println(err)
		return
	}

	for {
		//log.Println("in for loop")

		//_, _, err := conn.ReadMessage()
		//if err != nil {
		//	log.Println(err)
		//	return
		//}
		//log.Println("msg read")
		//log.Printf("received: %s\n", p)

		//log.Println("msg sending")
		if err = conn.WriteMessage(websocket.TextMessage, sayHiMsgBytes); err != nil {
			//if err = conn.WriteMessage(websocket.TextMessage, []byte("hoho")); err != nil {
			log.Println(err)
			return
		}
		log.Println("msg sent")

		time.Sleep(time.Second * 5)
	}
}
