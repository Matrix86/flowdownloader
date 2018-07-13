SHELL := bash

all: flowdownloader


flowdownloader: 
	@go build -o flowdownloader .

windows:
	@GOOS=windows GOARCH=386 go build -o flowdownloader.exe .
