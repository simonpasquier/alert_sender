package client

import (
	"time"

	"github.com/prometheus/alertmanager/cli"
)

// Builder generates full-fledged alerts.
type Builder struct {
	generator string
}

// NewBuilder returns a Builder instance.
func NewBuilder() *Builder {
	return &Builder{generator: "http://example.com/"}
}

// CreateAlert returns a single alert.
func (b *Builder) CreateAlert(lbls, anns map[string]string, start, end time.Time) cli.Alert {
	alert := cli.Alert{
		Labels:       cli.LabelSet{},
		Annotations:  cli.LabelSet{},
		StartsAt:     start,
		EndsAt:       end,
		GeneratorURL: b.generator,
	}

	for k, v := range lbls {
		alert.Labels[cli.LabelName(k)] = cli.LabelValue(v)
	}
	for k, v := range anns {
		alert.Annotations[cli.LabelName(k)] = cli.LabelValue(v)
	}

	return alert
}
