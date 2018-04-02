package main

import (
	"time"
	"path"
	"os"
	"strings"
	"io/ioutil"
	"errors"
	"strconv"
	"log"
)

type FileState struct {
	FileName string
}

func CreateFileState(selector string) *FileState {
	stateDir := path.Join(os.Getenv("HOME"), ".jira-retriever")
	stateFile := path.Join(stateDir, selector+".state")
	os.MkdirAll(stateDir, os.ModePerm)
	state := FileState{FileName: stateFile}
	log.Print("Using statefile: " + stateFile)
	return &state

}
func (fileState *FileState) read() (time.Time, error) {
	if _, err := os.Stat(fileState.FileName); os.IsNotExist(err) {
		return time.Now().Add(time.Duration(-24*5) * time.Hour), nil
	}
	lastTime, err := ioutil.ReadFile(fileState.FileName) // just pass the file name
	if err != nil {
		return time.Time{}, errors.New(err.Error())
	}
	epoch, err := strconv.Atoi(strings.Trim(string(lastTime), "\n"))
	if err != nil {
		return time.Time{}, errors.New(err.Error())
	}
	return time.Unix(int64(epoch), 0), nil
}
func (fileState *FileState) write(updated time.Time) error {
	return ioutil.WriteFile(fileState.FileName,
		[]byte(strconv.FormatInt(updated.Unix(), 10)),
		os.ModePerm)
}
