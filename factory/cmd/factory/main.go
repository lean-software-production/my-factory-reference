// Command factory turns a seed into working software by driving a coding agent.
package main

import (
	"flag"
	"fmt"
	"os"
)

// version is set by the release build with -ldflags "-X main.version=<v>".
var version = "dev"

func main() {
	flags := flag.NewFlagSet("factory", flag.ContinueOnError)
	flags.String("seed", "", "the seed file that says what to build")
	flags.String("target", "", "the folder to build it in")
	if err := flags.Parse(os.Args[1:]); err != nil {
		os.Exit(2)
	}
	fmt.Fprintln(os.Stderr, "factory: not built yet")
	os.Exit(1)
}
