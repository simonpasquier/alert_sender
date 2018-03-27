// Copyright 2015 The Prometheus Authors
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
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
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/prometheus/common/model"
)

var (
	help          bool
	ams           string
	lbls, anns    string
	num, interval int
	batch         int
	re            = regexp.MustCompile(`\s*(\w+)=(?:\"?([^",]+)\"?)\s*(?:,|$)`)
)

func init() {
	flag.BoolVar(&help, "help", false, "Help message")
	flag.StringVar(&ams, "addresses", "", "Comma-separated list of AlertManager servers.")
	flag.StringVar(&lbls, "labels", "AlertName=HighLatency,service=my-service,instance=instance-{{i}}", "Comma-separated list of alert's labels.")
	flag.StringVar(&anns, "annotations", "Summary=\"High Latency\",Description=\"Latency is high!\"", "Comma-separated list of alert's annotations.")
	flag.IntVar(&num, "num", 100, "Number of alerts to be sent.")
	flag.IntVar(&batch, "batch", 10, "Batch size when sending alerts.")
	flag.IntVar(&interval, "interval", 10, "Interval between alert sending.")
}

func sendAlerts(ctx context.Context, ams []string, alerts ...*model.Alert) error {
	var wg sync.WaitGroup

	b, err := json.Marshal(alerts)
	if err != nil {
		return err
	}

	errs := make(chan error, len(ams))
	for _, am := range ams {
		wg.Add(1)
		go func(am string) {
			defer wg.Done()
			client := &http.Client{Timeout: time.Duration(5 * time.Second)}
			req, err := http.NewRequest("POST", "http://"+am+"/api/v1/alerts", bytes.NewReader(b))
			req.WithContext(ctx)
			if err != nil {
				return
			}
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				errs <- err
				return
			}
			defer resp.Body.Close()

			// Any HTTP status 2xx is OK.
			if resp.StatusCode/100 != 2 {
				errs <- fmt.Errorf("bad response status %v", resp.Status)
				return
			}
		}(am)
	}
	go func() {
		wg.Wait()
		close(errs)
	}()
	err, ok := <-errs
	if ok {
		return err
	}
	return nil
}

func buildAlertSlice(n int, lbls, anns string, start, end time.Time) []*model.Alert {
	alerts := make([]*model.Alert, n)

	expand := func(i int, m map[string]string) model.LabelSet {
		lblset := model.LabelSet{}
		for k, v := range m {
			k = strings.Replace(k, "{{i}}", fmt.Sprintf("%d", i), -1)
			v = strings.Replace(v, "{{i}}", fmt.Sprintf("%d", i), -1)
			lblset[model.LabelName(k)] = model.LabelValue(v)
		}
		return lblset
	}

	labels, annotations := map[string]string{}, map[string]string{}
	for _, s := range re.FindAllStringSubmatch(lbls, -1) {
		labels[s[1]] = s[2]
	}
	for _, s := range re.FindAllStringSubmatch(anns, -1) {
		annotations[s[1]] = s[2]
	}

	for i := range alerts {
		alerts[i] = &model.Alert{
			Labels:      expand(i, labels),
			Annotations: expand(i, annotations),
			StartsAt:    start,
			EndsAt:      end,
		}
	}

	return alerts
}

func main() {
	flag.Parse()
	if help || ams == "" {
		log.Println("send_alerts: fire alerts to AlertManager and then resolve them.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if batch <= 0 || num <= 0 {
		log.Fatal("Invalid option")
	}

	alertmanagers := strings.Split(ams, ",")
	alerts := buildAlertSlice(num, lbls, anns, time.Now(), time.Time{})

	for {
		log.Println("sending alerts...")
		slice := alerts[:]
		for {
			if len(slice) == 0 {
				break
			}

			upper := batch
			if len(slice) < batch {
				upper = len(slice)
			}

			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			err := sendAlerts(ctx, alertmanagers, slice[:upper]...)
			if err != nil {
				log.Println("error sending alerts:", err)
			}
			cancel()
			slice = slice[upper:]
		}

		select {
		case <-time.After(time.Duration(interval) * time.Second):
		}
	}
}
