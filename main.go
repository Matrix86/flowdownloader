package main

import (
	"encoding/base64"
	"flag"
	"fmt"

	"github.com/Matrix86/flowdownloader/hlss"
)

var (
	aesKey      string
	url         string
	resolutions map[string]string
	dwn_workers int
	resKeys     []string
	outFile     string
	segments    int
	downloaded  int
	decrypted   int
)

func decrypt_callback(file string) {
	if decrypted == 0 {
		fmt.Printf("\n")
	}
	decrypted++
	fmt.Printf("\r[@] Decrypted %d/%d", decrypted, segments)
}

func download_callback(file string) {
	downloaded++
	fmt.Printf("\r[@] Downloading %d/%d", downloaded, segments)
}

func main() {
	flag.StringVar(&aesKey, "k", "", "AES key")
	flag.StringVar(&url, "u", "", "Url m3u8")
	flag.StringVar(&outFile, "o", "video.mp4", "Output File")
	flag.IntVar(&dwn_workers, "w", 4, "Number of workers to download the segments")

	flag.Parse()

	if url == "" {
		fmt.Println("Url not setted")
		flag.Usage()
		return
	}

	var binary_key []byte
	if aesKey == "" {
		fmt.Println("[@] Warning : AES key is empty")
	} else {
		binary_key, _ = base64.StdEncoding.DecodeString(aesKey)
	}

	h, err := hlss.New(url, binary_key, outFile, download_callback, decrypt_callback, dwn_workers)

	resolutions := h.GetResolutions()

	fmt.Println("Choose resolution:")
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

	downloaded = 0
	decrypted = 0
	segments = h.GetTotSegments()

	if err = h.ExtractVideo(); err != nil {
		fmt.Printf("\n[!] Error : %s\n", err)
	} else {
		fmt.Println("\n[@] Completed")
	}
}
