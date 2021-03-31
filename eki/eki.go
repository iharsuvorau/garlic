// Package eki (Eesti Keele Instituut) provides a wrapper around https://www.eki.ee/heli/syntproxy.php to give a way
// to synthesize Estonian speech given certain parameters.
//
// Available voices and their numeric codes:
//     "Eva (HTS)": 14,
//     "Eva (Ossian)": 18,
//     "Tõnu (HTS)": 15,
//     "Tõnu (Ossian)": 19,
//     "Tõnu (DNN)": 27,
//     "Liisi (HTS)": 16,
//     "Liisi (Ossian)": 20,
//     "Riina (HTS)": 17,
//     "Riina (Ossian)": 21,
//     "Riina (DNN)": 26,
//     "Meelis (HTS)": 24,
//     "Kihnu (HTS)": 25,
//     "Lee (HTS)": 29,
//     "Lee (Ossian)": 28,
//     "Liivika (HTS)": 30,
//     "Einar (Ossian)": 22,
//     "Luukas (Ossian)": 23
//
// Available emotions and their codes:
//     "neutral": 0,
//     "glad": 1,
//     "sad": 2,
//     "angry": 3
package eki

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

type Payload struct {
	Text    string
	Voice   uint8
	Emotion uint8
}

type Response struct {
	MP3 string `json:"mp3"`
	WAV string `json:"wav"`
}

func (p *Payload) Encode() string {
	data := url.Values{}
	data.Set("speech", p.Text)
	data.Set("v", fmt.Sprintf("%v", p.Voice))
	data.Set("e", fmt.Sprintf("%v", p.Emotion))
	return data.Encode()
}

func NewPayloadFrom(r io.Reader) (payload *Payload, err error) {
	payload = &Payload{}
	err = json.NewDecoder(r).Decode(&payload)
	return
}

func Send(p *Payload) (response *Response, err error) {
	dataString := p.Encode()
	req, err := http.NewRequest("POST", "https://www.eki.ee/heli/kiisu/syntproxy.php", strings.NewReader(dataString))
	if err != nil {
		return
	}
	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return
	}

	response = &Response{}
	err = json.NewDecoder(resp.Body).Decode(&response)
	if err != nil {
		return
	}

	// appending the host to response values

	u, err := url.Parse("https://www.eki.ee" + response.MP3)
	if err != nil {
		return
	}
	response.MP3 = u.String()

	u, err = url.Parse("https://www.eki.ee" + response.WAV)
	if err != nil {
		return
	}
	response.WAV = u.String()

	return
}
