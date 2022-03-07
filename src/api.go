package eduvpn

import (
	"encoding/json"
	"errors"
	"io/ioutil"
	"net/http"
)

type endpointList struct {
	Endpoint              string `json:"api_endpoint"`
	AuthorizationEndpoint string `json:"authorization_endpoint"`
	TokenEndpoint         string `json:"token_endpoint"`
}

// Struct that defines the json format for /.well-known/vpn-user-portal"
type EduVPNEndpoints struct {
	API struct {
		V2 endpointList `json:"http://eduvpn.org/api#2"`
		V3 endpointList `json:"http://eduvpn.org/api#3"`
	} `json:"api"`
	V string `json:"v"`
}

func APIGetEndpoints(vpnState *EduVPNState) (*EduVPNEndpoints, error) {
	url := vpnState.Server + "/.well-known/vpn-user-portal"
	resp, reqErr := http.Get(url)
	if reqErr != nil {
		return nil, reqErr
	}
	// Close the response body at the end
	defer resp.Body.Close()

	// Check if http response code is ok
	if resp.StatusCode != http.StatusOK {
		panic("http code not ok")
	}
	// Read the body
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return nil, readErr
	}

	structure := &EduVPNEndpoints{}
	jsonErr := json.Unmarshal(body, &structure)

	if jsonErr != nil {
		return nil, jsonErr
	}

	return structure, nil
}

func APIAuthenticatedInfo(vpnState *EduVPNState) (string, error) {
	url := vpnState.Endpoints.API.V3.Endpoint + "/info"
	resp, reqErr := vpnState.OAuth.client.Get(url)
	if reqErr != nil {
		return "", reqErr
	}
	// Close the response body at the end
	defer resp.Body.Close()

	// Check if http response code is ok
	if resp.StatusCode != http.StatusOK {
		return "", errors.New("HTTP code not ok")
	}
	// Read the body
	body, readErr := ioutil.ReadAll(resp.Body)
	if readErr != nil {
		return "", readErr
	}
	return string(body), nil
}