package main

import (
	"github.com/google/uuid"
	"os"
	"path"
	"regexp"
	"time"
)

type Session struct {
	ID          uuid.UUID
	Name        string
	Description string
	Items       []*SessionItem
}

type SessionItem struct {
	ID             uuid.UUID
	Question       *SayWithMotionItem
	PositiveAnswer *SayWithMotionItem
	NegativeAnswer *SayWithMotionItem
}

type SayWithMotionItem struct {
	ID            uuid.UUID
	Phrase        string
	AudioFilePath string
	MotionName    string
	MotionDelay   time.Duration
}

func (item *SayWithMotionItem) IsValid() bool {
	if item.Phrase == "" || !IsValidUUID(item.ID.String()) || item.AudioFilePath == "" {
		return false
	}
	return true
}

func IsValidUUID(uuid string) bool {
	r := regexp.MustCompile("^[a-fA-F0-9]{8}-[a-fA-F0-9]{4}-4[a-fA-F0-9]{3}-[8|9|aA|bB][a-fA-F0-9]{3}-[a-fA-F0-9]{12}$")
	return r.MatchString(uuid)
}

type Sessions []*Session

func (ss Sessions) GetSayWithMotionItemByID(id uuid.UUID) *SayWithMotionItem {
	for _, session := range ss {
		for _, item := range session.Items {

			if item.NegativeAnswer != nil && item.NegativeAnswer.ID == id {
				return item.NegativeAnswer
			}

			if item.PositiveAnswer != nil && item.PositiveAnswer.ID == id {
				return item.PositiveAnswer
			}

			if item.Question != nil && item.Question.ID == id {
				return item.Question
			}
		}
	}
	return nil
}

var sessions = []*Session{
	{
		ID:   uuid.Must(uuid.NewRandom()),
		Name: "Session 1",
		Items: []*SessionItem{
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Tere, mina olen robot Pepper. Mina olen 6-aastane ja tahan sinuga tuttavaks saada. Mis on sinu nimi?",
					AudioFilePath: "1out_tutvustus.wav",
					MotionName:    "hello_a010.qianim",
					MotionDelay:   0,
				},
				PositiveAnswer: &SayWithMotionItem{
					ID:          uuid.Must(uuid.NewRandom()),
					Phrase:      "OK",
					MotionDelay: 0,
				},
				NegativeAnswer: &SayWithMotionItem{
					ID:          uuid.Must(uuid.NewRandom()),
					Phrase:      "Not OK",
					MotionDelay: 0,
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Kui vana sa oled?",
					AudioFilePath: "2out_vanus.wav",
					MotionName:    "question_right_hand_a001.qianim",
					MotionDelay:   0,
				},
				PositiveAnswer: &SayWithMotionItem{
					ID:          uuid.Must(uuid.NewRandom()),
					Phrase:      "OK",
					MotionDelay: 0,
				},
				NegativeAnswer: &SayWithMotionItem{
					ID:          uuid.Must(uuid.NewRandom()),
					Phrase:      "Not OK",
					MotionDelay: 0,
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Kas Sul on vendi või õdesid?",
					AudioFilePath: "3out_vennad.wav",
					MotionName:    "question_both_hands_a007.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Ma tulin siia üksi, kuid mu pere on suur ja mööda maailma laiali.",
					AudioFilePath: "3out_vennadVV.wav",
					MotionName:    "both_hands_high_b001.qianim",
					MotionDelay:   0,
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Mina olen pärit Pariisist ja nüüd meeldib mulle väga Eestis elada. Mis sulle Sinu Eestimaa juures meeldib?",
					AudioFilePath: "4out_päritolu.wav",
					MotionName:    "exclamation_both_hands_a001.qianim",
					MotionDelay:   time.Second * 5,
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:            uuid.Must(uuid.NewRandom()),
					Phrase:        "Jaa, see on väike ja sõbralik maa ja teil on 4 aastaaega",
					AudioFilePath: "5out_eestimaavastus.wav",
					MotionName:    "affirmation_a009",
					MotionDelay:   0,
				},
			},
		},
	},
	{
		ID:   uuid.Must(uuid.NewRandom()),
		Name: "Session 2",
		Items: []*SessionItem{
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:     uuid.Must(uuid.NewRandom()),
					Phrase: "Q1",
				},
			},
			{
				ID: uuid.Must(uuid.NewRandom()),
				Question: &SayWithMotionItem{
					ID:     uuid.Must(uuid.NewRandom()),
					Phrase: "Q2",
				},
			},
		},
	},
}

func initSessions(sessions []*Session, dataDir string) error {
	for _, s := range sessions {
		for _, item := range s.Items {
			if item.Question != nil && item.Question.AudioFilePath != "" {
				item.Question.AudioFilePath = path.Join(dataDir, s.Name, item.Question.AudioFilePath)
				if _, err := os.Stat(item.Question.AudioFilePath); os.IsNotExist(err) {
					return err
				}
			}
			if item.PositiveAnswer != nil && item.PositiveAnswer.AudioFilePath != "" {
				item.PositiveAnswer.AudioFilePath = path.Join(dataDir, s.Name, item.PositiveAnswer.AudioFilePath)
				if _, err := os.Stat(item.PositiveAnswer.AudioFilePath); os.IsNotExist(err) {
					return err
				}
			}
			if item.NegativeAnswer != nil && item.NegativeAnswer.AudioFilePath != "" {
				item.NegativeAnswer.AudioFilePath = path.Join(dataDir, s.Name, item.NegativeAnswer.AudioFilePath)
				if _, err := os.Stat(item.NegativeAnswer.AudioFilePath); os.IsNotExist(err) {
					return err
				}
			}
		}
	}
	return nil
}
