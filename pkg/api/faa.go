package api

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
)

// This file should be used for accessing the FAA data via API

const AIRSPACEURL = "https://services6.arcgis.com/ssFJjBXIUyZDrSYZ/arcgis/rest/services/Class_Airspace/FeatureServer/0/query?where=STATE%20%3D%20'WA'%20AND%20CLASS%20%3D%20'D'&outFields=*&outSR=4326&f=json"

type AirspaceFAAFilters struct {
	State string
	Class string
}

func DownloadAirspaces(filters AirspaceFAAFilters) {
	res, err := http.Get(AIRSPACEURL)
	if err != nil {
		fmt.Print("Error getting airspace data")
		os.Exit(1)
	}
	defer res.Body.Close()

	if res.StatusCode != http.StatusOK {
		log.Fatalf("API request failed with code %d", res.StatusCode)
	}

	responseData, err := io.ReadAll(res.Body)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Println(responseData)
}
