package threat

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type ipApiResponse struct {
	Status      string  `json:"status"`
	Country     string  `json:"country"`
	CountryCode string  `json:"countryCode"`
	RegionName  string  `json:"regionName"`
	City        string  `json:"city"`
}

// ResolveGeoIP attempts to get the country localization of an IP address using ip-api.com.
func ResolveGeoIP(ctx context.Context, ip string) string {
	if ip == "" {
		return ""
	}

	url := fmt.Sprintf("http://ip-api.com/json/%s?fields=status,country,countryCode,regionName,city", ip)
	
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return ""
	}

	// Limit wait to 2 seconds for GeoIP
	client := &http.Client{Timeout: 2 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return ""
	}

	var data ipApiResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return ""
	}

	if data.Status != "success" || data.Country == "" {
		return ""
	}

	return fmt.Sprintf("%s, %s", data.City, data.Country)
}
