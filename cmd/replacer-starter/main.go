package main

import (
	"github.com/no-src/replacerbot/cmd/replacer-starter/starter"
)

func main() {
	starter.InitFlags()
	starter.RunWithFlags()
}
