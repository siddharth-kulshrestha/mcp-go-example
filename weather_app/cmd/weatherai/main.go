package main

import (
	"fmt"
	"log"
	"weatherapp/internal/client"
)

func main() {
	fmt.Println("Starting the weather app........")
	err := client.StartClient()
	if err != nil {
		fmt.Println("Got error!")
		log.Fatalln(err)
	}
}
