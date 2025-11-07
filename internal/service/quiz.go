package service

type QuizQuestion struct {
	ID       int
	Question string
	Options  []string
	Correct  int
}

type QuizSession struct {
	UserID          int64
	CurrentQuestion int
	Score           int
	Questions       []QuizQuestion
}
