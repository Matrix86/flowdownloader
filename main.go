package main

import (
	"encoding/base64"
	"flag"
	"fmt"
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
)

func decryptCallback(file string, done int, total int) {
	if decrypted == 0 {
		fmt.Printf("\n")
	}
	decrypted++
	fmt.Printf("\r[@] Decrypting %d/%d", done, total)
}

func downloadCallback(file string, done int, total int) {
	downloaded++
	fmt.Printf("\r[@] Downloading %d/%d", done, total)
}

func main() {
	flag.StringVar(&aesKey, "k", "", "AES key (base64 encoded or http url)")
	flag.StringVar(&url, "u", "", "Url master m3u8")
	flag.StringVar(&outFile, "o", "video.mp4", "Output File")
	flag.IntVar(&dwnWorkers, "w", 4, "Number of workers to download the segments")
	flag.BoolVar(&isSecondary, "s", false, "If true the url used on -u parameter will be considered as the secondary index url.")
	flag.StringVar(&cookieFile, "c", "", "File with authentication cookies.")
	flag.StringVar(&referer, "r", "", "Set the http referer.")

	flag.Parse()

	if url == "" {
		fmt.Println("Url not setted")
		flag.Usage()
		return
	}

	var binaryKey []byte
	var keyUrl string
	if aesKey == "" {
		fmt.Println("[@] Warning : AES key is empty")
	} else {
		if strings.HasPrefix(aesKey, "http") {
			keyUrl = aesKey
		} else {
			binaryKey, _ = base64.StdEncoding.DecodeString(aesKey)
		}
	}

	h, err := hlss.New(url, binaryKey, outFile, downloadCallback, decryptCallback, dwnWorkers, cookieFile, referer, keyUrl)
	if err != nil {
		fmt.Printf("[!] Error: %s\n", err)
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
			fmt.Println(err)
			return
		}

		if err = h.SetResolution(i); err != nil {
			fmt.Printf("[!] Error : %s\n", err)
			return
		}
	}

	downloaded = 0
	decrypted = 0
	segments = h.GetTotSegments()

	fmt.Println("[@] Starting download...\n")
	if err = h.ExtractVideo(); err != nil {
		fmt.Printf("\n[!] Error : %s\n", err)
	} else {
		fmt.Println("\n[@] Completed")
	}
}
