package client

import (
	"time"

	"github.com/prometheus/alertmanager/client"
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
func (b *Builder) CreateAlert(lbls, anns map[string]string, start, end time.Time) client.Alert {
	alert := client.Alert{
		Labels:       client.LabelSet{},
		Annotations:  client.LabelSet{},
		StartsAt:     start,
		EndsAt:       end,
		GeneratorURL: b.generator,
	}

	for k, v := range lbls {
		alert.Labels[client.LabelName(k)] = client.LabelValue(v)
	}
	for k, v := range anns {
		alert.Annotations[client.LabelName(k)] = client.LabelValue(v)
	}

	return alert
}
