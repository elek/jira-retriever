package main

import (
	"net/http"
	"io/ioutil"
	"encoding/json"
	_ "github.com/lib/pq"
	"database/sql"
	"net/url"
	"time"
	"strconv"
	"github.com/namsral/flag"
)

type config struct {
}

type JiraQueryResult struct {
	ErrorMessages []string
	StartAt       int
	MaxResults    int
	Total         int
	Issues        []map[string]interface{}
}

type JiraConfig struct {
	Url      string
	Username string
	Password string
}

type PostgresConfig struct {
	Host     string
	Username string
	Password string
	Db       string
	Table    string
}

type ChangeItem struct {
	HistoryId  int
	ItemIndex  int
	FromString string
	ToString   string
	AuthorKey  string
	AuthorName string
	Created    time.Time
	Field      string
}

func main() {
	selector := "default"
	var ratelimit int
	var queryFragment string = ""
	pgConfig := PostgresConfig{}
	jiraConfig := JiraConfig{}
	flag.StringVar(&pgConfig.Host, "pgserver", "localhost", "Postgres server host")
	flag.StringVar(&pgConfig.Username, "pgusername", "username", "Postgres username")
	flag.StringVar(&pgConfig.Password, "pgpassword", "password", "Postgres password")
	flag.StringVar(&pgConfig.Db, "pgdb", "database", "Postgres database")
	flag.StringVar(&pgConfig.Table, "pgtable", "jiraissues", "Postgres database")

	flag.StringVar(&jiraConfig.Url, "jurl", "http://localhost", "Base url for the jira API")
	flag.StringVar(&jiraConfig.Username, "jusername", "username", "Username for the jira")
	flag.StringVar(&jiraConfig.Password, "jpassword", "password", "Password for the jira")

	flag.StringVar(&selector, "selector", "default", "Selector for the postgres Db")
	flag.IntVar(&ratelimit, "ratelimit", 10, "Limit the requests to 1 request/ratelimit sec.")
	flag.StringVar(&queryFragment, "jql", "", "Custom JQL fragment to add to the query")
	flag.Parse()
	db, err := sql.Open("postgres", "postgres://"+pgConfig.Username+":"+pgConfig.Password+"@"+pgConfig.Host+"/"+pgConfig.Db+"?sslmode=disable")
	if err != nil {
		panic("Can' open database " + err.Error())
	}
	defer db.Close()

	dbAdapter := DbAdapter{Db: db}

	lastJiraCall := time.Time{}
	queryLoop := true
	for queryLoop == true {
		lastUpdated, err := dbAdapter.getLastUpdated(pgConfig.Table, selector)
		if err != nil {
			panic("Last update can' be determined " + err.Error())
		}

		//throttle the queries
		duration := time.Since(lastJiraCall)
		if duration.Seconds() < float64(ratelimit) {
			time.Sleep(time.Duration(ratelimit)*time.Second - duration)
		}

		jsonContent := read_query(lastUpdated, jiraConfig, queryFragment)
		if err != nil {
			panic("Can't load the jira json from the server API call: " + err.Error())
		}
		lastJiraCall = time.Now()

		var jsonResult JiraQueryResult

		err = json.Unmarshal(jsonContent, &jsonResult);
		if err != nil {
			println("Json can't be parsed " + err.Error())
		}

		if (len(jsonResult.ErrorMessages) > 0) {
			panic(jsonResult.ErrorMessages[0])
		}
		if (jsonResult.MaxResults == 0) {
			print("No more results")
			return
		}

		err = dbAdapter.Begin()
		if err != nil {
			panic("Transaction couldn't been started")
		}
		for r := 0; r < len(jsonResult.Issues); r++ {
			dbAdapter.saveIssue(pgConfig.Table, jsonResult.Issues[r], selector)
			processHistory(dbAdapter, jsonResult.Issues[r], selector);
		}
		err = dbAdapter.Commit()
		if err != nil {
			panic("Commiting to the database was unsuccesfull " + err.Error())
		}
		queryLoop = jsonResult.MaxResults < jsonResult.Total
	}
}

func processHistory(adapter DbAdapter, issue map[string]interface{}, selector string) {

	changelog := issue["changelog"].(map[string]interface{})
	histories := changelog["histories"].([]interface{})
	for _, historyRaw := range histories {
		history := historyRaw.(map[string]interface{})
		items := history["items"].([]interface{})
		historyId, err := strconv.Atoi(history["id"].(string))
		if err != nil {
			panic(err.Error())
		}

		created, err := time.Parse("2006-01-02T15:04:05.000-0700", history["created"].(string))
		if err != nil {
			panic(err.Error())
		}

		for idx, itemRaw := range items {
			item := itemRaw.(map[string]interface{})
			author := history["author"].(map[string]interface{})

			var fromString, toString string
			if item["fromString"] != nil {
				fromString = item["fromString"].(string)
			}
			if item["toString"] != nil {
				fromString = item["toString"].(string)
			}
			changeItem := ChangeItem{
				AuthorKey:  author["key"].(string),
				AuthorName: author["displayName"].(string),
				HistoryId:  historyId,
				FromString: fromString,
				ToString:   toString,
				Field:      item["field"].(string),
				Created:    created,
				ItemIndex:  idx,
			}
			err = adapter.saveChange(changeItem, selector)
			if err != nil {
				panic(err.Error())
			}

		}
	}
}

func read_query(lastUpdated time.Time, jiraConfig JiraConfig, queryFragment string) []byte {
	jiraBaseUrl := jiraConfig.Url
	jiraUrl := jiraBaseUrl + "/rest/api/2/search"
	sinceMs := lastUpdated.UnixNano() / 1000000
	if sinceMs < 0 {
		sinceMs = 0
	}
	query := "updated > " + strconv.FormatInt(sinceMs, 10) + " ORDER BY updated ASC"
	if len(queryFragment) > 0 {
		query = queryFragment + " AND " + query
	}
	parameter := url.Values{"jql": []string{query}, "expand": []string{"changelog"}}
	jiraUrl += "?" + parameter.Encode()

	client := http.Client{}
	println("Retrieve jira json from " + jiraUrl)
	req, err := http.NewRequest("GET", jiraUrl, nil)
	client.Do(req)
	if (jiraConfig.Username != "username") {
		req.SetBasicAuth(jiraConfig.Username, jiraConfig.Password)
	}
	response, err := client.Do(req)
	if err != nil {
		panic("Jira url couldn' be opened " + err.Error())
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
