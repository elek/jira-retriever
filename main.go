package main

import (
	"encoding/json"
	_ "github.com/lib/pq"
	"net/url"
	"time"
	"strconv"
	"github.com/spf13/cobra"
	"github.com/elek/jira-retriever/jiradata"
	"crypto/sha1"
	"fmt"
	"strings"
	"log"
)


type JiraQueryResult struct {
	ErrorMessages []string
	StartAt       int
	MaxResults    int
	Total         int
	Issues        []map[string]interface{}
}

type BaseIssueInfo struct {
	IssueKey     string
	IssueSummary string
	Created      time.Time
}

type WithBaseIssueInformation interface {
	GetIssueKey() string
	GetIssueSummary() string
	GetCreated() time.Time
}

func (i *BaseIssueInfo) GetIssueKey() string {
	return i.IssueKey
}
func (i *BaseIssueInfo) GetIssueSummary() string {
	return i.IssueSummary
}
func (i *BaseIssueInfo) GetCreated() time.Time {
	return i.Created
}

type JiraItem struct {
	BaseIssueInfo
	Issue jiradata.Issue
}

type CommentItem struct {
	BaseIssueInfo
	Comment jiradata.Comment
}
type ChangeItem struct {
	BaseIssueInfo
	HistoryId    int
	ItemIndex    int
	FromString   string
	ToString     string
	AuthorKey    string
	AuthorName   string
	Field        string
}

var timeFormat = "2006-01-02T15:04:05.000-0700"


func JiraFromJson(data jiradata.Issue) JiraItem {
	created, err := time.Parse(timeFormat, data.Fields["created"].(string))
	if err != nil {
		panic(err)
	}
	issueRef := BaseIssueInfo{
		Created:      created,
		IssueKey:     data.Key,
		IssueSummary: data.Fields["summary"].(string)}
	return JiraItem{
		BaseIssueInfo: issueRef,
		Issue:         data,
	}
}


type Adapter interface {
	saveIssue(issue JiraItem, selector string) error
	saveChange(item ChangeItem, selector string) error
	saveComment(item CommentItem, selector string) error

	getLastUpdated(selector string) (time.Time, error)
	saveLastUpdated(lastUpdated time.Time, selector string) error

	Commit() error
	Begin() error

	Finish() error
}



var rootCmd = &cobra.Command{
	Use:   "jira-retriever",
	Short: "Script to get latest changes from jira project",
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Help()
	},
}

func main() {

	rootCmd.PersistentFlags().String("jurl", "http://localhost", "Base url for the jira API")
	rootCmd.PersistentFlags().String("jusername", "username", "Username for the jira")
	rootCmd.PersistentFlags().String("jpassword", "password", "Password for the jira")
	rootCmd.PersistentFlags().String("jql", "", "Custom JQL fragment to add to the query")
	rootCmd.PersistentFlags().String("since", "last", "Define timebox to the jira quey. Could be a "+
		"1.) unix epoch 2.) last (to check the results since the last run")

	rootCmd.Execute()
}

