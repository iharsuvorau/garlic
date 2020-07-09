package main

import "time"

type Session struct {
	ID          int64
	Name        string
	Description string
	Items       []SessionItem
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
	MotionDelay   time.Duration
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
					MotionDelay:   0,
				},
				PositiveAnswer: SayWithMotionItem{
					Phrase:        "OK",
					AudioFilePath: "ok.wav",
					MotionName:    "ok.qianim",
					MotionDelay:   0,
				},
				NegativeAnswer: SayWithMotionItem{
					Phrase:        "Not OK",
					AudioFilePath: "notok.wav",
					MotionName:    "notok.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: 2,
				Question: SayWithMotionItem{
					Phrase:        "Kui vana sa oled?",
					AudioFilePath: "2out_vanus.wav",
					MotionName:    "question_right_hand_a001.qianim",
					MotionDelay:   0,
				},
				PositiveAnswer: SayWithMotionItem{
					Phrase:        "OK",
					AudioFilePath: "ok.wav",
					MotionName:    "ok.qianim",
					MotionDelay:   0,
				},
				NegativeAnswer: SayWithMotionItem{
					Phrase:        "Not OK",
					AudioFilePath: "notok.wav",
					MotionName:    "notok.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: 3,
				Question: SayWithMotionItem{
					Phrase:        "Kas Sul on vendi või õdesid?",
					AudioFilePath: "3out_vennad.wav",
					MotionName:    "question_both_hands_a007.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: 4,
				Question: SayWithMotionItem{
					Phrase:        "Ma tulin siia üksi, kuid mu pere on suur ja mööda maailma laiali.",
					AudioFilePath: "3out_vennadVV.wav",
					MotionName:    "both_hands_high_b001.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: 5,
				Question: SayWithMotionItem{
					Phrase:        "Mina olen pärit Pariisist ja nüüd meeldib mulle väga Eestis elada. Mis sulle Sinu Eestimaa juures meeldib?",
					AudioFilePath: "4out_päritolu.wav",
					MotionName:    "exclamation_both_hands_a001.qianim",
					MotionDelay:   time.Second * 5,
				},
			},
			{
				ID: 6,
				Question: SayWithMotionItem{
					Phrase:        "Jaa, see on väike ja sõbralik maa ja teil on 4 aastaaega",
					AudioFilePath: "5out_eestimaavastus.wav",
					MotionName:    "affirmation_a009",
					MotionDelay:   0,
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
