package client

import (
	"time"

	"github.com/go-openapi/strfmt"
	"github.com/prometheus/alertmanager/api/v2/models"
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
func (b *Builder) CreateAlert(lbls, anns map[string]string, start, end time.Time) *models.PostableAlert {
	return &models.PostableAlert{
		Annotations: anns,
		StartsAt:    strfmt.DateTime(start),
		EndsAt:      strfmt.DateTime(end),
		Alert: models.Alert{
			Labels:       lbls,
			GeneratorURL: strfmt.URI(b.generator),
		},
	}
}
