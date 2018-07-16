package main

import (
	"context"
	"flag"
	"fmt"

	"github.com/genuinetools/netns/bridge"
)

const removeHelp = `Delete a network.`

func (cmd *removeCommand) Name() string      { return "rm" }
func (cmd *removeCommand) Args() string      { return "[OPTIONS]" }
func (cmd *removeCommand) ShortHelp() string { return removeHelp }
func (cmd *removeCommand) LongHelp() string  { return removeHelp }
func (cmd *removeCommand) Hidden() bool      { return false }

func (cmd *removeCommand) Register(fs *flag.FlagSet) {}

type removeCommand struct{}

func (cmd *removeCommand) Run(ctx context.Context, args []string) error {
	if err := bridge.Delete(brOpt.Name); err != nil {
		return err
	}
	fmt.Printf("deleted bridge: %s\n", brOpt.Name)
	return nil
}
