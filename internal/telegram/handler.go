package telegram

import (
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/PoluyanbIch/GoTgBot/internal/service"
	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

type Bot struct {
	api                *tgbotapi.BotAPI
	quizSessions       map[int64]*service.QuizSession
	leaderboardService service.LeaderboardService
	quizQuestions      []service.QuizQuestion
}

func NewBot(token string, leaderboardService service.LeaderboardService, questionsFile string) (*Bot, error) {
	api, err := tgbotapi.NewBotAPI(token)
	if err != nil {
		return nil, err
	}

	questions := service.LoadQuizQuestions(questionsFile)

	return &Bot{
		api:                api,
		quizSessions:       make(map[int64]*service.QuizSession),
		leaderboardService: leaderboardService,
		quizQuestions:      questions,
	}, nil
}

func (b *Bot) Start() {
	b.api.Debug = true
	log.Printf("Authorised on account: %s", b.api.Self.UserName)

	u := tgbotapi.NewUpdate(0)
	u.Timeout = 60

	updates := b.api.GetUpdatesChan(u)

	for update := range updates {
		if update.Message != nil {
			switch update.Message.Command() {
			case "start":
				b.sendMainMenu(update.Message.Chat.ID)
			case "quiz":
				b.startQuiz(update.Message.Chat.ID)
			case "info":
				b.handleInfo(update.Message.Chat.ID)
			default:
				b.sendMessage(update.Message.Chat.ID, "Неизвестная команда")
			}
		}
		if update.CallbackQuery != nil {
			b.handleCallback(update.CallbackQuery)
		}
	}
}

func (b *Bot) handleCallback(callback *tgbotapi.CallbackQuery) {
	chatID := callback.Message.Chat.ID
	data := callback.Data
	user := callback.From

	callbackConfig := tgbotapi.NewCallback(callback.ID, "")
	if _, err := b.api.Request(callbackConfig); err != nil {
		log.Printf("Error Answering Callback: %v", err)
	}

	switch {
	case data == "start_quiz":
		b.startQuiz(chatID)
	case strings.HasPrefix(data, "quiz_"):
		b.handleQuizAnswer(chatID, data, user)
	case data == "exit_quiz":
		b.finishQuiz(chatID, true, user)
	case data == "back_to_menu":
		b.sendMainMenu(chatID)
	case data == "info":
		b.handleInfo(chatID)
	case data == "leaderboard":
		b.handleLeaderboard(chatID)
	default:
		b.sendMessage(chatID, "Неизвестная команда")
	}
}

func (b *Bot) sendMainMenu(chatID int64) {
	msg := tgbotapi.NewMessage(chatID, "📋 *Главное меню*")
	msg.ParseMode = "Markdown"

	kb := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🐖Харам тест🐖", "start_quiz"),
			tgbotapi.NewInlineKeyboardButtonData("🏆 Лидерборд", "leaderboard"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("ℹ️Обо мнеℹ️", "info"),
		),
	)
	msg.ReplyMarkup = kb
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending start message: %v", err)
	}
}

func (b *Bot) sendMessage(chatID int64, text string) {
	msg := tgbotapi.NewMessage(chatID, text)
	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sendinf msg: %v", err)
	}
}

func (b *Bot) startQuiz(chatID int64) {
	shuffledQuestions := service.ShuffleQuestions(b.quizQuestions)

	session := &service.QuizSession{
		UserID:          chatID,
		CurrentQuestion: 0,
		Score:           0,
		Questions:       shuffledQuestions,
	}

	b.quizSessions[chatID] = session
	b.sendQuestion(chatID, 0)
}

func (b *Bot) sendQuestion(chatID int64, questionIndex int) {
	session, exists := b.quizSessions[chatID]
	if !exists || questionIndex >= len(session.Questions) {
		return
	}
	question := session.Questions[questionIndex]

	message := fmt.Sprintf("❓ *Вопрос %d/%d*\n\n%s",
		questionIndex+1,
		len(session.Questions),
		question.Question)

	msg := tgbotapi.NewMessage(chatID, message)

	var rows [][]tgbotapi.InlineKeyboardButton
	for i, option := range question.Options {
		callbackData := fmt.Sprintf("quiz_%d_%d", questionIndex, i)
		button := tgbotapi.NewInlineKeyboardButtonData(option, callbackData)
		rows = append(rows, tgbotapi.NewInlineKeyboardRow(button))
	}

	rows = append(rows, tgbotapi.NewInlineKeyboardRow(
		tgbotapi.NewInlineKeyboardButtonData("🚪Выйти из викторины🚪", "exit_quiz"),
	))

	keyboard := tgbotapi.NewInlineKeyboardMarkup(rows...)
	msg.ReplyMarkup = keyboard

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending quesion: %v", err)
	}
}

