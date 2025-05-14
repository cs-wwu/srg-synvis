package test

import (
	"errors"
	"fmt"
	"image"
	"image/color"
	"math"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/cs-wwu/srg-synvis/internal/processor"
)

func TestNoMaps(t *testing.T) {
	err := processor.GenerateOsmPng(nil, "testdir", processor.OSMURL)
	if err == nil {
		t.Errorf("ran on nil, wanted err, got nil")
	}
}

func TestEmptyString(t *testing.T) {
	maps := [][]int{{0, 0}}
	err := processor.GenerateOsmPng(maps, "", processor.OSMURL)
	if err == nil {
		t.Errorf("ran on targetDir='', wanted err, got nil")
	}
}

func TestConvertCoordsToPngIndex(t *testing.T) {
	highways := []processor.Element{}
	testElement := processor.Element{ID: 1, Type: "way", Lat: 48.9855963, Lon: -122.734034, Nodes: []int64{1}}
	highways = append(highways, testElement)
	roads := [][]processor.Point{}
	nodes := make(map[int64]processor.Element)
	lat := 48
	lon := -122
	for i := range highways {
		tempRoad := []processor.Point{}
		for j := range highways[i].Nodes {
			node := nodes[highways[i].Nodes[j]]
			// Transforms the lat/lon into x/y coordinates
			y := math.Round((1 - (math.Abs(node.Lat) - math.Abs(float64(lat)))) * (processor.Scale * processor.ImgLength))
			x := math.Round((1 - (math.Abs(node.Lon) - math.Abs(float64(lon)+1))) * (processor.Scale * processor.ImgLength))
			index := processor.Point{X: x, Y: y}
			tempRoad = append(tempRoad, index)
		}
		roads = append(roads, tempRoad)
	}

	testRoads := processor.ConvertCoordsToPngIndex(highways, nodes, processor.Scale, lat, lon)
	for i := range roads {
		for j := range roads[i] {
			if roads[i][j] != testRoads[i][j] {
				fmt.Println("roads =", roads[i][j])
				fmt.Println("testRoads =", testRoads[i][j])
				t.Errorf("roads[%d][%d] != testRoads[%d][%d]", i, j, i, j)
			}
		}
	}
}

func TestCoordToString(t *testing.T) {
	retA := processor.CoordToString(-49, -20)
	if retA != "s49w020" {
		t.Errorf("ran on (-49, -20), wanted 's49w020', got %s", retA)
	}

	retB := processor.CoordToString(49, -20)
	if retB != "n49w020" {
		t.Errorf("ran on (49, -20), wanted 'n49w020', got %s", retB)
	}

	retC := processor.CoordToString(-49, 20)
	if retC != "s49e020" {
		t.Errorf("ran on (-49, 20), wanted 's49e020', got %s", retC)
	}

	retD := processor.CoordToString(49, 20)
	if retD != "n49e020" {
		t.Errorf("ran on (49, 20), wanted 'n49e020', got %s", retD)
	}

	retE := processor.CoordToString(0, 0)
	if retE != "n00e000" {
		t.Errorf("ran on (0, 0), wanted 'n00e000', got %s", retE)
	}
}

