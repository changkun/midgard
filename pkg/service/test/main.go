package main

import (
	"fmt"
	"os"
	"time"

	"changkun.de/x/midgard/pkg/service"
)

var log service.Logger

func main() {
	var name = "test"
	var displayName = "test is test service"
	var desc = "test service tests description"
	var args = []string{"daemon"}

	var s, err = service.NewService(name, displayName, desc, args)
	log = s

	if err != nil {
		fmt.Printf("%s unable to start: %s", displayName, err)
		return
	}

	if len(os.Args) > 1 {
		var err error
		verb := os.Args[1]
		switch verb {
		case "install":
			err = s.Install()
			if err != nil {
				fmt.Printf("Failed to install: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" installed.\n", displayName)
		case "remove":
			err = s.Remove()
			if err != nil {
				fmt.Printf("Failed to remove: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" removed.\n", displayName)
		case "run", "daemon":
			log.Info("verb: ", verb)
			work()
		case "start":
			err = s.Start()
			if err != nil {
				fmt.Printf("Failed to start: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" started.\n", displayName)
		case "stop":
			err = s.Stop()
			if err != nil {
				fmt.Printf("Failed to stop: %s\n", err)
				return
			}
			fmt.Printf("Service \"%s\" stopped.\n", displayName)
		}
		return
	}
	err = s.Run(doWork, stopWork)
	if err != nil {
		s.Error(err.Error())
	}
}

var exit = make(chan struct{})

func doWork() error {
	// start
	go work()
	return nil

}
func work() {
	log.Info("Running!")
	ticker := time.NewTicker(time.Second * 10)
	for {
		select {
		case <-ticker.C:
			log.Info("Still running...")
		case <-exit:
			ticker.Stop()
			return
		}
	}
}
func stopWork() error {
	log.Info("Stopping!")
	exit <- struct{}{}
	return nil
}
