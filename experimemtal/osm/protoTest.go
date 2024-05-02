package main

import "fmt"
import (
	//"google.golang.org/protobuf/encoding/protojson"
	//"google.golang.org/protobuf/proto"
	"encoding/json"
	//"protoBufs"
	"io"
	"os"
)

type Elements struct{
	Elements []Element `json:"elements"`
}

type Element struct{
	Type 		string 		`json:"type"`
	Bounds 		Bounds 		`json:"bounds"`
	Geometry	[]Geometry 	`json:"geometry"`
	Tags		[]string	`json:"tags"`
}

type Geometry struct{
	Lat float32 `json:"lat"`
	Lon float32 `json:"lon"`
}

type Bounds struct{
	MinLat float32 `json:"minlat"`
	MinLon float32 `json:"minlon"`
	MaxLat float32 `json:"maxlat"`
	MaxLon float32 `json:"maxlon"`
}

func main() {
	jsonFile, err := os.Open("osm.json")
	if err != nil {
		fmt.Println(err)
	}

	defer jsonFile.Close()

	byteValue, _ := io.ReadAll(jsonFile)

	var ways Elements

	json.Unmarshal(byteValue, &ways)

	fmt.Println("Element type: " + ways.Elements[0].Type)
	fmt.Println("Bounds: ")
	fmt.Println("	MinLat: ", ways.Elements[0].Bounds.MinLat);
	fmt.Println("	MinLon: ", ways.Elements[0].Bounds.MinLon);
	fmt.Println("	MaxLat: ", ways.Elements[0].Bounds.MaxLat);
	fmt.Println("	MaxLon: ", ways.Elements[0].Bounds.MaxLon);

	//req := &proto.Way{}
	//errr := protojson.Unmarshal(byteValue, req)

	//if errr != nil {
		//fmt.Println(errr)
	//}


}
