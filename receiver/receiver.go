package receiver

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/prometheus/alertmanager/cli"
)

// Notification represents an AlertManager notification.
type Notification struct {
	Timestamp    time.Time         `json:"timestamp"`
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
	nf  []Notification
}

// NewWebhook returns a webhook receiver.
func NewWebhook(l *log.Logger) *Webhook {
	return &Webhook{
		srv: &http.Server{},
		l:   l,
	}
}

func (w *Webhook) serve(_ http.ResponseWriter, r *http.Request) {
	b, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.l.Printf("fail to ready body: %s", err)
		return
	}

	var nf Notification
	err = json.Unmarshal(b, &nf)
	if err != nil {
		w.l.Printf("fail to decode body: %s", err)
	}
	nf.Timestamp = time.Now().UTC()
	w.nf = append(w.nf, nf)
	w.l.Printf("msg=%q gkey=%q status=%q", "received notification", nf.GroupKey, nf.Status)
}

// Run runs the webhook receiver.
func (w *Webhook) Run(addr string) error {
	http.HandleFunc("/", w.serve)

	ln, err := net.Listen("tcp", addr)
	if err != nil {
		w.l.Fatal(err)
	}

	return w.srv.Serve(ln)
}

// Stop stops the webhook receiver.
func (w *Webhook) Stop() {
	w.l.Printf("msg=%q", "Stopping the webhook receiver")
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(5*time.Second))
	w.srv.Shutdown(ctx)
	cancel()
}

// Notifications returns the notifications received from AlertManager.
func (w *Webhook) Notifications() []Notification {
	return w.nf
}
