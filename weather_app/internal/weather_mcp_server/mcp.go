package weather_mcp_server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
)

type Weather struct {
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

type WeatherResponse []*Weather

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

func CompareWeather(locationA, locationB string) string {
	return fmt.Sprintf(`You are acting as a helpful weather analyst. 
Your goal is to provide a clear and easy-to-read comparison of the weather in two different locations for a user.

The user wants to compare the weather between %q and %q.

To accomplish this, follow these steps:
1. First, gather the necessary weather data for both %q and %q.
2. Once you have the weather data for both locations, DO NOT simply list the raw results.
3. Instead, synthesize the information into a concise summary. Your final response should highlight the key differences, focusing on temperature, the general conditions (e.g., 'sunny' vs 'rainy'), and wind speed.
4. Present the comparison in a structured format, like a markdown table or a clear bulleted list, to make it easy for the user to understand at a glance.`,
		locationA, locationB, locationA, locationB)
}

func getWeatherMock(location, openAPIMapWeatherKey string) (*WeatherResult, error) {
	mockJSON := `[
  {
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
  },
  {
    "name": "London",
    "weather": [{"description": "light rain"}],
    "main": {
      "temp": 12.4,
      "feels_like": 10.8,
      "humidity": 82
    },
    "wind": {
      "speed": 3.1
    }
  },
  {
    "name": "New York",
    "weather": [{"description": "clear sky"}],
    "main": {
      "temp": -2.0,
      "feels_like": -7.5,
      "humidity": 40
    },
    "wind": {
      "speed": 8.2
    }
  }
]`
	var weatherResponse *WeatherResponse
	err := json.Unmarshal([]byte(mockJSON), &weatherResponse)
	if err != nil {
		return nil, err
	}

	var weather *Weather
	for _, w := range *weatherResponse {
		if strings.EqualFold(w.Name, location) {
			weather = w
			break
		}
	}

	result := WeatherResult{
		Location:           weather.Name,
		Weather:            weather.Weather[0].Description,
		TemperatureCelsius: fmt.Sprintf("%.1f°C", weather.Main.Temp),
		FeelsLikeCelsius:   fmt.Sprintf("%.1f°C", weather.Main.FeelsLike),
		Humidity:           fmt.Sprintf("%d%%", weather.Main.Humidity),
		WindSpeedMps:       fmt.Sprintf("%.1f m/s", weather.Wind.Speed),
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

	// fmt.Printf("Weather Data: %v\n", data)

	// TODO: need some parsing for weather result as well.
	return nil, nil
}

type WeatherInput struct {
	Location string `json:"location" jsonschema:"description:The city to get weather for"`
	Units    string `json:"units" jsonschema:"enum:metric|imperial,default:metric"`
}

type WeatherOutput struct {
	Location string         `json:"location" jsonschema:"description:The city to get weather for"`
	Result   *WeatherResult `json:"result" jsonschema:"description:Weather result for the city"`
}

func GetWeatherToolHandler(ctx context.Context, ctr *mcp.CallToolRequest, input *WeatherInput) (*mcp.CallToolResult, *WeatherOutput, error) {
	location := input.Location
	wr, err := GetWeather(location, "xyzaza")
	if err != nil {
		return nil, nil, err
	}
	wo := &WeatherOutput{
		Location: location,
		Result:   wr,
	}
	return nil, wo, nil

}

func CompareWeatherPrompt(ctx context.Context, req *mcp.GetPromptRequest) (*mcp.GetPromptResult, error) {
	locationA := req.Params.Arguments["location_a"]
	locationB := req.Params.Arguments["location_b"]
	if locationA == "" || locationB == "" {
		return nil, fmt.Errorf("any one or both the locations are empty: 1. %s,\n 2. %s", locationA, locationB)
	}

	prompt := CompareWeather(locationA, locationB)
	return &mcp.GetPromptResult{
		Description: "Weather comparision prompt",
		Messages: []*mcp.PromptMessage{
			{
				Role: "user",
				Content: &mcp.TextContent{
					Text: prompt,
				},
			},
		},
	}, nil

}

// HandleDeliveryLog is the resource handler for reading the physical file.
func HandleDeliveryLog(ctx context.Context, req *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
	// 1. Read the local file
	data, err := os.ReadFile("delivery_log.txt")

	// 2. Handle File Not Found specifically
	if os.IsNotExist(err) {
		return &mcp.ReadResourceResult{
			Contents: []*mcp.ResourceContents{
				&mcp.ResourceContents{
					URI:      req.Params.URI,
					MIMEType: "text/plain",
					Text:     "Error: The delivery_log.txt file was not found on the server.",
				},
			},
		}, nil
	}

	// 3. Handle unexpected errors
	if err != nil {
		return nil, fmt.Errorf("unexpected error reading log: %w", err)
	}

	// 4. Mimic the Python logic: strip and process lines
	// Since we return a single string to the LLM, we just clean up the text.
	content := strings.TrimSpace(string(data))

	return &mcp.ReadResourceResult{
		Contents: []*mcp.ResourceContents{
			&mcp.ResourceContents{
				URI:      req.Params.URI,
				MIMEType: "text/plain",
				Text:     content,
			},
		},
	}, nil
}

func StartServer() {
	s := mcp.NewServer(&mcp.Implementation{
		Name:    "Weather-Tool",
		Version: "0.0.1",
	}, nil)

	// Adding tool to fetch weather of a specific city
	mcp.AddTool(s, &mcp.Tool{
		Name:        "get_weather",
		Description: "Gets weather details based on a description",
	}, GetWeatherToolHandler)

	// Adding prompts to fetch weather for multiple cities..
	s.AddPrompt(&mcp.Prompt{
		Name:        "compare_weather",
		Description: "Generates a clear, comparative summary of the weather between two specified locations.",
		Arguments: []*mcp.PromptArgument{
			{
				Name:        "location_a",
				Description: "The first city for comparison (e.g., 'London')",
				Required:    true,
			},
			{
				Name:        "location_b",
				Description: "The second city for comparison (e.g., 'Paris')",
				Required:    true,
			},
		},
	}, CompareWeatherPrompt)

	// Adding resource of log file
	s.AddResource(&mcp.Resource{
		Name:        "file://delivery_log",
		Title:       "Delivery Log",
		Description: "Real-time delivery logs containing order numbers and locations.",
		MIMEType:    "text/plain",
	}, HandleDeliveryLog)

	// Running the server
	if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatal(err)
	}

}
