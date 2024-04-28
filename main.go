package main

import (
	"github.com/StupidTAO/crawler/cmd"
	_ "net/http/pprof"
)

func main() {
	cmd.Execute()
}
