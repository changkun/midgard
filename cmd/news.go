package cmd

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/pkg/types/proto"
	"golang.design/x/midgard/pkg/utils"
)

// newsCmd creates a new posts
var newsCmd = &cobra.Command{
	Use:   "news",
	Short: "news creates a new posts that can be seen in /midgard/news",
	Long:  `news creates a new posts that can be seen in /midgard/news`,
	Args:  cobra.ExactArgs(1),
	Run: func(_ *cobra.Command, args []string) {
		n := &news{title: args[0]}
		ok := n.waitInputs()
		if !ok {
			return
		}
		fmt.Println("")
		utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
			out, err := c.CreateNews(ctx, &proto.CreateNewsInput{
				Date:  time.Now().Format("2006-01-02-15:04"),
				Title: n.title,
				Body:  strings.Join(n.body, ""),
			})
			if err != nil {
				log.Fatalf("cannot interact with midgard daemon, err:\n%v", err)
			}
			fmt.Println(out.Message)
		})

	},
}

// news is what you want to share to the public
type news struct {
	date  string
	title string
	body  []string
}

func (n *news) waitInputs() bool {
	fmt.Println("(Ctrl+D to complete; Ctrl+C to cancel)")
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt)

	line := make(chan string, 1)
	go func() {
		s := bufio.NewReader(os.Stdin)
		for {
			fmt.Print("> ")
			l, err := s.ReadString('\n')
			if err != nil {
				if err == io.EOF {
					sigCh <- os.Kill
					return
				}
			}
			line <- l
		}
	}()

	for {
		select {
		case sig := <-sigCh:
			if sig != os.Kill {
				return false
			}
			return true
		case l := <-line:
			if len(l) == 0 {
				return true
			}
			n.body = append(n.body, l)
		}
	}
}
