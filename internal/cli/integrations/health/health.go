package health

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

func WaitForHTTPHealth(endpoint string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	client := http.Client{Timeout: 5 * time.Second}
	var lastErr error

	for {
		if time.Now().After(deadline) {
			if lastErr != nil {
				return lastErr
			}
			return errors.New("timed out waiting for API health")
		}

		resp, err := client.Get(endpoint)
		if err == nil {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 300 {
				return nil
			}
			lastErr = fmt.Errorf("health check returned %s", resp.Status)
		} else {
			lastErr = err
		}

		time.Sleep(3 * time.Second)
	}
}
