package config

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
)

const configFileName = ".gatorconfig.json"

type Config struct {
	DB_url   string `json:"db_url"`
	Username string `json:"current_user_name"`
}

func getConfigFilePath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		fmt.Printf("Couldn't find HOME directory.\n")
		return "", err
	}
	home += "/"
	return home + configFileName, nil
}

func write(cfg Config) error {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		fmt.Println("Unable to get config file path.")
		return err
	}
	configFile, err := json.Marshal(cfg)
	if err != nil {
		fmt.Println("ERROR: Could not marshal config file into json")
		return err
	}
	err = os.WriteFile(configFilePath, configFile, 0666)
	if err != nil {
		fmt.Printf("ERROR: Unable to write file to %v\n", configFilePath)
		return err
	}
	fmt.Println("Write config success")
	return nil
}

func Read() Config {
	configFilePath, err := getConfigFilePath()
	if err != nil {
		log.Fatal("Unable to read config file.")
	}
	configFile, err := os.ReadFile(configFilePath)
	if err != nil {
		log.Fatal("Config file not found.")
	}
	var config Config
	err = json.Unmarshal(configFile, &config)
	if err != nil {
		log.Fatal("Could not unmarshal json data.")
	}
	fmt.Println("Successfully read config file.")
	return config
}

func SetUser(current_user_name string, cfg Config) error {
	cfg.Username = current_user_name
	err := write(cfg)
	if err != nil {
		return err
	}
	fmt.Println("Successfully set username")
	return nil
}
