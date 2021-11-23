package hlss

import (
	"bufio"
	"encoding/hex"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"path"
	"regexp"
	"strings"

	"github.com/Matrix86/flowdownloader/downloader"
	"github.com/Matrix86/flowdownloader/utils"
	"github.com/evilsocket/islazy/log"
)

var rKeyUrl = regexp.MustCompile(`URI=\"([^\"]+)\"`)

type DecryptCallback func(string, int, int)

type Segment struct {
	URL string
	Key []byte
	IV  []byte
}

type Hlss struct {
	baseUrl          string
	key              []byte
	iv               []byte
	mainIdx          string
	secondaryIdx     []string
	segments         []*Segment
	file             string
	segmentsDir      string
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

	if referer == "" {
		obj.referer = utils.GetBaseUrl(mainUrl)
	}

	if cookieFile != "" {
		log.Debug("parsing cookies")
		err := obj.setCookies(cookieFile)
		if err != nil {
			return nil, err
		}
	}

	// Try to get key from URL
	if keyUrl != "" {
		var err error
		obj.key, err = obj.retrieveKeyFromURL(keyUrl)
		if err != nil {
			return nil, err
		}
	}

	obj.resolutions = make(map[string]string)
	if err := obj.parseMainIndex(); err != nil {
		return nil, err
	}

	obj.baseUrl = utils.GetBaseUrl(mainUrl)
	log.Debug("base url: '%s'", obj.baseUrl)

	return &obj, nil
}

func (h *Hlss) retrieveKeyFromURL(keyUrl string) ([]byte, error) {
	log.Debug("getting key from url: '%s'", keyUrl)
	resp, err := utils.HttpRequest("GET", keyUrl, h.cookies, h.referer)
	if err != nil {
		return nil, fmt.Errorf("http request error: %s", err)
	}
	defer resp.Body.Close()
	buf, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	log.Debug("key lenght: %d", len(buf))
	return buf, nil
}

func (h *Hlss) parseMainIndex() error {
	resp, err := utils.HttpRequest("GET", h.mainIdx, h.cookies, h.referer)
	if err != nil {
		return fmt.Errorf("http request error: %s", err)
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
	resp, err := utils.HttpRequest("GET", h.secondaryUrl, h.cookies, h.referer)
	if err != nil {
		return fmt.Errorf("http request error: %s", err)
	}
	defer resp.Body.Close()

	scanner := bufio.NewScanner(resp.Body)
	scanner.Split(bufio.ScanLines)

	baseUrl := utils.GetBaseUrl(h.secondaryUrl)
	baseURL, _ := url.Parse(baseUrl)

	firstLine := true
	getSegment := false
	//keyUrl := ""
	var key []byte
	var iv []byte
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
			//if len(params) < 2 {
			//	return errors.New("invalid m3u file format")
			//}
			for _, info := range params {
				if key == nil && strings.HasPrefix(info, "URI=\"") {
					match := rKeyUrl.FindStringSubmatch(info)
					if match != nil && len(match) >= 2 {
						log.Debug("key URL found: %s", match[1])
						key, err = h.retrieveKeyFromURL(match[1])
						if err != nil {
							log.Error("retrieving Key from URL: %s", err)
						}
					}
				} else if strings.HasPrefix(info, "IV=") {
					strIV := info[len("IV="):]
					log.Debug("IV found: %s", strIV)
					iv, err = hex.DecodeString(strIV[2:])
					if err != nil {
						return err
					}
				} else if strings.HasPrefix(info, "METHOD=NONE") {
					key = nil
					iv = nil
				}
			}
		} else if strings.HasPrefix(line, "#EXT-X-DISCONTINUITY") {
			key = nil
			iv = nil
		} else if strings.HasPrefix(line, "#") || line == "" {
			continue
		} else if getSegment {
			segment := &Segment{
				IV: iv,
				Key: key,
			}
			if strings.HasPrefix(line, "http://") || strings.HasPrefix(line, "https://") {
				segment.URL = line
				h.segments = append(h.segments, segment)
			} else {
				secondary, _ := baseURL.Parse(line)
				segment.URL = secondary.String()
				h.segments = append(h.segments, segment)
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
	log.Debug("downloading segments")
	parentDir := os.TempDir()
	segmentsDir, err := ioutil.TempDir(parentDir, "*-segments")
	if err != nil {
		return err
	}
	h.segmentsDir = segmentsDir
	d := downloader.New(h.downloadWorker, segmentsDir, h.downloadCallback)
	urls := make([]string, 0)
	for _, s := range h.segments {
		urls = append(urls, s.URL)
	}
	d.SetUrls(urls)
	d.SetCookies(h.cookies)
	d.SetReferer(h.referer)

	return d.StartDownload()
}

func (h *Hlss) decryptSegments() error {
	log.Debug("decrypting segments")
	pout, err := os.Create(h.file)
	defer pout.Close()
	if err != nil {
		return err
	}

	if len(h.iv) == 0 {
		log.Debug("empty IV...")
		h.iv = make([]byte, len(h.key))
	}

	n := 0
	for _, segment := range h.segments {
		name := utils.GetMD5Hash(segment.URL)
		fpath := path.Join(h.segmentsDir, name)

		if len(segment.Key) != 0 {
			if err = utils.DecryptFileAppend(pout, fpath, segment.Key, segment.IV); err != nil {
				return err
			}
		} else {
			// we assume that the segments are not encrypted
			if err = utils.FileAppend(pout, fpath); err != nil {
				return err
			}
		}

		os.Remove(fpath)
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
		baseURL, _ := url.Parse(h.baseUrl)
		secondary, _ := baseURL.Parse(h.resolutions[h.resKeys[res_idx]])
		h.secondaryUrl = secondary.String()
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