func (b *Bot) handleQuizAnswer(chatID int64, data string, user *tgbotapi.User) {
	parts := strings.Split(data, "_")
	if len(parts) != 3 {
		return
	}
	questionIndex, _ := strconv.Atoi(parts[1])
	answerIndex, _ := strconv.Atoi(parts[2])

	session, exists := b.quizSessions[chatID]
	if !exists {
		return
	}
	question := session.Questions[questionIndex]
	isCorrect := answerIndex == question.Correct

	resultMsg := tgbotapi.NewMessage(chatID, "")
	if isCorrect {
		session.Score++
		resultMsg.Text = "✅ *Правильно!* 🎉"
	} else {
		correctAnswer := question.Options[question.Correct]
		resultMsg.Text = fmt.Sprintf("❌ *Неправильно!*\nПравильный ответ: %s", correctAnswer)
	}
	resultMsg.ParseMode = "Markdown"
	if _, err := b.api.Send(resultMsg); err != nil {
		log.Printf("Error sending result: %v", err)
	}

	// Переходим к следующему вопросу или завершаем
	session.CurrentQuestion++
	if session.CurrentQuestion < len(session.Questions) {
		// Ждем секунду и показываем следующий вопрос
		time.Sleep(1 * time.Second)
		b.sendQuestion(chatID, session.CurrentQuestion)
	} else {
		// Викторина завершена
		time.Sleep(1 * time.Second)
		b.finishQuiz(chatID, false, user)
	}
}

func (b *Bot) finishQuiz(chatID int64, exited bool, user *tgbotapi.User) {
	session, exists := b.quizSessions[chatID]
	if !exists {
		return
	}

	delete(b.quizSessions, chatID)

	finalMsg := tgbotapi.NewMessage(chatID, "")
	resultText := ""
	if exited {
		resultText = "🚪 Викторина прервана.\nВаш результат не сохранен."
	} else {
		percentage := (session.Score * 100) / len(session.Questions)

		isNewBest := b.leaderboardService.AddEntry(
			user.ID,
			user.UserName,
			user.FirstName,
			session.Score,
			len(session.Questions),
		)

		resultText = fmt.Sprintf(
			"🏁 *Викторина завершена!*\n\n"+
				"📊 Результат: %d/%d\n"+
				"📈 Процент правильных: %d%%\n\n",
			session.Score, len(session.Questions), percentage)

		if isNewBest {
			position, _ := b.leaderboardService.GetUserPosition(user.ID)
			if position != -1 {
				resultText += fmt.Sprintf("🎉 *Новый рекорд!* Вы на %d месте в лидерборде!\n\n", position)
			}
		}
	}
	finalMsg.ParseMode = "Markdown"
	finalMsg.Text = resultText
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Начать заново", "start_quiz"),
			tgbotapi.NewInlineKeyboardButtonData("🔙 В меню", "back_to_menu"),
		),
	)

	finalMsg.ReplyMarkup = keyboard

	if _, err := b.api.Send(finalMsg); err != nil {
		log.Printf("Error sending final message: %v", err)
	}
}

func (b *Bot) handleLeaderboard(chatID int64) {
	top := b.leaderboardService.GetTop(10) // Топ 10

	if len(top) == 0 {
		b.sendMessage(chatID, "🏆 *Лидерборд*\n\nПока нет результатов. Будьте первым! 🎯")
		return
	}

	message := "🏆 <b>Топ 10 игроков<b>\n\n"

	for i, entry := range top {
		username := entry.FirstName
		if entry.Username != "" {
			username = "@" + entry.Username
		}

		medal := "🔸"
		switch i {
		case 0:
			medal = "🥇"
		case 1:
			medal = "🥈"
		case 2:
			medal = "🥉"
		}

		message += fmt.Sprintf("%s %d. %s - %d%% (%d/%d)\n   📅 %s\n\n",
			medal, i+1, username, entry.Percentage, entry.Score, entry.Total, entry.Date)
	}

	msg := tgbotapi.NewMessage(chatID, message)
	msg.ParseMode = "HTML"

	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🎯 Начать викторину", "start_quiz"),
			tgbotapi.NewInlineKeyboardButtonData("📋 Главное меню", "back_to_menu"),
		),
	)

	msg.ReplyMarkup = keyboard

	if _, err := b.api.Send(msg); err != nil {
		log.Printf("Error sending leaderboard: %v", err)
	}
}

func (b *Bot) handleInfo(chatID int64) {
	msg := "Мой исходный код:\n" +
		"https://github.com/PoluyanbIch/GoTgBot\n" +
		"Можно поставить звездочку⭐ на него и подписаться:\n" +
		"https://github.com/PoluyanbIch\n" +
		"отзывы, предложения, предпочтения -> https://t.me/PoluyanbIch"

	infoMsg := tgbotapi.NewMessage(chatID, msg)
	infoMsg.ParseMode = "Markdown"

	// Добавляем кнопки для удобства
	keyboard := tgbotapi.NewInlineKeyboardMarkup(
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("📂 GitHub репозиторий", "https://github.com/PoluyanbIch/GoTgBot"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonURL("👤 Автор", "https://github.com/PoluyanbIch"),
			tgbotapi.NewInlineKeyboardButtonURL("💬 Написать", "https://t.me/PoluyanbIch"),
		),
		tgbotapi.NewInlineKeyboardRow(
			tgbotapi.NewInlineKeyboardButtonData("🔙 Назад", "back_to_menu"),
		),
	)

	infoMsg.ReplyMarkup = keyboard

	if _, err := b.api.Send(infoMsg); err != nil {
		log.Printf("Error sending info: %v", err)
	}
}
