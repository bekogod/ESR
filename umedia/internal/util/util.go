package util

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"io"
	"log"
)

func EncodeToBytes(i interface{}) []byte {

	buf := bytes.Buffer{}
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(i)
	if err != nil {
		log.Fatal(err)
	}
	return buf.Bytes()
}

func DecodeToStruct(s []byte, i interface{}) error {
	dec := gob.NewDecoder(bytes.NewReader(s))
	err := dec.Decode(i)
	if err != nil && err != io.EOF {
		fmt.Println("Error decoding:", err.Error())
		return err
	}
	return nil
}

func DeleteNodeFromList(s [][]string, ToDelete string) [][]string {
	new := make([][]string, 0, len(s))
	toAdd := true
	for _, i := range s {
		for _, j := range i {
			if j == ToDelete {
				toAdd = false
			}
		}
		if toAdd {
			new = append(new, i)
		}
	}
	return new
}
