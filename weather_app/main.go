package main

import (
	"fmt"
	ws "weatherapp/weather_server"
)

func main() {
	// ws.StartServer()
	weatherResp, err := ws.GetWeather("London", "xyzzzyxzxs")
	fmt.Println(err)
	fmt.Println(weatherResp)
}