func TestDrawLine(t *testing.T) {
	start := processor.Point{X: 1, Y: 8}
	end := processor.Point{X: 10, Y: 8}
	width := 4.0
	baseImage := image.NewRGBA(image.Rect(0, 0, 100, 100))
	testImage := image.NewRGBA(image.Rect(0, 0, 100, 100))
	corners := []processor.Point{
		{X: 1, Y: 10},
		{X: 1, Y: 6},
		{X: 10, Y: 6},
		{X: 10, Y: 10},
	}

	for y := 0; y <= 100; y++ {
		for x := 0; x <= 100; x++ {
			baseImage.Set(x, y, color.Transparent)
		}
	}

	for y := 0; y <= 100; y++ {
		for x := 0; x <= 100; x++ {
			baseImage.Set(x, y, color.Transparent)
		}
	}

	for y := 6; y <= 10; y++ {
		for x := 1; x <= 10; x++ {
			if processor.IsPointInPolygon(float64(x), float64(y), corners) {
				baseImage.Set(x, y, color.Opaque)
				baseImage.Set(x, y, color.Opaque)
			}
		}
	}

	processor.DrawLine(start, end, width, testImage, color.Opaque)
	processor.DrawLine(start, end, width, testImage, color.Opaque)

	for p := range baseImage.Pix {
		if baseImage.Pix[p] != testImage.Pix[p] {
			t.Errorf("Pixel: %d, %d != %d", p, baseImage.Pix[p], testImage.Pix[p])
		}
	}
}
func TestDrawFilledPolygon(t *testing.T) {
	corners := []processor.Point{
		{X: 1, Y: 1},
		{X: 11, Y: 1},
		{X: 1, Y: 11},
		{X: 11, Y: 11},
	}

	baseImage := image.NewRGBA(image.Rect(0, 0, 100, 100))
	testImage := image.NewRGBA(image.Rect(0, 0, 100, 100))

	for y := 0; y <= 100; y++ {
		for x := 0; x <= 100; x++ {
			baseImage.Set(x, y, color.Transparent)
		}
	}
	for y := 1; y <= 11; y++ {
		for x := 1; x <= 11; x++ {
			if processor.IsPointInPolygon(float64(x), float64(y), corners) {
				baseImage.Set(x, y, color.Opaque)
				baseImage.Set(x, y, color.Opaque)
			}
		}
	}

	processor.DrawFilledPolygon(testImage, corners, color.Opaque)
	processor.DrawFilledPolygon(testImage, corners, color.Opaque)

	for p := range baseImage.Pix {
		if baseImage.Pix[p] != testImage.Pix[p] {
			t.Errorf("Pixel: %d, %d != %d", p, baseImage.Pix[p], testImage.Pix[p])
		}
	}
}
func TestMakeApiRequest(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name             string
		bbStatusCode     int
		bbBody           string
		wantElementCount int
		wantErr          error
	}{
		{
			name:         "bad status code",
			bbStatusCode: http.StatusInternalServerError,
			wantErr:      errors.New("error"),
		},
		{
			name:         "one available element",
			bbStatusCode: http.StatusOK,
			bbBody: `{
    				"version": 0.6,
    				"generator": "Overpass API 0.7.61.8 b1080abd",
					"osm3s": {
						"timestamp_osm_base": "2024-10-28T22:08:39Z",
						"copyright": "The data included in this document is from www.openstreetmap.org. The data is made available under ODbL."
					},
					"elements": [
						{
							"type": "way",
							"id": 5250240,
							"bounds": {
								"minlat": 48.9845541,
								"minlon": -122.7349281,
								"maxlat": 48.9893445,
								"maxlon": -122.7331845
							},
							"nodes": [
								37011528,
								6343941244,
								6343941227,
								6343941247,
								6343941265,
								6343941219,
								6343941239,
								6346044042,
								37011529,
								6346044041,
								6346044040,
								6346044039,
								6346044038,
								37011531,
								6346044043,
								37011534,
								37011535,
								37011536,
								6344179292,
								37011537
							],
							"geometry": [
								{
									"lat": 48.9845541,
									"lon": -122.7331845
								},
								{
									"lat": 48.984787,
									"lon": -122.7333241
								},
								{
									"lat": 48.9848992,
									"lon": -122.7334092
								},
								{
									"lat": 48.985143,
									"lon": -122.733618
								},
								{
									"lat": 48.9853769,
									"lon": -122.7338266
								},
								{
									"lat": 48.9855963,
									"lon": -122.734034
								},
								{
									"lat": 48.9859669,
									"lon": -122.7344037
								},
								{
									"lat": 48.9860565,
									"lon": -122.7344913
								},
								{
									"lat": 48.9861415,
									"lon": -122.7345664
								},
								{
									"lat": 48.9862393,
									"lon": -122.7346408
								},
								{
									"lat": 48.9863257,
									"lon": -122.7346992
								},
								{
									"lat": 48.9864272,
									"lon": -122.7347534
								},
								{
									"lat": 48.9865207,
									"lon": -122.7347949
								},
								{
									"lat": 48.986624,
									"lon": -122.7348316
								},
								{
									"lat": 48.9867335,
									"lon": -122.7348613
								},
								{
									"lat": 48.986843,
									"lon": -122.7348766
								},
								{
									"lat": 48.9869491,
									"lon": -122.734883
								},
								{
									"lat": 48.9870689,
									"lon": -122.7348895
								},
								{
									"lat": 48.988876,
									"lon": -122.7348898
								},
								{
									"lat": 48.9893445,
									"lon": -122.7349281
								}
							],
							"tags": {
								"bicycle": "yes",
								"bus": "designated",
								"destination": "H Street \u2013 Truck Customs",
								"destination:ref": "WA 543 North",
								"hgv": "designated",
								"highway": "motorway",
								"maxspeed": "40 mph",
								"oneway": "yes",
								"ref": "WA 543",
								"tiger:cfcc": "A63",
								"tiger:county": "Whatcom, WA",
								"tiger:reviewed": "no"
							}
						}`,
			wantElementCount: 1,
		},
	}

	// Each iteration of this loop creates a unique in-memory test server to run the http request against.
	// The post request returns the different data sets that are defined above this comment, those data sets
	// also include expected return counts for testing purposes
	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			testServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.bbStatusCode)
				_, err := w.Write([]byte(tc.bbBody))
				if err != nil {
					t.Errorf("Issues with httptest server: %e", err)
				}
			}))
			defer testServer.Close()

			maps := []int{48, -123}
			gotElements, gotErr := processor.MakeApiRequest(testServer.URL, maps)

			if tc.wantErr != nil && gotErr == nil {
				t.Errorf("wanted err: %e, got err: %e", tc.wantErr, gotErr)
			}
			if gotErr == nil {
				if len(gotElements.Elements) != tc.wantElementCount {
					t.Errorf("wanted %d elements, got %d elements", len(gotElements.Elements), tc.wantElementCount)
				}
			}
		})
	}
}
func TestIsPointInPolygon(t *testing.T) {
	polygon := []processor.Point{{X: 0.0, Y: 0.0}, {X: 1.0, Y: 0.0}, {X: 1.0, Y: 1.0}, {X: 0.0, Y: 1.0}}

	retA := processor.IsPointInPolygon(-1, -1, polygon)
	if retA {
		t.Errorf("ran on (-1, -1), wanted false, got true")
	}

	retB := processor.IsPointInPolygon(0, 0, polygon)
	if !retB {
		t.Errorf("ran on (0, 0), wanted true, got false")
	}

	retC := processor.IsPointInPolygon(0.5, 0.5, polygon)
	if !retC {
		t.Errorf("ran on (0.5, 0.5), wanted true, got false")
	}

	retD := processor.IsPointInPolygon(-1.0, 0.5, polygon)
	if retD {
		t.Errorf("ran on (-1.0, 0.5), wanted false, got true")
	}

	retF := processor.IsPointInPolygon(-1.0, 1.0, polygon)
	if retF {
		t.Errorf("ran on (-1.0, 1.0), wanted false got true")
	}
}

