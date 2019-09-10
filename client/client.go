package client

import (
	"context"
	"fmt"
	"log"
	"time"

	clientruntime "github.com/go-openapi/runtime/client"
	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/client"
	"github.com/prometheus/alertmanager/api/v2/client/alert"
	"github.com/prometheus/alertmanager/api/v2/client/general"
	"github.com/prometheus/alertmanager/api/v2/models"
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

func getStatus(am string) (*models.AlertmanagerStatus, error) {
	cr := clientruntime.New(am, "/api/v2", []string{"http"})
	c := client.New(cr, strfmt.Default)

	status, err := c.General.GetStatus(general.NewGetStatusParams())
	if err != nil {
		return nil, fmt.Errorf("failed to query client: %s: %s", am, err)
	}
	return status.Payload, nil
}

// Version returns the version of the AlertManager instance.
func Version(am string) (string, error) {
	s, err := getStatus(am)
	if err != nil {
		return "", err
	}
	info := s.VersionInfo
	return fmt.Sprintf("%s (branch: %s, rev: %s)", *info.Version, *info.Branch, *info.Revision), nil
}

// Configuration returns the configuration of the AlertManager instance.
func Configuration(am string) (string, error) {
	s, err := getStatus(am)
	if err != nil {
		return "", err
	}
	return *s.Config.Original, nil
}

// Send sends alerts to the AlertManager instances.
func (a *Sender) Send(ams []string, alerts models.PostableAlerts) error {
	var alertmanagers []*client.Alertmanager
	for _, am := range ams {
		cr := clientruntime.New(am, "/api/v2", []string{"http"})
		alertmanagers = append(alertmanagers, client.New(cr, strfmt.Default))
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
				alertParams := alert.NewPostAlertsParams().WithContext(ctx).WithAlerts(slice[:upper])
				if _, err := am.Alert.PostAlerts(alertParams); err != nil {
					a.l.Println("error sending alerts:", err)
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
