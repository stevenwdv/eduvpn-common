package eduvpn

import (
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/jwijenbergh/eduvpn-common/internal"
)

func getServerURI(t *testing.T) string {
	serverURI := os.Getenv("SERVER_URI")
	if serverURI == "" {
		t.Skip("Skipping server test as no SERVER_URI env var has been passed")
	}
	return serverURI
}

func runCommand(t *testing.T, errBuffer *strings.Builder, name string, args ...string) error {
	cmd := exec.Command(name, args...)

	cmd.Stderr = errBuffer
	err := cmd.Start()
	if err != nil {
		return err
	}

	return cmd.Wait()
}

func loginOAuthSelenium(t *testing.T, url string, state *VPNState) {
	// We could use the go selenium library
	// But it does not support the latest selenium v4 just yet
	var errBuffer strings.Builder
	err := runCommand(t, &errBuffer, "python3", "selenium_eduvpn.py", url)
	if err != nil {
		t.Errorf("Login OAuth with selenium script failed with error %v and stderr %s", err, errBuffer.String())
		state.CancelOAuth()
	}
}

func stateCallback(t *testing.T, oldState string, newState string, data string, state *VPNState) {
	if newState == "OAuth_Started" {
		loginOAuthSelenium(t, data, state)
	}
}

func Test_server(t *testing.T) {
	serverURI := getServerURI(t)
	state := &VPNState{}

	state.Register("org.eduvpn.app.linux", "configstest", func(old string, new string, data string) {
		stateCallback(t, old, new, data, state)
	}, false)

	_, configErr := state.Connect(serverURI)

	if configErr != nil {
		t.Errorf("Connect error: %v", configErr)
	}
}

func test_connect_oauth_parameter(t *testing.T, parameters internal.URLParameters, expectedErr interface{}) {
	serverURI := getServerURI(t)
	state := &VPNState{}
	configDirectory := "test_oauth_parameters"

	state.Register("org.eduvpn.app.linux", configDirectory, func(oldState string, newState string, data string) {
		if newState == "OAuth_Started" {
			baseURL := "http://127.0.0.1:8000/callback"
			url, err := internal.HTTPConstructURL(baseURL, parameters)
			if err != nil {
				t.Errorf("Error: Constructing url %s with parameters %s", baseURL, fmt.Sprint(parameters))
			}
			go http.Get(url)

		}
	}, false)
	_, configErr := state.Connect(serverURI)

	if !errors.As(configErr, expectedErr) {
		t.Errorf("error %T = %v, wantErr %T", configErr, configErr, expectedErr)
	}
}

func Test_connect_oauth_parameters(t *testing.T) {
	var (
		failedCallbackParameterError  *internal.OAuthFailedCallbackParameterError
		failedCallbackStateMatchError *internal.OAuthFailedCallbackStateMatchError
	)

	tests := []struct {
		expectedErr interface{}
		parameters  internal.URLParameters
	}{
		{&failedCallbackParameterError, internal.URLParameters{}},
		{&failedCallbackParameterError, internal.URLParameters{"code": "42"}},
		{&failedCallbackStateMatchError, internal.URLParameters{"code": "42", "state": "21"}},
	}

	for _, test := range tests {
		test_connect_oauth_parameter(t, test.parameters, test.expectedErr)
	}
}

func Test_token_expired(t *testing.T) {
	serverURI := getServerURI(t)
	expiredTTL := os.Getenv("OAUTH_EXPIRED_TTL")
	if expiredTTL == "" {
		t.Log("No expired TTL present, skipping this test. Set EXPIRED_TTL env variable to run it")
		return
	}

	// Convert the env variable to an int and signal error if it is not possible
	expiredInt, expiredErr := strconv.Atoi(expiredTTL)
	if expiredErr != nil {
		t.Errorf("Cannot convert EXPIRED_TTL env variable to an int with error %v", expiredErr)
	}

	// Get a vpn state
	state := &VPNState{}

	state.Register("org.eduvpn.app.linux", "configsexpired", func(old string, new string, data string) {
		stateCallback(t, old, new, data, state)
	}, false)

	_, configErr := state.Connect(serverURI)

	if configErr != nil {
		t.Errorf("Connect error before expired: %v", configErr)
	}

	server, serverErr := state.Servers.GetCurrentServer()
	if serverErr != nil {
		t.Errorf("No server found")
	}

	accessToken := server.OAuth.Token.Access
	refreshToken := server.OAuth.Token.Refresh

	// Wait for TTL so that the tokens expire
	time.Sleep(time.Duration(expiredInt) * time.Second)

	infoErr := server.APIInfo()

	if infoErr != nil {
		t.Errorf("Info error after expired: %v", infoErr)
	}

	// Check if tokens have changed
	accessTokenAfter := server.OAuth.Token.Access
	refreshTokenAfter := server.OAuth.Token.Refresh

	if accessToken == accessTokenAfter {
		t.Errorf("Access token is the same after refresh")
	}

	if refreshToken == refreshTokenAfter {
		t.Errorf("Refresh token is the same after refresh")
	}
}

func Test_token_invalid(t *testing.T) {
	serverURI := getServerURI(t)
	state := &VPNState{}

	state.Register("org.eduvpn.app.linux", "configsinvalid", func(old string, new string, data string) {
		stateCallback(t, old, new, data, state)
	}, false)

	_, configErr := state.Connect(serverURI)

	if configErr != nil {
		t.Errorf("Connect error before invalid: %v", configErr)
	}

	// Fake connect and then back to authorized so that we can re-authorize
	// Going to authorized fakes a disconnect
	state.FSM.GoTransition(internal.CONNECTED)
	state.FSM.GoTransition(internal.AUTHORIZED)

	dummy_value := "37"

	server, serverErr := state.Servers.GetCurrentServer()
	if serverErr != nil {
		t.Errorf("No server found")
		return
	}

	// Override tokens with invalid values
	server.OAuth.Token.Access = dummy_value
	server.OAuth.Token.Refresh = dummy_value

	infoErr := server.APIInfo()

	if infoErr != nil {
		t.Errorf("Info error after invalid: %v", infoErr)
	}

	if server.OAuth.Token.Access == dummy_value {
		t.Errorf("Access token is equal to dummy value: %s", dummy_value)
	}

	if server.OAuth.Token.Refresh == dummy_value {
		t.Errorf("Refresh token is equal to dummy value: %s", dummy_value)
	}
}
