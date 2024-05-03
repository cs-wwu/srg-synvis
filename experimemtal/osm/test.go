package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func main() {
	jsonFile, err := os.Open("osm.json")
	if err != nil {
		fmt.Println(err)
	}
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	ways := &SomeMessage{}
	opt := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = opt.Unmarshal(byteValue, ways)
	if err != nil {
		fmt.Println(err)
	}
	data, err := proto.Marshal(ways)
	if err != nil {
		fmt.Println(err)
	}
	proto.Unmarshal(data, ways)
}
