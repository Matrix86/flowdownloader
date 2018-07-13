package hlss

import (
	"bufio"
	"encoding/hex"
	"errors"
	"net/http"
	"os"
	"regexp"
	"strings"

	"github.com/Matrix86/flowdownloader/downloader"
	"github.com/Matrix86/flowdownloader/utils"
)

var keyRegex = regexp.MustCompile("#EXT-X-KEY:METHOD=AES-128,URI=\"([^\"]+)\",IV=(0x[a-f0-9]+)")

type DecryptCallback func(string)

type Hlss struct {
	base_url          string
	key               []byte
	iv                []byte
	main_idx          string
	secondary_idx     []string
	segments          []string
	file              string
	pout              *os.File
	resolutions       map[string]string
	res_keys          []string
	secondary_url     string
	bandwidths        map[string]string
	bandwidth_keys    []string
	download_callback downloader.Callback
	decrypt_callback  DecryptCallback
	download_worker   int
}

func New(main_url string, key []byte, outputfile string, download_callback downloader.Callback, decrypt_callback DecryptCallback, download_worker int) (*Hlss, error) {
	obj := Hlss{
		main_idx:          main_url,
		key:               key,
		file:              outputfile,
		download_callback: download_callback,
		decrypt_callback:  decrypt_callback,
		download_worker:   download_worker,
	}

	obj.resolutions = make(map[string]string)
	if err := obj.parseMainIndex(); err != nil {
		return nil, err
	}

	obj.base_url = utils.GetBaseUrl(main_url)

	return &obj, nil
}

func (h *Hlss) parseMainIndex() error {
	r, err := http.Get(h.main_idx)
	if err != nil {
		return err
	}

	defer r.Body.Close()

	scanner := bufio.NewScanner(r.Body)
	scanner.Split(bufio.ScanLines)

	var currentResolution string
	var resolution_keys []string
	var currentBandwidth string
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && !strings.HasPrefix(line, "#EXTM3U") {
			return errors.New("Invalid m3u file format")
		} else {
			firstLine = false
		}

		if strings.HasPrefix(line, "#EXT-X-STREAM-INF") {
			line = line[len("#EXT-X-STREAM-INF:"):]

			params := strings.Split(line, ",")
			if len(params) < 2 {
				return errors.New("Invalid m3u file format")
			}
			for _, info := range params {
				if strings.HasPrefix(info, "BANDWIDTH=") {
					currentBandwidth = info[len("BANDWIDTH="):]
				} else if strings.HasPrefix(info, "RESOLUTION=") {
					currentResolution = info[len("RESOLUTION="):]
				}
			}
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if currentBandwidth != "" || currentResolution != "" {
			currentTrack := currentBandwidth
			if currentResolution != "" {
				currentTrack = "[" + currentResolution + "] " + currentTrack
			}
			resolution_keys = append(resolution_keys, currentTrack)
			h.resolutions[currentTrack] = scanner.Text()
			currentResolution = ""
			currentBandwidth = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	h.res_keys = resolution_keys

	return nil
}

func (h *Hlss) parseSecondaryIndex() error {
	r, e := http.Get(h.secondary_url)
	if e != nil {
		return e
	}

	defer r.Body.Close()

	scanner := bufio.NewScanner(r.Body)
	scanner.Split(bufio.ScanLines)

	base_url := utils.GetBaseUrl(h.secondary_url)

	keyParsed := false
	for scanner.Scan() {
		if keyParsed == false {
			matches := keyRegex.FindStringSubmatch(scanner.Text())
			if len(matches) == 3 {
				//keyUrl = matches[1]
				str_iv := matches[2]
				keyParsed = true

				str_iv = str_iv[2:]
				h.iv, e = hex.DecodeString(str_iv)
				if e != nil {
					return e
				}
			}
		} else {
			line := scanner.Text()
			if line[0:1] != "#" {
				h.segments = append(h.segments, base_url+line)
			}
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}

	return nil
}

func (h *Hlss) downloadSegments() error {
	d := downloader.New(h.download_worker, ".", h.download_callback)
	d.SetUrls(h.segments)
	d.StartDownload()

	return nil
}

func (h *Hlss) decryptSegments() error {
	pout, err := os.Create(h.file)
	defer pout.Close()
	if err != nil {
		return err
	}

	for _, url := range h.segments {
		name := utils.GetFileFromUrl(url)

		if len(h.key) != 0 {
			if err = utils.DecryptFileAppend(pout, name, h.key, h.iv); err != nil {
				return err
			}
		} else {
			// we assume that the segments are not encrypted
			if err = utils.FileAppend(pout, name); err != nil {
				return err
			}
		}

		os.Remove(name)

		if h.decrypt_callback != nil {
			h.decrypt_callback(name)
		}
	}

	return nil
}

//! Public methods

func (h *Hlss) ExtractVideo() error {
	var err error
	if err = h.downloadSegments(); err != nil {
		return err
	}

	if err = h.decryptSegments(); err != nil {
		return err
	}

	return nil
}

func (h *Hlss) GetResolutions() []string {
	return h.res_keys
}

func (h *Hlss) SetResolution(res_idx int) error {
	if res_idx >= len(h.res_keys) {
		return errors.New("Resolution not found")
	}

	h.secondary_url = h.base_url + h.resolutions[h.res_keys[res_idx]]

	err := h.parseSecondaryIndex()

	return err
}

func (h *Hlss) GetTotSegments() int {
	return len(h.segments)
}

func (h *Hlss) GetBandwidths() []string {
	return h.bandwidth_keys
}
