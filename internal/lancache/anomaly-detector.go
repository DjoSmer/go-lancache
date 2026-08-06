// Detect any anomalies when client downloads file from storage.
// Anomaly 1: If a client tries to download file many times > AnomalyDetectorConfig.HistoryMax it's anomaly and Detect return true
package lancache

import (
	"fmt"
	"log/slog"
	"sync"
	"time"

	"go-lancache/internal/config"
)

type access struct {
	ip    string
	items []history
}

type history struct {
	Live         bool
	Key          string
	Times        int
	LastFinished time.Time
}

type AnomalyDetector struct {
	config            config.AnomalyDetectorConfig
	historyTTL        time.Duration
	cleanupTimeTicker time.Duration
	mu                sync.Mutex
	data              map[string]map[string]*history // ip -> key -> history
	stopWorker        chan struct{}
}

func NewAnomalyDetector(config config.AnomalyDetectorConfig, historyTTL time.Duration, cleanupTimeTicker time.Duration) *AnomalyDetector {
	detector := &AnomalyDetector{
		config:            config,
		historyTTL:        historyTTL,
		cleanupTimeTicker: cleanupTimeTicker,
		data:              make(map[string]map[string]*history, 255),
		stopWorker:        make(chan struct{}),
	}

	return detector
}

func (d *AnomalyDetector) Start() {
	go d.cleanupLoop()
}

func (d *AnomalyDetector) Stop() {
	close(d.stopWorker)
}

func (d *AnomalyDetector) Detect(ip, key string) bool {
	log := d.log("Detect")

	d.mu.Lock()
	defer d.mu.Unlock()

	_, ok := d.data[ip]
	if !ok {
		d.data[ip] = make(map[string]*history, 3000)
	}

	h, ok := d.data[ip][key]
	if !ok {
		h = &history{
			Key:          key,
			Times:        0,
			Live:         true,
			LastFinished: time.Now(),
		}
	}

	h.Live = true
	h.Times++

	if h.Times > d.config.HistoryMax {
		log.Debug(fmt.Sprintf("Anomaly - ip: %s key: %s", ip, key))
		h = nil
		delete(d.data[ip], key)
		return true
	}

	d.data[ip][key] = h

	return false
}

func (d *AnomalyDetector) Finish(ip, key string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	h, ok := d.data[ip][key]
	if !ok {
		return
	}

	h.Live = false
	h.LastFinished = time.Now()
}

func (d *AnomalyDetector) Size() int {
	d.mu.Lock()
	defer d.mu.Unlock()
	return len(d.data)
}

func (d *AnomalyDetector) GetAccessList() []access {
	d.mu.Lock()
	defer d.mu.Unlock()

	accesses := make([]access, len(d.data))

	x := 0
	for ip, data := range d.data {
		access := access{
			ip:    ip,
			items: make([]history, len(data)),
		}

		y := 0
		for _, history := range data {
			access.items[y] = *history
			y++
		}

		accesses[x] = access
		x++
	}

	return accesses
}

func (d *AnomalyDetector) Get(ip, key string) (history, error) {
	d.mu.Lock()
	defer d.mu.Unlock()

	_, ok := d.data[ip]
	if !ok {
		return history{}, fmt.Errorf("Ip: %s not found", ip)
	}

	h, ok := d.data[ip][key]
	if !ok {
		return history{}, fmt.Errorf("Key: %s not found", key)
	}

	return *h, nil
}

func (d *AnomalyDetector) cleanupLoop() {
	log := d.log("cleanupLoop")
	ticker := time.NewTicker(d.cleanupTimeTicker)

	for {
		select {
		case <-ticker.C:
			log.Debug("Cleanup - tick")
			d.mu.Lock()

			expireBefore := time.Now().Add(-d.historyTTL)

			for ip, histories := range d.data {
				for key, history := range histories {
					if !history.Live && history.LastFinished.Before(expireBefore) {
						log.Debug(fmt.Sprintf("Cleanup - ip: %s key: %s", ip, key))
						delete(histories, key)
					}
				}

				if len(histories) == 0 {
					delete(d.data, ip)
				}
			}

			d.mu.Unlock()

		case <-d.stopWorker:
			return
		}
	}
}

func (d *AnomalyDetector) log(name string) *slog.Logger {
	op := "AnomalyDetector." + name
	return slog.Default().With(slog.String("op", op))
}
