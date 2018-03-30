package main

import (
	"time"
	"github.com/spf13/cobra"
	"fmt"
	"strings"
	"sort"
)

type ConsoleAdapter struct {
	Changes []WithBaseIssueInformation
}

func init() {

	var consoleCmd = &cobra.Command{
		Use:   "console",
		Short: "Print out the latest changes to the console.",
		Run: func(cmd *cobra.Command, args []string) {

			adapter := ConsoleAdapter{}
			adapter.Changes = make([]WithBaseIssueInformation, 0)

			config := FromFlags(cmd)
			process(&config, &adapter)

		},
	}

	rootCmd.AddCommand(consoleCmd)
}

func (consoleAdapter *ConsoleAdapter) saveIssue(issue JiraItem) error {
	if issue.Issue.Fields["created"] == issue.Issue.Fields["updated"] {
		consoleAdapter.Changes = append(consoleAdapter.Changes, &issue)
	}
	return nil
}

func (consoleAdapter *ConsoleAdapter) saveChange(item ChangeItem) error {
	consoleAdapter.Changes = append(consoleAdapter.Changes, &item)
	return nil
}

func (consoleAdapter *ConsoleAdapter) saveComment(comment CommentItem) error {
	consoleAdapter.Changes = append(consoleAdapter.Changes, &comment)
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
		return consoleAdapter.Changes[a].GetCreated().Before(consoleAdapter.Changes[b].GetCreated())
	})

	prevKey := ""
	for _, genericItem := range consoleAdapter.Changes {
		if prevKey != genericItem.GetIssueKey() {
			println()
			println()
			println(fmt.Sprintf("[%s] %s", genericItem.GetIssueKey(), genericItem.GetIssueSummary()))
			println()
		}
		created := genericItem.GetCreated().Format("2006-01-02 15:04")
		switch item := genericItem.(type) {
		case *ChangeItem:
			from := ""

			if item.FromString != "" {
				from = fmt.Sprintf("%s --> ", item.FromString)
			}
			println(fmt.Sprintf("   %s -- %s: %s%s (%s)",
				created,
				item.Field,
				from,
				item.ToString,
				item.AuthorName))
			prevKey = item.IssueKey
			println()
		case *JiraItem:
			creator := item.Issue.Fields["creator"].(map[string]interface{})
			println(fmt.Sprintf("   %s -- CREATED by %s",
				created,
				creator["displayName"].(string)))
			println()
			println(item.Issue.Fields["description"].(string))
			println()
		case *CommentItem:
			println(fmt.Sprintf("   %s -- Comment (%s)",
				created,
				item.Comment.Author.DisplayName))
			println()
			if (item.Comment.Author.DisplayName != "genericqa") {
				comment := item.Comment.Body
				comment = strings.Replace(comment, "\n", "\n    ", -1)
				comment = strings.Replace(comment, "\n", "\n\n", -1)
				comment = "    " + comment
				println(comment)
				println()
			}

		}

	}
	return nil
}