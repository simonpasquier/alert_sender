package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/alertmanager/cli"
	"github.com/prometheus/client_golang/api"
)

// Alerter is used to send alerts on a regular basis.
type Alerter struct {
	runs     int
	batch    int
	interval time.Duration
	l        *log.Logger
}

// NewAlerter returns an Alerter struct.
func NewAlerter(runs, batch int, interval time.Duration, l *log.Logger) *Alerter {
	return &Alerter{
		runs:     runs,
		batch:    batch,
		interval: interval,
		l:        l,
	}
}

// Send sends alerts to the AlertManager instances.
func (a *Alerter) Send(ams []string, alerts []cli.Alert) error {
	var alertmanagers []api.Client
	for _, am := range ams {
		client, err := api.NewClient(api.Config{
			Address: fmt.Sprintf("http://%s", am),
		})
		if err != nil {
			return fmt.Errorf("failed to create client: %s: %s", am, err)
		}
		alertmanagers = append(alertmanagers, client)
	}

	sleep := time.NewTimer(0)
	for i := a.runs; i > 0; i-- {
		sleep.Reset(a.interval)

		a.l.Printf("sending %d alert(s)...\n", len(alerts))
		slice := alerts[:]
		for {
			if len(slice) == 0 {
				break
			}

			upper := a.batch
			if len(slice) <= a.batch {
				upper = len(slice)
			}

			ctx, cancel := context.WithTimeout(context.Background(), a.interval)
			for _, am := range alertmanagers {
				alertAPI := cli.NewAlertAPI(am)
				if err := alertAPI.Push(ctx, slice[:upper]...); err != nil {
					log.Println("error sending alerts:", err)
				}
				select {
				case <-ctx.Done():
					log.Println("context time-out")
					continue
				default:
				}
			}

			cancel()
			slice = slice[upper:]
		}

		select {
		case <-sleep.C:
		}
	}

	return nil
}
