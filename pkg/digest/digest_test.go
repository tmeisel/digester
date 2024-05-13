package digest

import (
	"bytes"
	"crypto/md5"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"testing"
	"time"
	// usually I use testify assert and require,
	// but due to the requirements of the task I
	// went with std packages only
)

func TestNew(t *testing.T) {
	// a simple int would be sufficient, but that's how I
	// usually structure tests to make them extendable
	type testCase struct {
		parallel int
	}

	for name, tc := range map[string]testCase{
		"1 parallel": {parallel: 1},
		"3 parallel": {parallel: 3},
	} {
		t.Run(name, func(t *testing.T) {
			d := New(tc.parallel)
			if d == nil {
				t.FailNow()
			}

			if d.parallel != tc.parallel {
				t.Errorf("failed asserting specified parallelism of %d (got %d)", tc.parallel, d.parallel)
			}

			if d.client.Timeout != DefaultTimeout {
				t.Errorf("http client does not seem to be initialized as expected")
			}
		})
	}

}

// TestDigester_SetTimeout tests overwriting the request
// timeout of the http.Client
func TestDigester_SetTimeout(t *testing.T) {
	const newTimeOut = DefaultTimeout * 2

	d := New(1)
	d.SetTimeout(newTimeOut)

	if d.client.Timeout != newTimeOut {
		t.Errorf("failed asserting overwritten default timeout")
	}
}

func TestDigester_Run(t *testing.T) {
	type testCase struct {
		parallel int
		total    int
		rtt      int64
		deadline time.Duration
	}

	urlPool := []string{"adjust.com", "google.com", "yahoo.com", "bing.com"}

	body, err := createRandomBytes(32)
	if err != nil {
		t.Fatalf("failed to create random body")
	}

	expectedHash := fmt.Sprintf("%x", md5.Sum(body))

	for name, tc := range map[string]testCase{
		"single request, no delay": {
			parallel: 1,
			total:    1,
			rtt:      0,
			deadline: time.Millisecond,
		},
		"queued": {
			parallel: 2,
			total:    4,
			rtt:      int64(time.Millisecond),
			deadline: time.Millisecond * 3,
		},
		"parallel": {
			parallel: 4,
			total:    4,
			rtt:      int64(time.Millisecond),
			deadline: time.Millisecond * 2,
		},
		"less urls than workers": {
			parallel: 4,
			total:    2,
			rtt:      0,
			deadline: time.Millisecond,
		},
	} {
		t.Run(name, func(t *testing.T) {
			if tc.total > len(urlPool) {
				// reduce tc.total or add urls to the pool
				t.Errorf("invalid number of urls specified for the test case")
				t.FailNow()
			}

			urls := urlPool[:tc.total]

			var delay *time.Duration
			if tc.rtt > 0 {
				rtt := time.Duration(tc.rtt)
				delay = &rtt
			}

			d := New(tc.parallel)
			d.client = mockHttpSuccessClient(t, body, delay)

			started := time.Now()
			result := d.Run(urls...)

			if time.Now().Sub(started) > tc.deadline {
				t.Errorf("deadline of %s exceeded", tc.deadline)
				t.FailNow()
			}

			for _, url := range urls {
				hash, ok := result[url]

				if !ok {
					t.Errorf("no result for %s", url)
					t.FailNow()
				}

				if hash != expectedHash {
					t.Errorf("unexpected hash %s", hash)
					t.FailNow()
				}

				t.Logf("%s: %s", url, hash)
			}
		})
	}
}

func mockHttpSuccessClient(_ *testing.T, body []byte, rtt *time.Duration) http.Client {
	return mockHttpClient(func(r *http.Request) (response *http.Response, err error) {
		if rtt != nil {
			time.Sleep(*rtt)
		}

		return &http.Response{
			StatusCode:    http.StatusOK,
			Body:          io.NopCloser(bytes.NewBuffer(body)),
			ContentLength: int64(len(body)),
		}, nil
	})
}

// getHttpClient returns an http.Client that calls resp on every request
// and returns the functions output
func mockHttpClient(resp func(r *http.Request) (*http.Response, error)) http.Client {
	return http.Client{Transport: &roundTripMock{
		responseFn: resp,
	}}
}

func createRandomBytes(n int) ([]byte, error) {
	buffer := make([]byte, n)

	if _, err := rand.Read(buffer); err != nil {
		return nil, err
	}

	return buffer, nil
}

type roundTripMock struct {
	responseFn func(r *http.Request) (*http.Response, error)
}

func (rt *roundTripMock) RoundTrip(r *http.Request) (*http.Response, error) {
	return rt.responseFn(r)
}
