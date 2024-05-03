package main

import (
	"fmt"
	"io"
	"os"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

func check(e error) {
	if e != nil {
		fmt.Println(e)
	}
}

func main() {
	jsonFile, err := os.Open("osm.json")
	check(err)
	defer jsonFile.Close()
	byteValue, _ := io.ReadAll(jsonFile)
	ways := &SomeMessage{}
	opt := protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
	err = opt.Unmarshal(byteValue, ways)
	check(err)
	data, err := proto.Marshal(ways)
	check(err)
	outFile, err := os.Create("osm.pbuf")
	check(err)
	writeNum, err := outFile.Write(data)
	check(err)
	fmt.Println("Wrote", writeNum, "bytes")
}
