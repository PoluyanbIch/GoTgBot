package service

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
	"unicode/utf8"
)

// ParseQuizQuestions парсит вопросы из TXT файла
func ParseQuizQuestions(filename string) ([]QuizQuestion, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %v", err)
	}
	defer file.Close()

	var questions []QuizQuestion
	scanner := bufio.NewScanner(file)
	questionID := 1

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			continue // Пропускаем пустые строки
		}

		// Парсим строку: "вопрос" <цифра>
		question, correct, err := parseQuestionLine(line)
		if err != nil {
			return nil, fmt.Errorf("error parsing line '%s': %v", line, err)
		}

		questions = append(questions, QuizQuestion{
			ID:       questionID,
			Question: question,
			Options:  []string{"👍Халяль", "🐖Харам"},
			Correct:  correct,
		})
		questionID++
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}

	if len(questions) == 0 {
		return nil, fmt.Errorf("no valid questions found in file")
	}

	return questions, nil
}

// parseQuestionLine парсит одну строку с вопросом
func parseQuestionLine(line string) (string, int, error) {
	// Ищем закрывающую кавычку
	quoteEnd := strings.Index(line[1:], `"`) + 1
	if quoteEnd <= 0 {
		return "", 0, fmt.Errorf("invalid format: no closing quote")
	}

	// Извлекаем вопрос (без кавычек)
	question := line[1:quoteEnd]

	// Остаток строки после кавычки
	remaining := strings.TrimSpace(line[quoteEnd+1:])

	// Парсим цифру (0 или 1)
	if len(remaining) == 0 {
		return "", 0, fmt.Errorf("no correctness indicator found")
	}

	correct, err := strconv.Atoi(string(remaining[0]))
	if err != nil {
		return "", 0, fmt.Errorf("invalid correctness indicator: %v", err)
	}

	if correct != 0 && correct != 1 {
		return "", 0, fmt.Errorf("correctness must be 0 or 1, got %d", correct)
	}

	// Валидация вопроса
	if utf8.RuneCountInString(question) == 0 {
		return "", 0, fmt.Errorf("question cannot be empty")
	}

	return question, correct, nil
}

// LoadQuizQuestions загружает вопросы из файла или возвращает дефолтные при ошибке
func LoadQuizQuestions(filename string) []QuizQuestion {
	questions, err := ParseQuizQuestions(filename)
	if err != nil {
		fmt.Printf("Warning: Failed to load questions from %s: %v\n", filename, err)
		fmt.Println("Using default questions...")
		return DefaultQuizQuestions()
	}

	fmt.Printf("Successfully loaded %d questions from %s\n", len(questions), filename)
	return questions
}

// DefaultQuizQuestions возвращает вопросы по умолчанию
func DefaultQuizQuestions() []QuizQuestion {
	return []QuizQuestion{
		{
			ID:       1,
			Question: "Свинина",
			Options:  []string{"👍Халяль", "🐖Харам"},
			Correct:  1,
		},
		{
			ID:       2,
			Question: "Курица",
			Options:  []string{"👍Халяль", "🐖Харам"},
			Correct:  0,
		},
	}
}
