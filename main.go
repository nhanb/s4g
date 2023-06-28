package main

import (
	"flag"
	"log"
	"net/http"
	"os"
)

func main() {
	var port, folder string
	flag.StringVar(&port, "port", "3338", "Web port")
	flag.StringVar(&folder, "folder", "www", "Web port")
	flag.Parse()

	err := os.Chdir(folder)
	if err != nil {
		log.Fatal(err)
	}

	println("Serving local website at http://localhost:" + port)
	fs := http.FileServer(http.Dir("."))
	http.Handle("/", fs)
	err = http.ListenAndServe("127.0.0.1:"+port, nil)
	if err != nil {
		log.Fatal(err)
	}
}
