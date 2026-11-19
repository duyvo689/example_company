package outbox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"example-company/internal/domain"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

type Dispatcher struct {
	db       *gorm.DB
	endpoint string
	interval time.Duration
	client   *http.Client
	log      *slog.Logger
}

func New(db *gorm.DB, endpoint string, interval time.Duration, log *slog.Logger) *Dispatcher {
	return &Dispatcher{db: db, endpoint: endpoint, interval: interval, client: &http.Client{Timeout: 10 * time.Second}, log: log}
}

func (d *Dispatcher) Run(ctx context.Context) {
	if d.endpoint == "" {
		d.log.Info("outbox_disabled", "reason", "INTEGRATION_WEBHOOK_URL is empty")
		return
	}
	ticker := time.NewTicker(d.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			for i := 0; i < 20; i++ {
				processed, err := d.dispatchOne(ctx)
				if err != nil {
					d.log.Error("outbox_dispatch", "error", err)
				}
				if !processed {
					break
				}
			}
		}
	}
}

func (d *Dispatcher) dispatchOne(ctx context.Context) (bool, error) {
	var event domain.IntegrationEvent
	err := d.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Clauses(clause.Locking{Strength: "UPDATE", Options: "SKIP LOCKED"}).
			Where("status IN ? AND next_attempt_at <= ?", []string{"pending", "failed"}, time.Now()).
			Order("created_at ASC").First(&event)
		if result.Error != nil {
			return result.Error
		}
		return tx.Model(&event).Updates(map[string]any{"status": "processing", "attempts": gorm.Expr("attempts + 1")}).Error
	})
	if err == gorm.ErrRecordNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, d.endpoint, bytes.NewReader(event.Payload))
	if err != nil {
		return true, d.fail(ctx, &event, err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Event-ID", event.ID.String())
	req.Header.Set("X-Event-Type", event.EventType)
	resp, err := d.client.Do(req)
	if err != nil {
		return true, d.fail(ctx, &event, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return true, d.fail(ctx, &event, fmt.Errorf("webhook returned HTTP %d", resp.StatusCode))
	}
	now := time.Now()
	err = d.db.WithContext(ctx).Model(&event).Updates(map[string]any{"status": "published", "published_at": now, "last_error": ""}).Error
	return true, err
}

func (d *Dispatcher) fail(ctx context.Context, event *domain.IntegrationEvent, cause error) error {
	attempt := event.Attempts + 1
	delay := time.Duration(1<<min(attempt, 8)) * time.Minute
	if err := d.db.WithContext(ctx).Model(event).Updates(map[string]any{"status": "failed", "last_error": cause.Error(), "next_attempt_at": time.Now().Add(delay)}).Error; err != nil {
		return err
	}
	return cause
}
