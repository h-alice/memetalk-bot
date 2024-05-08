package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
)

func main() {

	var config_path string

	// Load configuration from command line arguments.
	flag.StringVar(&config_path, "config", "config.yaml", "Path to configuration file.")

	flag.Parse()

	// Load configuration from file.
	config_data, err := os.ReadFile(config_path)
	print("config_data: ", config_data)
	if err != nil {
		log.Fatalf("[x] Error while reading configuration file: %v\n", err)
	}

	config, err := ParseConfig(config_data)
	if err != nil {
		log.Fatalf("[x] Error while parsing configuration file: %v\n", err)
	}

	fmt.Printf("%+v\n", config)

	ctx := context.Background()

	// Create IRC client.
	chatbot := NewChatbot(config)

	chatbot.Start(ctx)

	fmt.Println("Client exited")

}
