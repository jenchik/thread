# thread
Asynchonous goroutine pool for Golang

[![Build Status](https://travis-ci.org/jenchik/thread.svg)](https://travis-ci.org/jenchik/thread)

Installation
------------

```bash
go get github.com/jenchik/thread
```

Example
-------
```go
package main

import (
	"fmt"
	"net/http"
	"time"

	"github.com/jenchik/thread"
)

func sendRequest(url string) (*http.Response, error) {
	client := http.Client{
		Timeout: time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}

	return res, nil
}

func worker(p *thread.Funcer, url string) {
	f := func() {
		response, err := sendRequest(url)
		if err != nil {
			fmt.Println("Error request:", err)
			return
		}

		if response.Body == nil {
			fmt.Println("Error empty body")
			return
		}
		defer response.Body.Close()

		fmt.Printf("Request to '%s': %s\n", url, response.Status)

		// ...
	}

	p.Put(f)
}

func main() {
	start := time.Now()

	fpool := thread.NewFuncer(func(c <-chan func()) {
		fmt.Println("Uncompleted requests", len(c))
		for f := range c {
			f()
		}
	}, 4)

	urlMask := "http://example.com/page/%d"
	for i := 0; i < 5; i++ {
		worker(fpool, fmt.Sprintf(urlMask, i))
	}

	// ... other work
	time.Sleep(time.Millisecond * 500)

	fpool.Close()
	fpool.Wait()

	fmt.Println("Duration", time.Since(start).String())
}
```
