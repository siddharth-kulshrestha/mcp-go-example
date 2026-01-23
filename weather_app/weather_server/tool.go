package weatherserver

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type WeatherResponse struct {
	Name    string `json:"name"`
	Weather []struct {
		Description string `json:"description"`
	} `json:"weather"`
	Main struct {
		Temp      float64 `json:"temp"`
		FeelsLike float64 `json:"feels_like"`
		Humidity  int     `json:"humidity"`
	} `json:"main"`
	Wind struct {
		Speed float64 `json:"speed"`
	} `json:"wind"`
}

type WeatherResult struct {
	Location           string `json:"location"`
	Weather            string `json:"weather"`
	TemperatureCelsius string `json:"temperature_celsius"`
	FeelsLikeCelsius   string `json:"feels_like_celsius"`
	Humidity           string `json:"humidity"`
	WindSpeedMps       string `json:"wind_speed_mps"`
}

func GetWeather(location, openAPIMapWeatherKey string) (*WeatherResult, error) {
	return getWeatherMock(location, openAPIMapWeatherKey)
}

func getWeatherMock(location, openAPIMapWeatherKey string) (*WeatherResult, error) {
	mockJSON := `{
		"name": "Bengaluru",
		"weather": [{"description": "scattered clouds"}],
		"main": {
			"temp": 28.5,
			"feels_like": 30.2,
			"humidity": 55
		},
		"wind": {
			"speed": 5.4
		}
	}`
	var weatherResponse *WeatherResponse
	err := json.Unmarshal([]byte(mockJSON), &weatherResponse)
	if err != nil {
		return nil, err
	}
	result := WeatherResult{
		Location:           weatherResponse.Name,
		Weather:            weatherResponse.Weather[0].Description,
		TemperatureCelsius: fmt.Sprintf("%.1f°C", weatherResponse.Main.Temp),
		FeelsLikeCelsius:   fmt.Sprintf("%.1f°C", weatherResponse.Main.FeelsLike),
		Humidity:           fmt.Sprintf("%d%%", weatherResponse.Main.Humidity),
		WindSpeedMps:       fmt.Sprintf("%.1f m/s", weatherResponse.Wind.Speed),
	}

	// Output the result
	return &result, nil
}

func getWeather(location string, openAPIMapWeatherKey string) (*WeatherResult, error) {
	if openAPIMapWeatherKey == "" {
		return nil, fmt.Errorf("OpenAPIMapWeatherKey is empty")
	}
	baseURL := "http://api.openweathermap.org/data/2.5/weather"
	params := url.Values{}
	params.Add("q", location)
	params.Add("appid", openAPIMapWeatherKey)
	params.Add("units", "metric")
	fullURL := fmt.Sprintf("%s?%s", baseURL, params.Encode())
	// 4. Make the request
	// 3. Create a client with a timeout
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Get(fullURL)
	if err != nil {
		log.Fatalf("Request failed: %s", err)
	}
	defer resp.Body.Close()

	// 5. Check status code (equivalent to raise_for_status)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Fatalf("API returned non-200 status: %d", resp.StatusCode)
	}

	// 6. Read and Parse JSON
	body, _ := io.ReadAll(resp.Body)

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		log.Fatalf("JSON parse error: %s", err)
	}

	fmt.Printf("Weather Data: %v\n", data)

	// TODO: need some parsing for weather result as well.
	return nil, nil
}

func StartServer() {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "Weather App",
		Version: "0.0.1",
	}, nil)
	log.Println(s)
}
