package main

import (
	"flag"
	"log"
	"time"

	"github.com/jroimartin/gocui"
	"github.com/serialx/goclair"
)

func main() {
	var bucket, key string
	var timeout time.Duration

	flag.StringVar(&bucket, "b", "", "Bucket name.")
	flag.StringVar(&key, "k", "", "Object key name.")
	flag.DurationVar(&timeout, "d", 0, "Upload timeout.")
	flag.Parse()

	ctrl := goclair.NewInstanceController()
	go func() {
		ctrl.SetInstances(goclair.GetInstances())
	}()

	g, err := gocui.NewGui(gocui.OutputNormal)
	if err != nil {
		log.Panicln(err)
	}
	defer g.Close()

	if err := ctrl.InitializeGui(g); err != nil {
		log.Panicln(err)
	}

	if err := g.MainLoop(); err != nil && err != gocui.ErrQuit {
		log.Panicln(err)
	}
}
