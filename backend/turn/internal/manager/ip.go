package manager

import (
	"backend/common"
	"bytes"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

func (tm *TurnManager) monitorAndReflectIPChange(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for range ticker.C {
		latestIP, err := common.FetchPublicWANIP()
		if err != nil {
			log.Printf("[TURN Monitor] Warning: failed to check WAN IP: %v", err)
			continue
		}

		tm.mu.Lock()
		activeIP := tm.publicIP
		tm.mu.Unlock()

		if !latestIP.Equal(activeIP) {
			log.Printf("[TURN Monitor] IP change detected! Old: %s -> New: %s", activeIP.String(), latestIP.String())
			err = tm.updateCloudflareDNS(latestIP.String())
			if err != nil {
				log.Panicf("[TURN Monitor] Error updating Cloudflare DNS: %v", err)
			}
			err = tm.serve(latestIP)
			if err != nil {
				log.Panicf("[TURN Monitor] Critical: Failed to restart TURN server with new IP: %v", err)
			}
		}
	}
}

func (tm *TurnManager) updateCloudflareDNS(newIP string) error {
	url := fmt.Sprintf("https://api.cloudflare.com/client/v4/zones/%s/dns_records/%s", tm.zoneID, tm.recordID)
	payload := fmt.Sprintf(`{"type":"A","name":"%s","content":"%s","proxied":false}`, tm.realm, newIP)
	req, err := http.NewRequest("PATCH", url, bytes.NewBuffer([]byte(payload)))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+tm.apiToken)
	req.Header.Set("Content-Type", "application/json")
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		bodyBytes, err1 := io.ReadAll(resp.Body)
		if err1 != nil {
			return fmt.Errorf("fail to read resp body : %v", err1)
		}
		return fmt.Errorf("cloudflare API rejected request (status %d): %s", resp.StatusCode, string(bodyBytes))
	}
	log.Printf("[TURN Monitor] Cloudflare DNS successfully pointed to %s", newIP)
	return nil
}
