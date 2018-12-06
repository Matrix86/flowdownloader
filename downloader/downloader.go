package downloader

import (
	"errors"
	"io"
	"net/http"
	"os"
	"sync"

	"github.com/Matrix86/flowdownloader/utils"
)

type Callback func(filename string, done int, total int)

type downloader struct {
	workers           int
	jobs              chan string
	urls              []string
	path              string
	done              int
	total             int
	wg                sync.WaitGroup
	download_callback Callback
}

//var wg sync.WaitGroup

func (d *downloader) downloadFile(filepath string, url string) error {

	// Create the file
	out, err := os.Create(filepath)
	if err != nil {
		return errors.New(" : downloadFile can't create file : " + err.Error())
	}
	defer out.Close()

	// Get the data
	resp, err := http.Get(url)
	if err != nil {
		return err
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
		d.downloadFile("./"+utils.GetFileFromUrl(j), j)
		d.done++
		if d.download_callback != nil {
			d.download_callback(j, d.done, d.total)
		}
	}
}

func New(workers int, path string, clb Callback) *downloader {
	d := downloader{workers: workers, path: path, download_callback: clb}

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
