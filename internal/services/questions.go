package services

import (
	"math/rand"
	"time"
	"buildprize-game/internal/models"
)

type QuestionDatabase struct {
	questions []models.Question
}

func NewQuestionDatabase() *QuestionDatabase {
	return &QuestionDatabase{
		questions: []models.Question{
			{
				ID:       "1",
				Text:     "What is the capital of France?",
				Options:  []string{"London", "Berlin", "Paris", "Madrid"},
				Correct:  2,
				Category: "Geography",
			},
			{
				ID:       "2",
				Text:     "Which planet is known as the Red Planet?",
				Options:  []string{"Venus", "Mars", "Jupiter", "Saturn"},
				Correct:  1,
				Category: "Science",
			},
			{
				ID:       "3",
				Text:     "What is 2 + 2?",
				Options:  []string{"3", "4", "5", "6"},
				Correct:  1,
				Category: "Math",
			},
			{
				ID:       "4",
				Text:     "Who painted the Mona Lisa?",
				Options:  []string{"Van Gogh", "Picasso", "Da Vinci", "Monet"},
				Correct:  2,
				Category: "Art",
			},
			{
				ID:       "5",
				Text:     "What is the largest ocean on Earth?",
				Options:  []string{"Atlantic", "Indian", "Pacific", "Arctic"},
				Correct:  2,
				Category: "Geography",
			},
			{
				ID:       "6",
				Text:     "Which programming language was created by Google?",
				Options:  []string{"Java", "Python", "Go", "C++"},
				Correct:  2,
				Category: "Technology",
			},
			{
				ID:       "7",
				Text:     "What is the chemical symbol for gold?",
				Options:  []string{"Go", "Gd", "Au", "Ag"},
				Correct:  2,
				Category: "Science",
			},
			{
				ID:       "8",
				Text:     "In which year did World War II end?",
				Options:  []string{"1944", "1945", "1946", "1947"},
				Correct:  1,
				Category: "History",
			},
			{
				ID:       "9",
				Text:     "What is the fastest land animal?",
				Options:  []string{"Lion", "Cheetah", "Leopard", "Tiger"},
				Correct:  1,
				Category: "Nature",
			},
			{
				ID:       "10",
				Text:     "Which country has the most natural lakes?",
				Options:  []string{"Russia", "Canada", "USA", "Finland"},
				Correct:  1,
				Category: "Geography",
			},
		},
	}
}

func (qd *QuestionDatabase) GetRandomQuestion() *models.Question {
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(qd.questions))
	return &qd.questions[index]
}

func (qd *QuestionDatabase) GetQuestionByCategory(category string) *models.Question {
	var categoryQuestions []models.Question
	for _, q := range qd.questions {
		if q.Category == category {
			categoryQuestions = append(categoryQuestions, q)
		}
	}
	
	if len(categoryQuestions) == 0 {
		return qd.GetRandomQuestion()
	}
	
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(categoryQuestions))
	return &categoryQuestions[index]
}
