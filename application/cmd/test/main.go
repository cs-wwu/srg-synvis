package main

import (
	"log"

	"github.com/cs-wwu/srg-synvis/test"
)

func init() {
	log.SetFlags(log.Lshortfile)
}

func main() {
	test.VulkanMathTest()
}
