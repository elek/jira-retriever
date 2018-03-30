package main

import (
	"time"
	"net/url"
	"net/http"
	"strconv"
	"io/ioutil"
	"github.com/spf13/cobra"
)

type JiraConfig struct {
	Url          string
	Username     string
	Password     string
	RateLimit    int
	JQL          string
	Since        time.Time
	lastJiraCall time.Time
}

func (jiraConfig *JiraConfig) query(query string) []byte {
	return jiraConfig.queryWithParameters(query, make(map[string][]string))
}

func FromFlags(cmd *cobra.Command) JiraConfig {

	since, _ := strconv.Atoi(cmd.Flag("since").Value.String())

	config := JiraConfig{
		Url:       cmd.Flag("jurl").Value.String(),
		Username:  cmd.Flag("jusername").Value.String(),
		Password:  cmd.Flag("jpassword").Value.String(),
		JQL:       cmd.Flag("jql").Value.String(),
		RateLimit: 10,
	}
	if since > 0 {
		config.Since = time.Unix(int64(since), 0)
	}
	return config
}
func (jiraConfig *JiraConfig) queryWithParameters(query string, parameters url.Values) []byte {
	jiraBaseUrl := jiraConfig.Url
	jiraUrl := jiraBaseUrl + "/rest/api/2" + query

	//throttle the queries
	duration := time.Since(jiraConfig.lastJiraCall)
	if duration.Seconds() < float64(jiraConfig.RateLimit) {
		time.Sleep(time.Duration(jiraConfig.RateLimit)*time.Second - duration)
	}

	jiraUrl += "?" + parameters.Encode()

	client := http.Client{}
	println("Calling jira REST api " + jiraUrl)
	req, err := http.NewRequest("GET", jiraUrl, nil)
	jiraConfig.lastJiraCall = time.Now()
	client.Do(req)
	if jiraConfig.Username != "username" {
		req.SetBasicAuth(jiraConfig.Username, jiraConfig.Password)
	}
	response, err := client.Do(req)
	if err != nil {
		panic("Jira url couldn't be opened " + err.Error())
	}

	defer response.Body.Close()
	if err != nil {
		panic("Can' read body " + err.Error())
	}
	if response.StatusCode > 400 {
		panic("Jira API is responded with error: HTTP " + strconv.Itoa(response.StatusCode))
	}
	body, err := ioutil.ReadAll(response.Body)
	return body
}
