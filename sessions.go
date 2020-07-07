package main

type Session struct {
	ID    int64
	Name  string
	Items []SessionItem
}

type SessionItem struct {
	ID             int64
	Question       SayWithMotionItem
	PositiveAnswer SayWithMotionItem
	NegativeAnswer SayWithMotionItem
}

type SayWithMotionItem struct {
	Phrase        string
	AudioFilePath string
	MotionName    string
}

var sessions = []Session{
	{
		ID:   1,
		Name: "Session 1",
		Items: []SessionItem{
			{
				ID: 1,
				Question: SayWithMotionItem{
					Phrase:        "Tere, mina olen robot Pepper. Mina olen 6-aastane ja tahan sinuga tuttavaks saada. Mis on sinu nimi?",
					AudioFilePath: "1out_tutvustus.wav",
					MotionName:    "hello_a010.qianim",
				},
				PositiveAnswer: SayWithMotionItem{
					Phrase:        "OK",
					AudioFilePath: "ok.wav",
					MotionName:    "ok.qianim",
				},
				NegativeAnswer: SayWithMotionItem{
					Phrase:        "Not OK",
					AudioFilePath: "notok.wav",
					MotionName:    "notok.qianim",
				},
			},
			{
				ID: 2,
				Question: SayWithMotionItem{
					Phrase:        "Kui vana sa oled?",
					AudioFilePath: "2out_vanus.wav",
					MotionName:    "hello_a010.qianim",
				},
				PositiveAnswer: SayWithMotionItem{
					Phrase:        "OK",
					AudioFilePath: "ok.wav",
					MotionName:    "ok.qianim",
				},
				NegativeAnswer: SayWithMotionItem{
					Phrase:        "Not OK",
					AudioFilePath: "notok.wav",
					MotionName:    "notok.qianim",
				},
			},
		},
	},
	{
		ID:   2,
		Name: "Session 2",
		Items: []SessionItem{
			{
				ID: 1,
				Question: SayWithMotionItem{
					Phrase: "Q1",
				},
			},
			{
				ID: 2,
				Question: SayWithMotionItem{
					Phrase: "Q2",
				},
			},
		},
	},
}
