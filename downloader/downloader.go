package downloader

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"sync"

	"github.com/Matrix86/flowdownloader/utils"
	"github.com/evilsocket/islazy/log"
)

type Callback func(filename string, done int, total int)

type downloader struct {
	workers          int
	jobs             chan string
	urls             []string
	path             string
	done             int
	total            int
	wg               sync.WaitGroup
	downloadCallback Callback
	cookies          []*http.Cookie
	referer          string
}

//var wg sync.WaitGroup

func (d *downloader) downloadFile(filepath string, url string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return errors.New("downloadFile can't create file : " + err.Error())
	}
	defer out.Close()

	// Get the data
	client := &http.Client{}
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}
	if len(d.cookies) > 0 {
		for _, c := range d.cookies {
			req.AddCookie(c)
		}
	}
	if d.referer != "" {
		req.Header.Set("Referer", d.referer)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	} else if resp.StatusCode != 200 {
		return fmt.Errorf("http response status: %d %s", resp.StatusCode, http.StatusText(resp.StatusCode))
	}
	defer resp.Body.Close()

	// Write the body to file
	_, err = io.Copy(out, resp.Body)
	if err != nil {
		return err
	}

	return nil
}

func (d *downloader) worker(id int, jobs <-chan string) {
	defer d.wg.Done()

	for j := range jobs {
		log.Debug("download segment from '%s'", j)
		fpath := path.Join(d.path, utils.GetMD5Hash(j))
		err := d.downloadFile(fpath, j)
		if err != nil {
			log.Error("during segment download: segment='%s': %s", j, err)
		}
		d.done++
		if d.downloadCallback != nil {
			d.downloadCallback(j, d.done, d.total)
		}
	}
}

func New(workers int, path string, clb Callback) *downloader {
	d := downloader{workers: workers, path: path, downloadCallback: clb}

	d.jobs = make(chan string, 100)
	for w := 1; w <= workers; w++ {
		d.wg.Add(1)
		go d.worker(w, d.jobs)
	}

	return &d
}

func (d *downloader) SetUrls(urls []string) {
	d.urls = urls
	d.total = len(urls)
}

func (d *downloader) SetCookies(cookies []*http.Cookie) {
	d.cookies = cookies
}

func (d *downloader) SetReferer(referer string) {
	d.referer = referer
}

func (d *downloader) StartDownload() error {
	go func() {
		for _, url := range d.urls {
			d.jobs <- url
		}
		close(d.jobs)
	}()

	done := make(chan struct{})
	d.wg.Wait()
	close(done)

	return nil
}
