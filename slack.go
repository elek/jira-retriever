package main

import (
	"time"
	"github.com/spf13/cobra"
	"fmt"
	"strings"
	"sort"
	"bytes"
	"github.com/nlopes/slack"
	"strconv"
	"encoding/json"
)

type SlackAdapter struct {
	Changes   []WithBaseIssueInformation
	selector  string
	fileState FileState
	Channel   string
	Token     string
}

func init() {
	var token, channel string
	var consoleCmd = &cobra.Command{
		Use:   "slack",
		Short: "Send the latest changes to slack",
		Run: func(cmd *cobra.Command, args []string) {

			adapter := SlackAdapter{
				Channel: channel,
				Token:   token,
			}
			adapter.Changes = make([]WithBaseIssueInformation, 0)

			config := FromFlags(cmd)
			process(&config, &adapter)

		},
	}
	consoleCmd.Flags().StringVar(&token, "token", "", "Slack authorization token")
	consoleCmd.Flags().StringVar(&channel, "channel", "sandbox", "Channel to send to message to")
	rootCmd.AddCommand(consoleCmd)
}

func (slackAdapter *SlackAdapter) saveIssue(issue JiraItem, selector string) error {
	if issue.Issue.Fields["created"] == issue.Issue.Fields["updated"] {
		slackAdapter.Changes = append(slackAdapter.Changes, &issue)
	}
	return nil
}

func (slackAdapter *SlackAdapter) saveChange(item ChangeItem, selector string) error {
	slackAdapter.Changes = append(slackAdapter.Changes, &item)
	return nil
}

func (slackAdapter *SlackAdapter) saveComment(comment CommentItem, selector string) error {
	slackAdapter.Changes = append(slackAdapter.Changes, &comment)
	return nil
}

func (slackAdapter *SlackAdapter) getLastUpdated(selector string) (time.Time, error) {
	state := CreateFileState(selector)
	return state.read()
}
func (slackAdapter *SlackAdapter) saveLastUpdated(lastUpdated time.Time, selector string) error {
	state := CreateFileState(selector)
	return state.write(lastUpdated)
}

func (slackAdapter *SlackAdapter) Commit() error {
	return nil
}
func (slackAdapter *SlackAdapter) Begin() error {
	return nil
}
func (slackAdapter *SlackAdapter) Finish() error {
	sort.Slice(slackAdapter.Changes, func(a int, b int) bool {
		return slackAdapter.Changes[a].GetCreated().Before(slackAdapter.Changes[b].GetCreated())
	})

	var buffer bytes.Buffer
	attachments := make([]slack.Attachment, 0)
	prevKey := ""
	for _, genericItem := range slackAdapter.Changes {
		if prevKey != genericItem.GetIssueKey() {
			if buffer.Len() > 0 {
				slackAdapter.PostMessage(buffer.String(), attachments)
				buffer.Reset()
				attachments = make([]slack.Attachment, 0)

			}
			buffer.WriteString(fmt.Sprintf("<https://issues.apache.org/jira/browse/"+
				"%s|%s> *%s*",
				genericItem.GetIssueKey(),
				genericItem.GetIssueKey(),
				genericItem.GetIssueSummary()))
		}
		switch item := genericItem.(type) {
		case *ChangeItem:
			from := ""

			if item.FromString != "" {
				from = fmt.Sprintf("%s --> ", item.FromString)
			}

			if item.Field != "Comment" {
				attachment := slack.Attachment{
					AuthorName: item.AuthorName,
					Title:      item.Field + " field is changed",
					Text:       fmt.Sprintf("%s %s", from, item.ToString),
					MarkdownIn: []string{"text", "footer", "title"},
					Ts:         json.Number(strconv.Itoa(int(genericItem.GetCreated().Unix()))),
				}
				attachments = append(attachments, attachment)
			}
		case *JiraItem:
			creator := item.Issue.Fields["creator"].(map[string]interface{})

			attachment := slack.Attachment{
				AuthorName: creator["displayName"].(string),
				Title:      "Issue is created",
				Text:       item.Issue.Fields["description"].(string),
				Ts:         json.Number(strconv.Itoa(int(genericItem.GetCreated().Unix()))),
			}
			attachments = append(attachments, attachment)
		case *CommentItem:
			comment := ""
			if item.Comment.Author.DisplayName != "genericqa" && item.Comment.Author.DisplayName != "Hadoop QA" {
				comment = item.Comment.Body
				comment = strings.Replace(comment, "\n", "\n\n", -1)
			}

			attachment := slack.Attachment{
				AuthorName: item.Comment.Author.DisplayName,
				Title:      "Comment",
				Text:       comment,
				MarkdownIn: []string{"text", "footer", "title"},
				Ts:         json.Number(strconv.Itoa(int(genericItem.GetCreated().Unix()))),
			}
			attachments = append(attachments, attachment)

		}
		prevKey = genericItem.GetIssueKey()

	}
	if buffer.Len() > 0 {
		slackAdapter.PostMessage(buffer.String(), attachments)
	}

	return nil
}

func (slackAdapter *SlackAdapter) PostMessage(message string, attachments []slack.Attachment) {
	api := slack.New(slackAdapter.Token)
	parameters := slack.NewPostMessageParameters()
	parameters.Attachments = attachments
	parameters.Username = "Jira changes bot"
	parameters.EscapeText = false
	_, response, err := api.PostMessage(slackAdapter.Channel, message, parameters)
	if err != nil {
		panic(response)
	}
}
