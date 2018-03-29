package main

import (
	"database/sql"
	"time"
	"encoding/json"
	"github.com/spf13/cobra"
)

type PostgresConfig struct {
	Host     string
	Username string
	Password string
	Db       string
	Table    string
}


type DbAdapter struct {
	Db *sql.DB
	tx *sql.Tx
}

func init() {
	pgConfig := PostgresConfig{}

	var toDbCmd = &cobra.Command{
		Use:   "todb",
		Short: "Save latest changes to postgresql db.",
		Run: func(cmd *cobra.Command, args []string) {
			db, err := sql.Open("postgres", "postgres://"+pgConfig.Username+":"+pgConfig.Password+"@"+pgConfig.Host+"/"+pgConfig.Db+"?sslmode=disable")
			if err != nil {
				panic("Can' open database " + err.Error())
			}
			defer db.Close()

			dbAdapter := DbAdapter{Db: db}
			config := JiraConfig{
				Url:       cmd.Flag("jurl").Value.String(),
				Username:  cmd.Flag("jusername").Value.String(),
				Password:  cmd.Flag("jpassword").Value.String(),
				JQL:       cmd.Flag("jql").Value.String(),
				RateLimit: 10,
			}
			process(config, dbAdapter)

		},
	}

	toDbCmd.Flags().StringVar(&pgConfig.Host, "pgserver", "localhost", "Postgres server host")
	toDbCmd.Flags().StringVar(&pgConfig.Username, "pgusername", "postgres", "Postgres username")
	toDbCmd.Flags().StringVar(&pgConfig.Password, "pgpassword", "", "Postgres password")
	toDbCmd.Flags().StringVar(&pgConfig.Db, "pgdb", "jira", "Postgres database")
	toDbCmd.Flags().StringVar(&pgConfig.Table, "pgtable", "", "Postgres database")

	rootCmd.AddCommand(toDbCmd)
}

func (db *DbAdapter) saveIssue(issue Issue) error {
	rawIssue := issue.Raw
	selector := "default"
	content, err := json.Marshal(rawIssue);
	if err != nil {
		return err
	}
	key := rawIssue["key"].(string)
	fields := rawIssue["fields"].(map[string]interface{})
	updatedString := fields["updated"].(string)
	updated, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedString)
	if err != nil {
		panic("Time could not been parsed " + err.Error())
	}

	if err != nil {
		print("Json can't be encoded " + err.Error())
	}
	_, err = db.tx.Exec("INSERT INTO issue (key,value, updated, selector) values ($1,$2,$3,$4) ON CONFLICT (key) DO UPDATE SET value = $2,updated=$3", key, string(content), updated, selector)
	if err != nil {
		println("SQL ERROR " + err.Error())
	}
	println(key + " is updated")
	return nil
}

func (adapter DbAdapter) saveChange(item ChangeItem) error {
	selector := "default"
	_, err := adapter.tx.Exec("INSERT INTO change ("+
		"created,selector,toString,fromString,author_name,author_key,history_id,item_index,field) values ($1,$2,$3,$4,$5,$6,$7,$8,$9)",
		item.Created,
		selector,
		item.ToString,
		item.FromString,
		item.AuthorName,
		item.AuthorKey,
		item.HistoryId,
		item.ItemIndex,
		item.Field)
	return err
}

func (db *DbAdapter) saveChangeItem(issue map[string]interface{}, selector string) error {
	content, err := json.Marshal(issue);
	if err != nil {
		return err
	}
	key := issue["key"].(string)
	fields := issue["fields"].(map[string]interface{})
	updatedString := fields["updated"].(string)
	updated, err := time.Parse("2006-01-02T15:04:05.000-0700", updatedString)
	if err != nil {
		panic("Time could not been parsed " + err.Error())
	}

	if err != nil {
		print("Json can't be encoded " + err.Error())
	}
	_, err = db.tx.Exec("INSERT INTO change (key,value, updated, selector) values ($1,$2,$3,$4) ON CONFLICT (key) DO UPDATE SET value = $2,updated=$3", key, string(content), updated, selector)
	if err != nil {
		println("SQL ERROR " + err.Error())
	}
	println(key + " is updated")
	return nil
}

func (db *DbAdapter) getLastUpdated() (time.Time, error) {
	selector := "default"
	result, err := db.Db.Query("select updated from issue WHERE selector = $1 order by updated desc limit 1", selector)
	if err != nil {
		return time.Now(), err
	}
	time := time.Time{}
	result.Next()
	result.Scan(&time)
	return time, nil

}

func (db *DbAdapter) Commit() error {
	return db.tx.Commit()
}
func (db *DbAdapter) Begin() error {
	tx, err := db.Db.Begin()
	db.tx = tx
	return err
}
