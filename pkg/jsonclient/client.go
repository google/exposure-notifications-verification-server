package jsonclient

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net/http"
)

func MakeRequest(client *http.Client, url string, input interface{}, output interface{}) error {

	data, err := json.Marshal(input)
	if err != nil {
		return err
	}

	buffer := bytes.NewBuffer(data)
	r, err := client.Post(url, "application/json", buffer)
	if err != nil {
		return err
	}
	defer r.Body.Close()

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return err
	}

	err = json.Unmarshal(body, output)
	return nil
}