func process(config *JiraClient, adapter Adapter) {
	var err error
	var lastUpdated time.Time
	selector := getHash(config.JQL)
	epoch, err := strconv.Atoi(config.Since)
	if err != nil {
		if config.Since == "" || config.Since == "last" {
			lastUpdated, err = adapter.getLastUpdated(selector)
			if err != nil {
				panic(err)
			}
		} else {
			panic("Unknown since value")
		}

	} else {
		lastUpdated = time.Unix(int64(epoch), 0)
	}

	log.Print(fmt.Sprintf("Checking jira changes since %s", lastUpdated.Format(time.RFC3339)))
	queryLoop := true
	for queryLoop == true {

		jsonContent := readQuery(lastUpdated, config, config.JQL)
		if err != nil {
			panic("Can't load the jira json from the server API call: " + err.Error())
		}

		var searchResults jiradata.SearchResults

		err = json.Unmarshal(jsonContent, &searchResults)
		if err != nil {
			println("Json can't be parsed to typed object " + err.Error())
		}

		if len(searchResults.ErrorMessages) > 0 {
			panic(searchResults.ErrorMessages[0])
		}
		if searchResults.MaxResults == 0 {
			print("No more results")
			return
		}

		err = adapter.Begin()
		if err != nil {
			panic("Transaction couldn't been started")
		}
		for r := 0; r < len(searchResults.Issues); r++ {
			adapter.saveIssue(JiraFromJson(*searchResults.Issues[r]), selector)
			processHistory(lastUpdated, adapter, searchResults.Issues[r], selector);
			processComments(lastUpdated, adapter, searchResults.Issues[r], selector);

			updated, err := time.Parse(timeFormat, searchResults.Issues[r].Fields["updated"].(string))
			if err != nil {
				panic(err.Error())
			}

			if updated.After(lastUpdated) {
				lastUpdated = updated
			}
		}
		err = adapter.Commit()
		if err != nil {
			panic("Committing to the database was unsuccessful " + err.Error())
		}
		queryLoop = searchResults.MaxResults < searchResults.Total
	}
	adapter.saveLastUpdated(lastUpdated, selector)
	adapter.Finish()
}
func getHash(input string) string {
	h := sha1.New()
	h.Write([]byte(input))
	bs := h.Sum(nil)
	return strings.Trim(fmt.Sprintf("%x\n", bs), "\n")
}

func processComments(fromTime time.Time, adapter Adapter, issue *jiradata.Issue, selector string) {
	commentPage := issue.Fields["comment"].(map[string]interface{})
	var comments []jiradata.Comment

	marshalled, err := json.Marshal(commentPage["comments"])
	if err != nil {
		panic(err)
	}

	err = json.Unmarshal(marshalled, &comments)
	if err != nil {
		panic(err)
	}

	for _, comment := range comments {
		created, err := time.Parse(timeFormat, comment.Created)
		if err != nil {
			panic(err.Error())
		}
		if fromTime.Before(created) {
			adapter.saveComment(CommentItem{
				BaseIssueInfo: BaseIssueInfo{
					IssueKey:     issue.Key,
					IssueSummary: issue.Fields["summary"].(string),
					Created:      created,
				},
				Comment: comment,
			}, selector)
		}
	}
}

func processHistory(fromTime time.Time, adapter Adapter, issue *jiradata.Issue, selector string) {
	for _, history := range issue.Changelog.Histories {
		created, err := time.Parse(timeFormat, history.Created)
		if err != nil {
			panic(err.Error())
		}
		if created.After(fromTime) {

			for idx, item := range history.Items {

				historyId, err := strconv.Atoi(history.ID)
				if err != nil {
					panic(err)
				}
				changeItem := ChangeItem{
					BaseIssueInfo: BaseIssueInfo{
						IssueKey:     issue.Key,
						IssueSummary: issue.Fields["summary"].(string),
						Created:      created,
					},
					AuthorKey:  history.Author.Key,
					AuthorName: history.Author.DisplayName,
					HistoryId:  historyId,
					FromString: item.FromString,
					ToString:   item.ToString,
					Field:      item.Field,
					ItemIndex:  idx,
				}
				err = adapter.saveChange(changeItem, selector)
				if err != nil {
					panic(err.Error())
				}
			}

		}
	}
}

func readQuery(lastUpdated time.Time, jiraConfig *JiraClient, queryFragment string) []byte {
	sinceMs := lastUpdated.UnixNano() / 1000000
	if sinceMs < 0 {
		sinceMs = 0
	}
	query := "updated > " + strconv.FormatInt(sinceMs, 10) + " ORDER BY updated ASC"
	if len(queryFragment) > 0 {
		query = "(" + queryFragment + ") AND " + query
	}
	parameter := url.Values{"jql": []string{query}, "expand": []string{"changelog,comments"}, "fields": []string{"*all"}}

	return jiraConfig.queryWithParameters("/search", parameter)
}
