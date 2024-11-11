package main

import (
	"fmt"
	"testing"
)

func TestHandleURL(t *testing.T) {
	url := "http://test.com?param1=hallo&param2=world"
	parsedUrl, err := HandleURL(url)
	if err == nil && parsedUrl != url {
		t.Log("url should be http://test.com?param1=hallo&param2=world")
		t.Fail()
	}
}

// When we have no url of it is empty, return an error with message that url is empty
func ExampleHandleURL() {
	url := ""
	if _, err := HandleURL(url); err != nil {
		if err.Error() == "you missed to set url query param" {
			fmt.Println("msg should be you missed to set url query param")
		}
	}
	// Output:
	// msg should be you missed to set url query param
}
