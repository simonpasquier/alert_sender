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
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/prometheus/alertmanager/cli"
	"gopkg.in/yaml.v2"

	"github.com/simonpasquier/alert_sender/client"
	"github.com/simonpasquier/alert_sender/receiver"
)

var (
	help     bool
	ams      string
	planFile string
	listen   string
)

const (
	statusFiring   = "firing"
	statusResolved = "resolved"
)

func init() {
	flag.BoolVar(&help, "help", false, "Help message")
	flag.StringVar(&ams, "addresses", "", "Comma-separated list of AlertManager servers.")
	flag.StringVar(&planFile, "plan", "", "Plan file (YAML format).")
	flag.StringVar(&listen, "listen", ":8080", "Listen address for the webhook receiver.")
}

type template struct {
	Labels      map[string]string `yaml:"labels"`
	Annotations map[string]string `yaml:"annotations"`
	StartsAt    time.Time
	EndsAt      time.Time
}

type alert struct {
	Ref    string `yaml:"ref"`
	Status string `yaml:"status"`
}

type step struct {
	Description string        `yaml:"description"`
	Runs        int           `yaml:"runs"`
	Repeat      time.Duration `yaml:"repeat"`
	Alerts      []alert       `yaml:"alerts"`
}

type plan struct {
	Templates map[string]*template `yaml:"templates"`
	Steps     []step               `yaml:"steps"`
}

func (a *alert) UnmarshalYAML(unmarshal func(interface{}) error) error {
	type plain alert
	if err := unmarshal((*plain)(a)); err != nil {
		return err
	}
	if a.Status == "" {
		a.Status = statusFiring
	}
	if a.Status != statusFiring && a.Status != statusResolved {
		return fmt.Errorf("invalid alert status: %s", a.Status)
	}
	return nil
}

func main() {
	l := log.New(os.Stderr, "", log.Ltime|log.Lshortfile)
	flag.Parse()
	if help || ams == "" {
		l.Println("runner: send alerts to AlertManager.")
		flag.PrintDefaults()
		os.Exit(0)
	}

	b, err := ioutil.ReadFile(planFile)
	if err != nil {
		l.Fatalf("fail to read plan: %s", err)
	}
	var p plan
	err = yaml.UnmarshalStrict(b, &p)
	if err != nil {
		l.Fatalf("fail to parse plan: %s", err)
	}

	alertmanagers := strings.Split(ams, ",")
	for _, am := range alertmanagers {
		version, err := client.Version(am)
		if err != nil {
			l.Fatalf("fail to query alertmanager: %s", err)
		}
		l.Printf("address=%q version=%q", am, version)
	}

	builder := client.NewBuilder()
	wh := receiver.NewWebhook(l)
	go wh.Run(listen)

	for _, s := range p.Steps {
		alerts := make([]cli.Alert, len(s.Alerts))
		for i, a := range s.Alerts {
			t, ok := p.Templates[a.Ref]
			if !ok {
				l.Printf("msg=%q", fmt.Sprintf("cannot find reference: %s", a.Ref))
				continue
			}
			if t.StartsAt.IsZero() && a.Status == statusFiring {
				t.StartsAt = time.Now()
			}
			if t.EndsAt.IsZero() && a.Status == statusResolved {
				t.EndsAt = time.Now()
			}
			alerts[i] = builder.CreateAlert(t.Labels, t.Annotations, t.StartsAt, t.EndsAt)
		}

		l.Printf("msg=%q desc=%q", "running step", s.Description)
		c := client.NewSender(s.Runs, len(alerts), s.Repeat, l)
		if err := c.Send(alertmanagers, alerts); err != nil {
			l.Fatal(err)
		}
	}
	l.Printf("msg=%q", "sleeping for 10s")
	time.Sleep(10 * time.Second)
	wh.Stop()

	now := time.Now().UTC()
	fname := filepath.Base(planFile)
	fname = strings.TrimSuffix(fname, filepath.Ext(fname))
	fname = fmt.Sprintf("notifications-%s-%s", fname, now.Format("20060102-150405"))
	f, err := os.Create(fname)
	if err != nil {
		log.Fatalf("cannot create %s", fname)
	}
	defer f.Close()
	wr := bufio.NewWriter(f)
	defer wr.Flush()
	for _, nf := range wh.Notifications() {
		fmt.Fprintf(wr, "ts=%q status=%q gkey=%q nb_alerts=%d\n", nf.Timestamp, nf.Status, nf.GroupKey, len(nf.Alerts))
		for _, a := range nf.Alerts {
			fmt.Fprintf(wr, "\tstart=%q end=%q labels=%q\n", a.StartsAt, a.EndsAt, a.Labels)
		}
	}
}
