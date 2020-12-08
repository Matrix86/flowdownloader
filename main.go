package main

import (
	"encoding/base64"
	"flag"
	"fmt"
	"github.com/Matrix86/flowdownloader/utils"
	"github.com/evilsocket/islazy/log"
	"github.com/evilsocket/islazy/tui"
	"runtime"
	"strings"

	"github.com/Matrix86/flowdownloader/hlss"
)

var (
	aesKey      string
	url         string
	cookieFile  string
	outFile     string
	referer     string
	dwnWorkers  int
	segments    int
	downloaded  int
	decrypted   int
	isSecondary bool
	debugFlag   bool
)

func decryptCallback(file string, done int, total int) {
	if decrypted == 0 {
		fmt.Printf("\n")
	}
	if decrypted != 0 && log.Level != log.DEBUG {
		fmt.Print("\033[A")// move cursor up
	}
	decrypted++
	fmt.Printf("\r[@] Decrypting %d/%d\n", done, total)
}

func downloadCallback(file string, done int, total int) {
	if downloaded != 0 && log.Level != log.DEBUG {
		fmt.Print("\033[A")// move cursor up
	}
	downloaded++
	fmt.Printf("\r[@] Downloading %d/%d\n", done, total)
}

func main() {
	flag.StringVar(&aesKey, "k", "", "AES key (base64 encoded or http url)")
	flag.StringVar(&url, "u", "", "Url master m3u8")
	flag.StringVar(&outFile, "o", "video.mp4", "Output File")
	flag.IntVar(&dwnWorkers, "w", 4, "Number of workers to download the segments")
	flag.BoolVar(&isSecondary, "s", false, "If true the url used on -u parameter will be considered as the secondary index url.")
	flag.StringVar(&cookieFile, "c", "", "File with authentication cookies.")
	flag.StringVar(&referer, "r", "", "Set the http referer.")
	flag.BoolVar(&debugFlag, "debug", false, "Enable debug logs.")

	flag.Parse()

	appName := fmt.Sprintf("%s v%s", utils.Name, utils.Version)
	appBuild := fmt.Sprintf("(built for %s %s with %s)", runtime.GOOS, runtime.GOARCH, runtime.Version())
	appAuthor := fmt.Sprintf("Author: %s", utils.Author)

	fmt.Printf("%s %s\n%s\n", tui.Bold(appName), tui.Dim(appBuild), tui.Dim(appAuthor))

	log.Output = ""
	log.Level = log.INFO
	log.OnFatal = log.ExitOnFatal
	log.Format = "[{datetime}] {level:color}{level:name}{reset} {message}"

	if debugFlag {
		log.Level = log.DEBUG
	}

	if url == "" {
		log.Error("url not setted...exiting")
		flag.Usage()
		return
	}

	var binaryKey []byte
	var keyUrl string
	if aesKey == "" {
		log.Warning("AES key is empty")
	} else {
		if strings.HasPrefix(aesKey, "http") {
			keyUrl = aesKey
		} else {
			binaryKey, _ = base64.StdEncoding.DecodeString(aesKey)
		}
	}

	h, err := hlss.New(url, binaryKey, outFile, downloadCallback, decryptCallback, dwnWorkers, cookieFile, referer, keyUrl)
	if err != nil {
		log.Error("%s", err)
		return
	}

	if isSecondary == false {
		resolutions := h.GetResolutions()
		fmt.Println("Choose resolution/bandwidth:")
		for i, k := range resolutions {
			fmt.Printf(" %d) %s\n", i, k)
		}

		fmt.Print("> ")
		var i int
		_, err = fmt.Scanf("%d", &i)
		if err != nil {
			log.Error("%s", err)
			return
		}

		if err = h.SetResolution(i); err != nil {
			log.Error("resolution selection: %s", err)
			return
		}
	}

	downloaded = 0
	decrypted = 0
	segments = h.GetTotSegments()

	log.Info("download is starting...")
	if err = h.ExtractVideo(); err != nil {
		log.Error("%s", err)
	} else {
		log.Info("download completed")
	}
}
