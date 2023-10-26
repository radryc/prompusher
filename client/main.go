package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"io"
	"log"
	"net/http"
	"os"

	"github.com/radryc/prompusher/metrics"
)

func main() {
	mode := flag.String("mode", "register", "mode of operation (register or store)")
	filePath := flag.String("file", "", "path to JSON file containing the request")
	url := flag.String("url", "http://localhost:8080", "URL to send the request to")
	flag.Parse()

	var reqBody []byte
	var err error
	if *filePath != "" {
		reqBody, err = os.ReadFile(*filePath)
		if err != nil {
			log.Println("Error reading request file:", err)
			return
		}
	} else {
		if *mode == "register" {
			reg := metrics.RegistrationRequest{
				MetricsName: "my_metric",
				Labels: []map[string]string{
					{"key1": "value1"},
					{"key2": "value2"},
				},
				Prefix:        "prefix_foo",
				Type:          "counter",
				CheckInterval: 60,
				Help:          "This is a test metric",
			}
			reqBody, err = json.Marshal(reg)
			if err != nil {
				log.Println("Error marshaling registration request:", err)
				return
			}
		} else if *mode == "store" {
			store := metrics.StoreRequest{
				MetricsName: "my_metric",
				Labels: []map[string]string{
					{"key1": "value1"},
					{"key2": "value2"},
				},
				Prefix: "prefix_foo",
				Value:  1.0,
			}
			reqBody, err = json.Marshal(store)
			if err != nil {
				log.Println("Error marshaling store request:", err)
				return
			}
		} else {
			log.Println("Error: invalid mode specified")
			return
		}
	}

	postUrl := *url + "/store"
	if *mode == "register" {
		postUrl = *url + "/register"
	}

	resp, err := http.Post(postUrl, "application/json", bytes.NewBuffer(reqBody))
	if err != nil {
		log.Println("Error sending request:", err)
		return
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		log.Println("Error sending store request:", resp.Status)
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			log.Fatal(err)
		}
		bodyString := string(bodyBytes)
		log.Println(bodyString)
		return
	}
}
