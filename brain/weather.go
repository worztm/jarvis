package brain

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Weather pulls live conditions from wttr.in (keyless plain HTTP).
type Weather struct {
	Place  string
	Desc   string
	TempC  int
	FeelsC int
	Humid  int
	Wind   int
}

type wttrResponse struct {
	CurrentCondition []struct {
		TempC      string `json:"temp_C"`
		FeelsLikeC string `json:"FeelsLikeC"`
		Humidity   string `json:"humidity"`
		WindKmph   string `json:"windspeedKmph"`
		WeatherDesc []struct {
			Value string `json:"value"`
		} `json:"weatherDesc"`
	} `json:"current_condition"`
	NearestArea []struct {
		AreaName []struct {
			Value string `json:"value"`
		} `json:"areaName"`
	} `json:"nearest_area"`
}

var httpClient = &http.Client{Timeout: 9 * time.Second}

// GetWeather fetches current conditions for city, or auto-location when empty.
func GetWeather(city string) (*Weather, error) {
	endpoint := "https://wttr.in/?format=j1"
	if city != "" {
		endpoint = fmt.Sprintf("https://wttr.in/%s?format=j1", url.PathEscape(city))
	}
	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "jarvis-local-go/1.0")

	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("weather service returned %s", resp.Status)
	}

	var data wttrResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		return nil, err
	}
	if len(data.CurrentCondition) == 0 {
		return nil, fmt.Errorf("weather service returned no data")
	}

	cc := data.CurrentCondition[0]
	w := &Weather{
		Desc:   strings.ToLower(strings.TrimSpace(cc.WeatherDesc[0].Value)),
		TempC:  atoi(cc.TempC),
		FeelsC: atoi(cc.FeelsLikeC),
		Humid:  atoi(cc.Humidity),
		Wind:   atoi(cc.WindKmph),
	}
	if city != "" && len(data.NearestArea) > 0 {
		w.Place = data.NearestArea[0].AreaName[0].Value
	} else if city != "" {
		w.Place = city
	} else {
		w.Place = "your location"
	}
	return w, nil
}

func atoi(s string) int {
	n := 0
	for _, r := range s {
		if r < '0' || r > '9' {
			return n
		}
		n = n*10 + int(r-'0')
	}
	return n
}
