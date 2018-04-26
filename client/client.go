package client

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/prometheus/alertmanager/client"
	"github.com/prometheus/client_golang/api"
)

// Sender is used to send alerts on a regular basis.
type Sender struct {
	runs     int
	batch    int
	interval time.Duration
	l        *log.Logger
}

// NewSender returns a Sender instance.
func NewSender(runs, batch int, interval time.Duration, l *log.Logger) *Sender {
	return &Sender{
		runs:     runs,
		batch:    batch,
		interval: interval,
		l:        l,
	}
}

func getStatus(am string) (*client.ServerStatus, error) {
	c, err := api.NewClient(api.Config{
		Address: fmt.Sprintf("http://%s", am),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create client: %s: %s", am, err)
	}
	status := client.NewStatusAPI(c)
	ctx := context.Background()
	s, err := status.Get(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to query client: %s: %s", am, err)
	}
	return s, nil
}

// Version returns the version of the AlertManager instance.
func Version(am string) (string, error) {
	s, err := getStatus(am)
	if err != nil {
		return "", err
	}
	info := s.VersionInfo
	return fmt.Sprintf("%s (branch: %s, rev: %s)", info["version"], info["branch"], info["revision"]), nil
}

// Configuration returns the configuration of the AlertManager instance.
func Configuration(am string) (string, error) {
	s, err := getStatus(am)
	if err != nil {
		return "", err
	}
	return s.ConfigYAML, nil
}

// Send sends alerts to the AlertManager instances.
func (a *Sender) Send(ams []string, alerts []client.Alert) error {
	var alertmanagers []api.Client
	for _, am := range ams {
		c, err := api.NewClient(api.Config{
			Address: fmt.Sprintf("http://%s", am),
		})
		if err != nil {
			return fmt.Errorf("failed to create client: %s: %s", am, err)
		}
		alertmanagers = append(alertmanagers, c)
	}

	sleep := time.NewTimer(0)
	for i := a.runs; i > 0; i-- {
		sleep.Reset(a.interval)

		a.l.Printf("msg=%q", fmt.Sprintf("sending %d alert(s)", len(alerts)))
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
				alertAPI := client.NewAlertAPI(am)
				if err := alertAPI.Push(ctx, slice[:upper]...); err != nil {
					a.l.Println("error sending alerts:", err)
				}
				select {
				case <-ctx.Done():
					a.l.Println("context time-out")
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
