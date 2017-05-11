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

	flag.StringVar(&selector, "selector", "default", "Selector for the postgres db")
	flag.IntVar(&ratelimit, "ratelimit", 10, "Limit the requests to 1 request/ratelimit sec.")
	flag.StringVar(&queryFragment, "jql", "", "Custom JQL fragment to add to the query")
	flag.Parse()
	db, err := sql.Open("postgres", "postgres://" + pgConfig.Username + ":" + pgConfig.Password + "@" + pgConfig.Host + "/" + pgConfig.Db + "?sslmode=disable")
	if err != nil {
		panic("Can' open database " + err.Error())
	}
	defer db.Close()

	lastJiraCall := time.Time{}
	queryLoop := true
	for queryLoop == true {
		lastUpdated, err := getLastUpdated(db, pgConfig.Table, selector)
		if err != nil {
			panic("Last update can' be determined " + err.Error())
		}

		//throttle the queries
		duration := time.Since(lastJiraCall)
		if duration.Seconds() < float64(ratelimit) {
			time.Sleep(time.Duration(ratelimit) * time.Second - duration)
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

		tx, err := db.Begin()
		for r := 0; r < len(jsonResult.Issues); r++ {
			saveIssue(db, tx, pgConfig.Table, jsonResult.Issues[r], selector)
		}
		err = tx.Commit()
		if err != nil {
			panic("Commiting to the database was unsuccesfull " + err.Error())
		}
		queryLoop = jsonResult.MaxResults < jsonResult.Total
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
	parameter := url.Values{"jql" : []string{query}, "expand":[]string{"changelog"}}
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

func saveIssue(db *sql.DB, tx *sql.Tx, tableName string, issue map[string]interface{}, selector string) error {
	content, err := json.Marshal(issue);
	if err != nil {
		return err
	}
	key := issue["key"].(string)
	fields := issue["fields"].(map[string]interface{})
	updatedString := fields["updated"].(string)
	updated, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedString)
	if (err != nil) {
		panic("Time could not been parsed " + err.Error())
	}

	if err != nil {
		print("Json can't be encoded " + err.Error())
	}

	_, err = tx.Exec("INSERT INTO " + tableName + " (key,value, updated, selector) values ($1,$2,$3,$4) ON CONFLICT (key) DO UPDATE SET value = $2,updated=$3", key, string(content), updated, selector)
	if err != nil {
		println("SQL ERROR " + err.Error())
	}
	println(key + " is updated")
	return nil
}
func getLastUpdated(db *sql.DB, tableName string, selector string) (time.Time, error) {
	result, err := db.Query("select updated from " + tableName + " WHERE selector = $1 order by updated desc limit 1", selector)
	if (err != nil) {
		return time.Now(), err
	}
	time := time.Time{}
	result.Next()
	result.Scan(&time)
	return time, nil

}
