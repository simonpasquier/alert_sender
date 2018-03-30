// Copyright 2018 Simon Pasquier
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
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/cli"

	"github.com/simonpasquier/alert_sender/client"
)

var (
	help             bool
	ams              string
	lbls, anns       string
	repeatInterval   string
	startsAt, endsAt string
	runs, num        int
	batch            int
	re               = regexp.MustCompile(`\s*(\w+)=(?:\"?([^",]+)\"?)\s*(?:,|$)`)
)

func init() {
	flag.BoolVar(&help, "help", false, "Help message")
	flag.StringVar(&ams, "addresses", "", "Comma-separated list of AlertManager servers.")
	flag.StringVar(&lbls, "labels", "AlertName=HighLatency,service=my-service,instance=instance-{{i}}", "Comma-separated list of alert's labels.")
	flag.StringVar(&anns, "annotations", "Summary=\"High Latency\",Description=\"Latency is high!\"", "Comma-separated list of alert's annotations.")
	flag.IntVar(&runs, "runs", 1, "Total number of runs.")
	flag.IntVar(&num, "num", 1, "Total number of alerts to be sent at every run.")
	flag.IntVar(&batch, "batch", 1, "How many alerts to send per batch.")
	flag.StringVar(&repeatInterval, "repeat-interval", "10s", "Interval before sending the alerts again.")
	flag.StringVar(&startsAt, "start", "now", "Start time of the alerts (RFC3339 format).")
	flag.StringVar(&endsAt, "end", "", "End time of the alerts (RFC3339 format). If empty, the end time isn't set. It can be a duration relative to the start time or 'now'.")
}

func buildAlertSlice(n int, lbls, anns string, start, end time.Time) []cli.Alert {
	builder := client.NewBuilder("xxx")
	alerts := make([]cli.Alert, n)

	labels, annotations := map[string]string{}, map[string]string{}
	for _, s := range re.FindAllStringSubmatch(lbls, -1) {
		labels[s[1]] = s[2]
	}
	for _, s := range re.FindAllStringSubmatch(anns, -1) {
		annotations[s[1]] = s[2]
	}

	expand := func(i int, m map[string]string) map[string]string {
		set := map[string]string{}
		for k, v := range m {
			k = strings.Replace(k, "{{i}}", fmt.Sprintf("%d", i), -1)
			v = strings.Replace(v, "{{i}}", fmt.Sprintf("%d", i), -1)
			set[k] = v
		}
		return set
	}

	for i := range alerts {
		alerts[i] = builder.CreateAlert(expand(i, labels), expand(i, annotations), start, end)
	}

	return alerts
}

func main() {
	l := log.New(os.Stderr, "", log.Ltime|log.Lshortfile)
	flag.Parse()
	if help || ams == "" {
		l.Println("sender: send alerts to AlertManager.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	if batch <= 0 || num <= 0 {
		l.Fatal("Invalid option")
	}

	repeat, err := time.ParseDuration(repeatInterval)
	if err != nil {
		l.Fatal("Cannot parse interval:", err)
	}

	var start, end time.Time
	if start, err = time.Parse(time.RFC3339, startsAt); err != nil {
		start = time.Now()
	}
	if end, err = time.Parse(time.RFC3339, endsAt); err != nil {
		if d, err := time.ParseDuration(endsAt); err == nil {
			end = start.Add(d)
		} else if endsAt == "now" {
			end = time.Now()
		}
	}
	alerts := buildAlertSlice(num, lbls, anns, start, end)

	c := client.NewSender(runs, batch, repeat, l)
	if err := c.Send(strings.Split(ams, ","), alerts); err != nil {
		l.Fatal(err)
	}
}
