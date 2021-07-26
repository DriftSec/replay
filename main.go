package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"strings"
)

type ReplacerSlice []string

var Replacers ReplacerSlice

func (i *ReplacerSlice) String() string {
	return fmt.Sprintf("%s", *i)
}

func (i *ReplacerSlice) Set(value string) error {
	*i = append(*i, value)
	return nil
}

func main() {

	flag.Var(&Replacers, "R", "Replace sting in request file. Use multiple times (-R infile=./test.txt will replace {{infile}} with ./test.txt)")
	dumpResp := flag.Bool("resp", false, "Print the full response")
	https := flag.Bool("https", false, "HTTPS request, defaults to HTTP")
	reqFile := flag.String("file", "", "The request file to replay")

	flag.Parse()

	if *reqFile == "" {
		fmt.Println()
		fmt.Println("[ERROR] What the hell am I supposed to replay?  -file is required. ")
		fmt.Println()
		flag.Usage()
	}
	repl := make(map[string]string)
	if len(Replacers) > 0 {
		for _, r := range Replacers {
			parts := strings.SplitN(r, "=", 2)
			if len(parts) != 2 {
				fmt.Println("[ERROR] Bad Replacement string:", r)
				os.Exit(1)
			}
			srch := parts[0]
			rep := parts[1]
			repl[srch] = rep
		}
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

	r, err := CreateReq(raw.ReplaceVars(repl))
	if err != nil {
		log.Fatal(err)
	}
	resp, err := DoRequest(r, "")
	if err != nil {
		log.Fatal(err)
	}
	if *dumpResp {
		PrintResponse(resp)
	} else {
		fmt.Println(resp.Status)
	}
}
