package main

import (
	"time"
	"github.com/spf13/cobra"
	"fmt"
	"sort"
)

type ConsoleAdapter struct {
	Changes []ChangeItem
}

func init() {

	var consoleCmd = &cobra.Command{
		Use:   "console",
		Short: "Print out the latest changes to the console.",
		Run: func(cmd *cobra.Command, args []string) {

			adapter := ConsoleAdapter{}
			adapter.Changes = make([]ChangeItem, 0)

			config := JiraConfig{
				Url:       cmd.Flag("jurl").Value.String(),
				Username:  cmd.Flag("jusername").Value.String(),
				Password:  cmd.Flag("jpassword").Value.String(),
				JQL:       cmd.Flag("jql").Value.String(),
				RateLimit: 10,
			}
			process(config, &adapter)

		},
	}

	rootCmd.AddCommand(consoleCmd)
}

func (console *ConsoleAdapter) saveIssue(issue Issue) error {
	return nil
}

func (console *ConsoleAdapter) saveChange(item ChangeItem) error {
	console.Changes = append(console.Changes, item)
	return nil
}

func (consoleAdapter *ConsoleAdapter) getLastUpdated() (time.Time, error) {
	return time.Now().Add(-24 * time.Hour), nil

}

func (consoleAdapter *ConsoleAdapter) Commit() error {
	return nil
}
func (consoleAdapter *ConsoleAdapter) Begin() error {
	return nil
}
func (consoleAdapter *ConsoleAdapter) Finish() error {

	sort.Slice(consoleAdapter.Changes, func(a int, b int) bool {
		return consoleAdapter.Changes[a].Created.Before(consoleAdapter.Changes[b].Created)
	})
	prevKey := ""
	for _, item := range consoleAdapter.Changes {
		from := ""
		if prevKey != item.IssueKey {
			println()
			println()
			println(fmt.Sprintf("[%s] %s", item.IssueKey, item.IssueSummary))
			println()


		}
		if item.FromString != "" {
			from = fmt.Sprintf("%s --> ", item.FromString)
		}
		println(fmt.Sprintf("   %s -- %s: %s%s (%s)",
			item.Created.Format("2006-01-02 15:04"),
			item.Field,
			from,
			item.ToString,
			item.AuthorName))
		prevKey = item.IssueKey

	}
	return nil
}
