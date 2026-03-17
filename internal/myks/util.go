package myks

import (
	"fmt"
	"iter"
	"os"

	"github.com/alecthomas/chroma/v2/quick"
	aurora "github.com/logrusorgru/aurora/v4"
	"github.com/rs/zerolog/log"
	"golang.org/x/sync/errgroup"
	"golang.org/x/term"
)

func printFileNicely(name, content, syntax string) {
	if !term.IsTerminal(int(os.Stdout.Fd())) { // #nosec G115 -- file descriptor fits in int
		fmt.Println(content)
		return
	}

	fmt.Println(aurora.Bold(fmt.Sprintf("=== %s ===\n", name)))
	err := quick.Highlight(os.Stdout, content, syntax, "terminal16m", "doom-one2")
	if err != nil {
		log.Error().Err(err).Msg("Failed to highlight")
	} else {
		fmt.Printf("\n\n")
	}
}

func process[Item any](asyncLevel int, collection iter.Seq[Item], fn func(Item) error) error {
	var eg errgroup.Group
	if asyncLevel == 0 {
		// no limit
		asyncLevel = -1
	}
	eg.SetLimit(asyncLevel)

	for item := range collection {
		// Create a new variable to avoid capturing the same item in the closure
		innerItem := item
		eg.Go(func() error {
			return fn(innerItem)
		})
	}

	return eg.Wait()
}

func msgWithSteps(step1, step2, msg string) string {
	return fmt.Sprintf(GlobalExtendedLogFormat, step1, step2, msg)
}
