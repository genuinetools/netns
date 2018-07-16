package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"
)

const listHelp = `List networks.`

func (cmd *listCommand) Name() string      { return "ls" }
func (cmd *listCommand) Args() string      { return "" }
func (cmd *listCommand) ShortHelp() string { return listHelp }
func (cmd *listCommand) LongHelp() string  { return listHelp }
func (cmd *listCommand) Hidden() bool      { return false }

func (cmd *listCommand) Register(fs *flag.FlagSet) {}

type listCommand struct{}

func (cmd *listCommand) Run(ctx context.Context, args []string) error {
	networks, err := client.List()
	if err != nil {
		return err
	}

	// Print the networks.
	w := tabwriter.NewWriter(os.Stdout, 20, 1, 3, ' ', 0)
	fmt.Fprint(w, "IP\tLOCAL VETH\tPID\tSTATUS\tNS FD\n")
	for _, n := range networks {
		fmt.Fprintf(w, "%s\t%s\t%d\t%s\t%d\n", n.IP.String(), n.VethPair.Attrs().Name, n.PID, n.Status, n.FD)
	}
	w.Flush()

	return nil
}
