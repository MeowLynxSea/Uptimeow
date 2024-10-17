package main

import (
	"log"
	"net/http"
	"strconv"

	"github.com/MeowLynxSea/Uptimeow/api"
	"github.com/MeowLynxSea/Uptimeow/config"
	"github.com/MeowLynxSea/Uptimeow/web"
)

var GlobalConfig config.ConfigData

func main() {
	GlobalConfig = config.Load()

	http.HandleFunc("/ws", api.WebSocketHandler)
	http.HandleFunc("/api/", api.APIHandler)
	http.HandleFunc("/", web.IndexHandler)

	log.Println("[INFO] Starting server on " + GlobalConfig.Web.Host + ":" + strconv.Itoa(GlobalConfig.Web.Port) + "...")
	if err := http.ListenAndServe(GlobalConfig.Web.Host+":"+strconv.Itoa(GlobalConfig.Web.Port), nil); err != nil {
		panic(err)
	}
}
