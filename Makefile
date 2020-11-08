PROJECT_NAME := "flowdownloader"
TARGET=flowdownloader
LDFLAGS="-s -w"

all: build

build: clean
	@go build -o flowdownloader -v -ldflags=${LDFLAGS} .

windows:
	@GOOS=windows GOARCH=386 go build -o flowdownloader.exe .

clean:
	@rm -rf flowdownloader