package main

import (
	"github.com/no-src/replacerbot/cmd/replacerbot/bot"
)

func main() {
	bot.InitFlags()
	bot.RunWithFlags()
}
