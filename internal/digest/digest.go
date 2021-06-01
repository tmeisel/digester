package digest

import (
	"crypto/md5"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"
)

type Digester struct {
	parallel int
	client   http.Client
}

type job struct {
	idx int
	url string
}

type result struct {
	idx      int
	hash     string
	started  time.Time
	finished time.Time
	err      error
}

type (
	HttpError error
	ByteError error
	HashError error
)

const (
	DefaultTimeout = time.Second * 5
	failedFormat   = "failed: %s"
)

// New returns an initialized *Digester ready to be used
func New(parallel int) *Digester {
	return &Digester{
		parallel: parallel,
		client: http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// SetTimeout modifies the request timeout of the underlying http client
func (d *Digester) SetTimeout(t time.Duration) {
	d.client.Timeout = t
}

// Run spawns as many works as specified for this digester
func (d *Digester) Run(URLs ...string) map[string]string {
	jobs := make(chan job, d.parallel)
	results := make(chan result)

	for i := 0; i < d.parallel; i++ {
		go d.worker(jobs, results)
	}

	for idx, url := range URLs {
		jobs <- job{
			idx: idx,
			url: url,
		}
	}

	close(jobs)

	output := make(map[string]string)

	for {
		select {
		case res := <-results:
			url := URLs[res.idx]
			output[url] = res.hash

			if res.err != nil {
				output[url] = fmt.Sprintf(failedFormat, res.err)
			}
		default:
			if len(output) == len(URLs) {
				return output
			}
		}
	}
}

func (d *Digester) worker(jobs <-chan job, results chan<- result) {
	for job := range jobs {
		result := result{
			idx:     job.idx,
			started: time.Now(),
		}

		result.hash, result.err = d.fetchHash(job.url)
		result.finished = time.Now()

		results <- result
	}
}

func (d *Digester) fetchHash(url string) (string, error) {
	rsp, err := d.client.Get(url)
	if err != nil {
		return "", err
	}

	if rsp.StatusCode >= http.StatusBadRequest {
		return "", newHttpError("remote returned rsp code %d", rsp.StatusCode)
	}

	body, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return "", newByteError("failed to read response body (%s)", err)
	}

	md5Sum := md5.Sum(body)

	return fmt.Sprintf("%x", md5Sum), nil
}

func newHttpError(format string, a ...interface{}) HttpError {
	return HttpError(errors.New(fmt.Sprintf(format, a...)))
}

func newByteError(format string, a ...interface{}) ByteError {
	return ByteError(errors.New(fmt.Sprintf(format, a...)))
}
