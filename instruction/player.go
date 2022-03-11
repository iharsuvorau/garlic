package instruction

import (
	"log"
	"os"
	s "strings"
	"time"

	"github.com/faiface/beep"
	"github.com/faiface/beep/mp3"
	"github.com/faiface/beep/speaker"
	"github.com/faiface/beep/wav"
)

func soundPlayer(inp string) {
	log.Printf("From func: %v", inp)

	f, err := os.Open(inp)
	if err != nil {
		log.Println("err!=nil")

		// log.Fatal(err)
	}
	log.Println(err)
	log.Println("Checked")

	// var ss string
	// fmt.Scanln(&ss)
	var streamer beep.StreamSeekCloser
	var format beep.Format

	if s.HasSuffix(inp, "wav") {
		streamer, format, err = wav.Decode(f)
	} else if s.HasSuffix(inp, "mp3") {
		streamer, format, err = mp3.Decode(f)
	} else {
		return
	}

	if err != nil {
		log.Println(err)

		// log.Fatal(err)
	}
	defer streamer.Close()

	speaker.Init(format.SampleRate, format.SampleRate.N(time.Second/10))

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))

	<-done
}
