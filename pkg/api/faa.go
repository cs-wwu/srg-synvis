package api

// This file should be used for accessing the FAA data via API

const AIRSPACEURL = "https://services6.arcgis.com/ssFJjBXIUyZDrSYZ/arcgis/rest/services/Class_Airspace/FeatureServer/0/query?where=1%3D1&outFields=*&outSR=4326&f=json"

type AirspaceFAAFilters struct {
	State string
	Class string
}

func DownloadAirspaces(filters AirspaceFAAFilters) {

}
