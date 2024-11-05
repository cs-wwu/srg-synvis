package test

import (
	"testing"

	"github.com/cs-wwu/srg-synvis/internal/processor"
)

func TestMain(m *testing.M) {
	m.Run()
}

func TestNoMaps(t *testing.T) {
	err := processor.GenerateOsmPng(nil, "testdir")
	if err == nil{
		t.Errorf("ran on nil, wanted err, got nil")
	}
}

func TestEmptyString(t *testing.T) {
	maps := [][]int{{0,0}}
	err := processor.GenerateOsmPng(maps, "")
	if err == nil{
		t.Errorf("ran on targetDir='', wanted err, got nil")
	}
}
