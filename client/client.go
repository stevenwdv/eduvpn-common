package client

import (
	"fmt"
	"strings"

	"github.com/eduvpn/eduvpn-common/internal/config"
	"github.com/eduvpn/eduvpn-common/internal/discovery"
	"github.com/eduvpn/eduvpn-common/internal/fsm"
	"github.com/eduvpn/eduvpn-common/internal/log"
	"github.com/eduvpn/eduvpn-common/internal/server"
	"github.com/eduvpn/eduvpn-common/internal/util"
	"github.com/eduvpn/eduvpn-common/types"
)

type (
	// ServerBase is an alias to the internal ServerBase
	// This contains the details for each server
	ServerBase = server.ServerBase
)

func (client Client) isLetsConnect() bool {
	// see https://git.sr.ht/~fkooman/vpn-user-portal/tree/v3/item/src/OAuth/ClientDb.php
	return strings.HasPrefix(client.Name, "org.letsconnect-vpn.app")
}

// Client is the main struct for the VPN client
type Client struct {
	// The name of the client
	Name string `json:"-"`

	// The language used for language matching
	Language string `json:"-"` // language should not be saved

	// The chosen server
	Servers server.Servers `json:"servers"`

	// The list of servers and organizations from disco
	Discovery discovery.Discovery `json:"discovery"`

	// The fsm
	FSM fsm.FSM `json:"-"`

	// The logger
	Logger log.FileLogger `json:"-"`

	// The config
	Config config.Config `json:"-"`

	// Whether to enable debugging
	Debug bool `json:"-"`
}

// Register initializes the clientwith the following parameters:
//  - name: the name of the client
//  - directory: the directory where the config files are stored. Absolute or relative
//  - stateCallback: the callback function for the FSM that takes two states (old and new) and the data as an interface
//  - debug: whether or not we want to enable debugging
// It returns an error if initialization failed, for example when discovery cannot be obtained and when there are no servers.
func (client *Client) Register(
	name string,
	directory string,
	language string,
	stateCallback func(FSMStateID, FSMStateID, interface{}),
	debug bool,
) error {
	errorMessage := "failed to register with the GO library"
	if !client.InFSMState(STATE_DEREGISTERED) {
		return &types.WrappedErrorMessage{
			Message: errorMessage,
			Err:     FSMDeregisteredError{}.CustomError(),
		}
	}
	client.Name = name

	// TODO: Verify language setting?
	client.Language = language

	// Initialize the logger
	logLevel := log.LOG_WARNING
	if debug {
		logLevel = log.LOG_INFO
	}

	loggerErr := client.Logger.Init(logLevel, name, directory)
	if loggerErr != nil {
		return &types.WrappedErrorMessage{Message: errorMessage, Err: loggerErr}
	}

	// Initialize the FSM
	client.FSM = newFSM(stateCallback, directory, debug)
	client.Debug = debug

	// Initialize the Config
	client.Config.Init(directory, "state")

	// Try to load the previous configuration
	if client.Config.Load(&client) != nil {
		// This error can be safely ignored, as when the config does not load, the struct will not be filled
		client.Logger.Info("Previous configuration not found")
	}

	// Go to the No Server state with the saved servers after we're done
	defer client.FSM.GoTransitionWithData(STATE_NO_SERVER, client.Servers, true)

	// Let's Connect! doesn't care about discovery
	if client.isLetsConnect() {
		return nil
	}

	// Check if we are able to fetch discovery, and log if something went wrong
	_, discoServersErr := client.GetDiscoServers()
	if discoServersErr != nil {
		client.Logger.Warning(fmt.Sprintf("Failed to get discovery servers: %v", discoServersErr))
	}
	_, discoOrgsErr := client.GetDiscoOrganizations()
	if discoOrgsErr != nil {
		client.Logger.Warning(fmt.Sprintf("Failed to get discovery organizations: %v", discoOrgsErr))
	}

	return nil
}

// Deregister 'deregisters' the client, meaning saving the log file and the config and emptying out the client struct.
func (client *Client) Deregister() {
	// Close the log file
	client.Logger.Close()

	// Save the config
	saveErr := client.Config.Save(&client)
	if saveErr != nil {
		client.Logger.Info(
			fmt.Sprintf(
				"Failed saving configuration, error: %s",
				types.GetErrorTraceback(saveErr),
			),
		)
	}

	// Empty out the state
	*client = Client{}
}

// askProfile asks the user for a profile by moving the FSM to the ASK_PROFILE state.
func (client *Client) askProfile(chosenServer server.Server) error {
	base, baseErr := chosenServer.GetBase()
	if baseErr != nil {
		return &types.WrappedErrorMessage{Message: "failed asking for profiles", Err: baseErr}
	}
	client.FSM.GoTransitionWithData(STATE_ASK_PROFILE, &base.Profiles, false)
	return nil
}

// GetDiscoOrganizations gets the organizations list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#organization-list.
func (client *Client) GetDiscoOrganizations() (*types.DiscoveryOrganizations, error) {
	errorMessage := "failed getting discovery organizations list"
	// Not supported with Let's Connect!
	if client.isLetsConnect() {
		return nil, &types.WrappedErrorMessage{Message: errorMessage, Err: LetsConnectNotSupportedError{}}
	}

	orgs, orgsErr := client.Discovery.GetOrganizationsList()
	if orgsErr != nil {
		client.Logger.Warning(
			fmt.Sprintf(
				"Failed getting discovery organizations, Err: %s",
				types.GetErrorTraceback(orgsErr),
			),
		)
		return nil, &types.WrappedErrorMessage{
			Message: errorMessage,
			Err:     orgsErr,
		}
	}
	return orgs, nil
}

// GetDiscoServers gets the servers list from the discovery server
// If the list cannot be retrieved an error is returned.
// If this is the case then a previous version of the list is returned if there is any.
// This takes into account the frequency of updates, see: https://github.com/eduvpn/documentation/blob/v3/SERVER_DISCOVERY.md#server-list.
func (client *Client) GetDiscoServers() (*types.DiscoveryServers, error) {
	errorMessage := "failed getting discovery servers list"

	// Not supported with Let's Connect!
	if client.isLetsConnect() {
		return nil, &types.WrappedErrorMessage{Message: errorMessage, Err: LetsConnectNotSupportedError{}}
	}

	servers, serversErr := client.Discovery.GetServersList()
	if serversErr != nil {
		client.Logger.Warning(
			fmt.Sprintf("Failed getting discovery servers, Err: %s", types.GetErrorTraceback(serversErr)),
		)
		return nil, &types.WrappedErrorMessage{
			Message: errorMessage,
			Err:     serversErr,
		}
	}
	return servers, nil
}

// GetTranslated gets the translation for `languages` using the current state language.
func (client *Client) GetTranslated(languages map[string]string) string {
	return util.GetLanguageMatched(languages, client.Language)
}

type LetsConnectNotSupportedError struct{}

func (e LetsConnectNotSupportedError) Error() string {
	return "Any operation that involves discovery is not allowed with the Let's Connect! client"
}