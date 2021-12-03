package cmd

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/stripe/stripe-cli/pkg/validators"
)

type feedbackCmd struct {
	cmd *cobra.Command
}

func newFeedbackdCmd() *feedbackCmd {
	return &feedbackCmd{
		cmd: &cobra.Command{
			Use:   "feedback",
			Args:  validators.NoArgs,
			Short: "Provide us with feedback on the CLI",
			Run: func(cmd *cobra.Command, args []string) {
				url := "https://stripe.com/docs/dev-tools-csat"

				output := `
     _        _
 ___| |_ _ __(_)_ __   ___
/ __| __| '__| | '_ \ / _ \
\__ \ |_| |  | | |_) |  __/
|___/\__|_|  |_| .__/ \___|
               |_|

We'd love to know what you think of the CLI:

* Report bugs or issues on GitHub: https://github.com/stripe/stripe-cli/issues
* Leave us feedback on how you're using it or features you'd like to see: %s
				`

				fmt.Println(fmt.Sprintf(output, url))
			},
		},
	}
}
