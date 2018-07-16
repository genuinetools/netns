package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/genuinetools/netns/bridge"
)

const createHelp = `Create a network.`

func (cmd *createCommand) Name() string      { return "create" }
func (cmd *createCommand) Args() string      { return "[OPTIONS]" }
func (cmd *createCommand) ShortHelp() string { return createHelp }
func (cmd *createCommand) LongHelp() string  { return createHelp }
func (cmd *createCommand) Hidden() bool      { return false }

func (cmd *createCommand) Register(fs *flag.FlagSet) {}

type createCommand struct{}

func (cmd *createCommand) Run(ctx context.Context, args []string) error {
	i, err := bridge.Init(brOpt)
	if err != nil {
		return err
	}
	fmt.Printf("created bridge: %#v\n", i)
	return nil
}
