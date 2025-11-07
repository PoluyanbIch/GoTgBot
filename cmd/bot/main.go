package main

import (
	"log"
	"os"

	"github.com/PoluyanbIch/GoTgBot/internal/service"
	"github.com/PoluyanbIch/GoTgBot/internal/telegram"
)

func main() {
	token := os.Getenv("TELEGRAM_BOT_TOKEN")
	if token == "" {
		log.Fatal("TELEGRAM_BOT_TOKEN environment variable is required")
	}

	// Автоматически выбирает Gist или Memory
	leaderboardService := service.NewLeaderboardService()

	// Создаем бота
	bot, err := telegram.NewBot(token, leaderboardService, "questions.txt")
	if err != nil {
		log.Fatal(err)
	}

	log.Println("🤖 Bot is starting...")
	bot.Start()
}