func TestDrawRoads(t *testing.T) {
	highways := []processor.Element{}
	testElement := processor.Element{ID: 1, Type: "way", Lat: 48.9855963, Lon: -122.734034, Nodes: []int64{1}}
	highways = append(highways, testElement)
	nodes := make(map[int64]processor.Element)
	lat := 48
	lon := -122
	roads := processor.ConvertCoordsToPngIndex(highways, nodes, processor.Scale, lat, lon)
	baseImage := image.NewRGBA(image.Rect(0, 0, processor.Scale*processor.ImgLength, processor.Scale*processor.ImgLength))
	testImage := image.NewRGBA(image.Rect(0, 0, processor.Scale*processor.ImgLength, processor.Scale*processor.ImgLength))

	// Connect all the points
	for r := range roads {
		road := roads[r]
		for p := 0; p < len(road)-1; p++ {
			p1 := road[p]
			p2 := road[p+1]
			processor.DrawLine(p1, p2, 8, baseImage, color.Opaque)
		}
	}

	coords := []int{lat, lon}
	processor.DrawRoads(highways, nodes, coords, testImage, "motorway")

	for p := range baseImage.Pix {
		if baseImage.Pix[p] != testImage.Pix[p] {
			t.Errorf("Pixel: %d, %d != %d", p, baseImage.Pix[p], testImage.Pix[p])
		}
	}
}
