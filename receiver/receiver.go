package receiver

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/alertmanager/cli"
)

type notification struct {
	GroupKey     string            `json:"groupKey"`
	Receiver     string            `json:"receiver"`
	Status       string            `json:"status"`
	Alerts       []cli.Alert       `json:"alerts"`
	GroupLabels  map[string]string `json:"groupLabels"`
	CommonLabels map[string]string `json:"commonLabels"`
	CommonAnns   map[string]string `json:"commonAnnotations"`
}

// Webhook proceses and stores notifications from AlertManager.
type Webhook struct {
	srv *http.Server
	l   *log.Logger
	nf  []notification
}

// NewWebhook returns a webhook receiver.
func NewWebhook(addr string, l *log.Logger) *Webhook {
	return &Webhook{
		srv: &http.Server{Addr: addr},
		l:   l,
	}
}

func (w *Webhook) serve(_ http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.l.Printf("fail to ready body: %s", err)
		return
	}

	var nf notification
	err = json.Unmarshal(b, &nf)
	if err != nil {
		w.l.Printf("fail to decode body: %s", err)
	}
	w.nf = append(w.nf, nf)
	w.l.Printf("gkey=%q status=%q", nf.GroupKey, nf.Status)
}

// Run runs the webhook receiver.
func (w *Webhook) Run() error {
	http.HandleFunc("/", w.serve)
	return w.srv.ListenAndServe()
}

// Stop stops the webhook receiver.
func (w *Webhook) Stop() {
	w.l.Println("Stopping the webhook receiver")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
	w.srv.Shutdown(ctx)
	cancel()
}

// Print logs the notifications received from AlertManager.
func (w *Webhook) Print() {
	for _, nf := range w.nf {
		w.l.Printf("%v", nf)
	}
}
