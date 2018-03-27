package main

import (
	"database/sql"
	"time"
	"encoding/json"
)

type DbAdapter struct {
	Db *sql.DB
	tx *sql.Tx
}

func (db *DbAdapter) saveIssue(tableName string, issue map[string]interface{}, selector string) error {
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
	_, err = db.tx.Exec("INSERT INTO "+tableName+" (key,value, updated, selector) values ($1,$2,$3,$4) ON CONFLICT (key) DO UPDATE SET value = $2,updated=$3", key, string(content), updated, selector)
	if err != nil {
		println("SQL ERROR " + err.Error())
	}
	println(key + " is updated")
	return nil
}
func (db *DbAdapter) getLastUpdated(tableName string, selector string) (time.Time, error) {
	result, err := db.Db.Query("select updated from "+tableName+" WHERE selector = $1 order by updated desc limit 1", selector)
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
