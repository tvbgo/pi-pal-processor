// This code was derived from https://github.com/GoogleCloudPlatform/pi-delivery
// Authored by Emma Haruka Iwao
// Changes were made only to encompass the specific processing needs
// of the challenge.

//Copyright 2022 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sync"
	"time"
	"strconv"

	"github.com/googlecloudplatform/pi-delivery/gen/index"
	"github.com/googlecloudplatform/pi-delivery/pkg/obj"
	"github.com/googlecloudplatform/pi-delivery/pkg/obj/gcs"
	"github.com/googlecloudplatform/pi-delivery/pkg/unpack"
	"github.com/sethvargo/go-retry"
	"go.uber.org/zap"
)

const (
	CHUNK_SIZE        = 100_000_000
	WORKERS           = 150
)

var logger *zap.SugaredLogger
var wg sync.WaitGroup

type workerContextKey string

type task struct {
	start  int64
	n      int32
	cancel context.CancelFunc
	unp 	 string
	offset int
	out    io.Writer
	id 		 int64
}

func process(ctx context.Context, task *task, logger *zap.SugaredLogger, client obj.Client) error {
	logger.Infof("processing task, start = %d, n = %v", task.start, task.n)

	rrd := index.Decimal.NewReader(ctx, client.Bucket(index.BucketName))
	defer rrd.Close()
	urd := unpack.NewReader(ctx, rrd)
	if _, err := urd.Seek(task.start, io.SeekStart); err != nil {
		return err
	}
	var buf bytes.Buffer

	if _, err := io.CopyN(&buf, urd, CHUNK_SIZE); err != nil {
		fmt.Fprintf(os.Stderr, "I/O error: %v\n", err)
		os.Exit(1)
	}

	task.unp = buf.String()

	outfile := fmt.Sprintf("full_results/batch-%d.txt", task.id)
	f, err := os.OpenFile(outfile, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "couldn't open %s: %v", outfile, err)
		os.Exit(1)
	}
	defer f.Close()
	task.out = f

	FindCandidates(task)


	logger.Infof("digits processed: %d + %d digits",
		task.start, task.n)
	return nil
}

func FindCandidates(b *task) {
  s := b.unp
  ps := b.offset
  for i := range s {
    if i >= (len(s)-1)-ps { break }
    if i-ps < 0 { continue }
    pLen := ps

    for i-pLen >= 0 && i+pLen < len(s) && s[i-pLen] == s[i+pLen] {
      pLen++
    }

    if i - pLen > 0 && pLen > 8  {
        pal := s[i-pLen+1 : i+pLen]
        index := toString(i)
        fmt.Fprintf(b.out, "%d, %s, %s, %d \n", b.start, index, pal, len(pal))
      }
  }
}

func toString(i int) string {
	var s interface{} = i
	if _, ok := s.(int64); ok {
		return strconv.FormatInt(int64(i), 10)
	} else {
		return strconv.Itoa(i)
	}
}

func worker(ctx context.Context, taskChan <-chan task, client obj.Client) {
	defer wg.Done()
	logger := logger.With("worker id", ctx.Value(workerContextKey("workerId")))
	defer logger.Sync()
	defer logger.Infow("worker exiting")

	logger.Info("worker started")
	b := retry.WithMaxRetries(3, retry.NewExponential(1*time.Second))
	for task := range taskChan {
		select {
		case <-ctx.Done():
			return
		default:
		}

		if err := retry.Do(ctx, b, func(ctx context.Context) error {
			if err := process(ctx, &task, logger, client); err != nil {
				return retry.RetryableError(err)
			}
			return nil
		}); err != nil {
			logger.Errorw("process failed", "error", err)
			task.cancel()
		}
	}

}

func main() {
	l, _ := zap.NewDevelopment()
	defer l.Sync()
	zap.ReplaceGlobals(l)
	logger = l.Sugar()

	start := flag.Int64("s", 0, "Start offset")
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	client, err := gcs.NewClient(ctx)
	if err != nil {
		logger.Errorf("couldn't create a GCS client: %v", err)
		os.Exit(1)
	}
	defer client.Close()

	taskChan := make(chan task, 150)

	for i := 0; i < WORKERS; i++ {
		wg.Add(1)
		ctx = context.WithValue(ctx, workerContextKey("workerId"), i)
		go worker(ctx, taskChan, client)
	}

	for i := *start; i < index.Decimal.TotalDigits(); i += CHUNK_SIZE {
		var s int64
		if i == 0 {
			s = i
		} else {
			s = i - 1000
		}

		task := task{
			start:  s,
			n:      CHUNK_SIZE,
			cancel: cancel,
			offset: 1,
			id:     i,
		}
		taskChan <- task
		if ctx.Err() != nil {
			logger.Errorf("context error: %v", ctx.Err())
			break
		}
	}
	close(taskChan)
	wg.Wait()
}
