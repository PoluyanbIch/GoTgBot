package service

import (
	"math/rand"
	"time"
)

// ShuffleQuestions перемешивает вопросы в случайном порядке
func ShuffleQuestions(questions []QuizQuestion) []QuizQuestion {
	// Создаем копию массива, чтобы не изменять оригинал
	shuffled := make([]QuizQuestion, len(questions))
	copy(shuffled, questions)

	// Инициализируем генератор случайных чисел
	r := rand.New(rand.NewSource(time.Now().UnixNano()))

	// Перемешиваем вопросы используя алгоритм Фишера-Йейтса
	for i := len(shuffled) - 1; i > 0; i-- {
		j := r.Intn(i + 1)
		shuffled[i], shuffled[j] = shuffled[j], shuffled[i]
	}

	return shuffled
}

// ShuffleQuestionsWithLimit перемешивает вопросы и возвращает только limit штук
func ShuffleQuestionsWithLimit(questions []QuizQuestion, limit int) []QuizQuestion {
	shuffled := ShuffleQuestions(questions)

	if limit <= 0 || limit > len(shuffled) {
		limit = len(shuffled)
	}

	return shuffled[:limit]
}
