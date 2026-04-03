package main

import (
	"fmt"

	"tanabata/db"
)

func main() {
	db.InitDB("postgres://hiko:taikibansei@192.168.0.25/Tanabata_new?application_name=Tanabata%20testing")
	// test_json := json.RawMessage([]byte("{\"valery\": \"ponosoff\"}"))
	// data, statusCode, err := db.FileGetSlice(2, "", "+2", -2, 0)
	// data, statusCode, err := db.FileGet(1, "0197d056-cfb0-76b5-97e0-bd588826393c")
	// data, statusCode, err := db.FileAdd(1, "ABOBA.png", "image/png", time.Now(), "slkdfjsldkflsdkfj;sldkf", test_json)
	// statusCode, err := db.FileUpdate(2, "0197d159-bf3a-7617-a3a8-a4a9fc39eca6", map[string]interface{}{
	// 	"name": "ponos.png",
	// })
	statusCode, err := db.FileDelete(1, "0197d155-848f-7221-ba4a-4660f257c7d5")
	fmt.Printf("Status: %d\n", statusCode)
	fmt.Printf("Error:  %s\n", err)
	// fmt.Printf("%+v\n", data)
}
