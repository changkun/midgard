package ping

import (
	"context"
	"log"

	"github.com/spf13/cobra"
	"golang.design/x/midgard/pkg/types/proto"
	"golang.design/x/midgard/pkg/utils"
)

// Cmd ...
var Cmd = &cobra.Command{
	Use:   "ping",
	Short: "ping midgard server",
	Long:  `ping midgard server`,
	Args:  cobra.ExactArgs(0),
	Run:   run,
}

func run(_ *cobra.Command, args []string) {
	// ping
	utils.Connect(func(ctx context.Context, c proto.MidgardClient) {
		out, err := c.Ping(ctx, &proto.PingInput{})
		if err != nil {
			log.Fatalf("midgard server is not responding, err: %v", err)
		}

		log.Println(out.Message)
	})
}
