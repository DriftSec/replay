package main

import (
	"flag"
	"fmt"
	"log"
)

func main() {

	https := flag.Bool("https", false, "HTTPS request, defaults to HTTP")
	reqFile := flag.String("r", "", "The request file to replay")

	flag.Parse()

	if *reqFile == "" {
		fmt.Println()
		fmt.Println("[ERROR] What the hell am I supposed to replay?  -r is required. ")
		fmt.Println()
		flag.Usage()
	}

	var scheme string
	if *https {
		scheme = "https"
	} else {
		scheme = "http"
	}
	raw, err := ReadRawRequest(*reqFile, scheme)
	if err != nil {
		log.Fatal(err)
	}

	r, err := CreateReq(raw)
	if err != nil {
		log.Fatal(err)
	}
	resp, err := DoRequest(r, "")
	if err != nil {
		log.Fatal(err)
	}
	DumpResponse(resp)
}
