package hlss

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"os"
	"strings"

	"github.com/Matrix86/flowdownloader/downloader"
	"github.com/Matrix86/flowdownloader/utils"
)

type DecryptCallback func(string, int, int)

type Hlss struct {
	baseUrl          string
	key              []byte
	iv               []byte
	mainIdx          string
	secondaryIdx     []string
	segments         []string
	file             string
	pout             *os.File
	resolutions      map[string]string
	resKeys          []string
	secondaryUrl     string
	bandwidths       map[string]string
	bandwidthKeys    []string
	downloadCallback downloader.Callback
	decryptCallback  DecryptCallback
	downloadWorker   int
	cookies          []*http.Cookie
	referer          string
}

func New(mainUrl string, key []byte, outputfile string, downloadCallback downloader.Callback, decryptCallback DecryptCallback, downloadWorker int, cookieFile string, referer string, keyUrl string) (*Hlss, error) {
	obj := Hlss{
		mainIdx:          mainUrl,
		key:              key,
		file:             outputfile,
		downloadCallback: downloadCallback,
		decryptCallback:  decryptCallback,
		downloadWorker:   downloadWorker,
		referer:          referer,
	}

	if cookieFile != "" {
		err := obj.setCookies(cookieFile)
		if err != nil {
			return nil, err
		}
	}

	// Try to get key from URL
	if keyUrl != "" {
		client := &http.Client{}
		req, err := http.NewRequest("GET", keyUrl, nil)
		if err != nil {
			return nil, err
		}
		if len(obj.cookies) > 0 {
			for _, c := range obj.cookies {
				req.AddCookie(c)
			}
		}
		if obj.referer != "" {
			req.Header.Set("Referer", obj.referer)
		}
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		} else if resp.StatusCode != 200 {
			return nil, fmt.Errorf("http response status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
		}
		defer resp.Body.Close()
		buf, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		obj.key = buf
	}

	obj.resolutions = make(map[string]string)
	if err := obj.parseMainIndex(); err != nil {
		return nil, err
	}

	obj.baseUrl = utils.GetBaseUrl(mainUrl)

	return &obj, nil
}

func (h *Hlss) parseMainIndex() error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", h.mainIdx, nil)
	if err != nil {
		return err
	}
	if len(h.cookies) > 0 {
		for _, c := range h.cookies {
			req.AddCookie(c)
		}
	}
	if h.referer != "" {
		req.Header.Set("Referer", h.referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("http response status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	var currentResolution string
	var resolutionKeys []string
	var currentBandwidth string
	firstLine := true

	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && !strings.HasPrefix(line, "#EXTM3U") {
			return errors.New("parseMainIndex: Invalid m3u file format")
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
			resolutionKeys = append(resolutionKeys, currentTrack)
			h.resolutions[currentTrack] = scanner.Text()
			currentResolution = ""
			currentBandwidth = ""
		}
	}
	if err := scanner.Err(); err != nil {
		return err
	}

	h.resKeys = resolutionKeys

	return nil
}

func (h *Hlss) parseSecondaryIndex() error {
	client := &http.Client{}
	req, err := http.NewRequest("GET", h.secondaryUrl, nil)
	if err != nil {
		return err
	}
	if len(h.cookies) > 0 {
		for _, c := range h.cookies {
			req.AddCookie(c)
		}
	}
	if h.referer != "" {
		req.Header.Set("Referer", h.referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("http response status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	baseUrl := utils.GetBaseUrl(h.secondaryUrl)

	firstLine := true
	getSegment := false
	//keyUrl := ""
	iv := ""
	for scanner.Scan() {
		line := scanner.Text()
		if firstLine && !strings.HasPrefix(line, "#EXTM3U") {
			return errors.New("parseSecondaryIndex: Invalid m3u file format")
		} else {
			firstLine = false
		}

		if strings.HasPrefix(line, "#EXTINF") {
			getSegment = true
		} else if strings.HasPrefix(line, "#EXT-X-KEY:") {
			line = line[len("#EXT-X-KEY:"):]

			params := strings.Split(line, ",")
			if len(params) < 2 {
				return errors.New("Invalid m3u file format")
			}
			for _, info := range params {
				if strings.HasPrefix(info, "URI=\"") {
					//keyUrl = info[len("URI=\"") : len(info)-1]
				} else if strings.HasPrefix(info, "IV=") {
					iv = info[len("IV="):]
					fmt.Println("[@] IV FOUND:", iv)
					h.iv, err = hex.DecodeString(iv[2:])
					if err != nil {
						return err
					}
				}
			}
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if getSegment {
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				h.segments = append(h.segments, line)
			} else {
				h.segments = append(h.segments, baseUrl+line)
			}
			getSegment = false
		}
	}
	if e := scanner.Err(); e != nil {
		return e
	}

	return nil
}

func (h *Hlss) downloadSegments() error {
	d := downloader.New(h.downloadWorker, ".", h.downloadCallback)
	d.SetUrls(h.segments)
	d.SetCookies(h.cookies)
	d.SetReferer(h.referer)
	d.StartDownload()

	return nil
}

func (h *Hlss) decryptSegments() error {
	pout, err := os.Create(h.file)
	defer pout.Close()
	if err != nil {
		return err
	}

	n := 0
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
		n++

		if h.decryptCallback != nil {
			h.decryptCallback(name, n, h.GetTotSegments())
		}
	}

	return nil
}

//! Public methods

func (h *Hlss) ExtractVideo() error {
	var err error
	if h.secondaryUrl == "" {
		h.secondaryUrl = h.mainIdx
		if err = h.parseSecondaryIndex(); err != nil {
			return err
		}
	}

	if err = h.downloadSegments(); err != nil {
		return err
	}

	if err = h.decryptSegments(); err != nil {
		return err
	}

	return nil
}

func (h *Hlss) GetResolutions() []string {
	return h.resKeys
}

func (h *Hlss) SetResolution(res_idx int) error {
	if res_idx >= len(h.resKeys) {
		return errors.New("Resolution not found")
	}

	if strings.HasPrefix(h.resolutions[h.resKeys[res_idx]], "http://") || strings.HasPrefix(h.resolutions[h.resKeys[res_idx]], "https://") {
		h.secondaryUrl = h.resolutions[h.resKeys[res_idx]]
	} else {
		h.secondaryUrl = h.baseUrl + h.resolutions[h.resKeys[res_idx]]
	}

	err := h.parseSecondaryIndex()

	return err
}

func (h *Hlss) GetTotSegments() int {
	return len(h.segments)
}

func (h *Hlss) GetBandwidths() []string {
	return h.bandwidthKeys
}

func (h *Hlss) setCookies(cookieFile string) error {
	if cookieFile != "" {
		cookies, err := utils.ParseCookieFile(cookieFile)
		if err != nil {
			return fmt.Errorf("cannot parse cookie file: %s", err)
		}
		h.cookies = cookies
	}
	return nil
}
