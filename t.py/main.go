package main

import "fmt"

func v1(v ...string) {
	fmt.Println(v)
}
func main() {
	v1("eden,app", "")
}
