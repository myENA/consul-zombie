package main

import (
	"log"
	"regexp"

	"github.com/hashicorp/consul/api"
)

// get a client handle for a specified address (or the local agent if "")
func getClient(address, token string) (*api.Client, error) {
	config := api.DefaultConfig()
	if address != "" {
		config.Address = address
	}
	if token != "" {
		config.Token = token
	}
	return api.NewClient(config)
}

// get a list of all services, limit to those matching the search criteria
func getList(serviceString, token, tag string) []*api.ServiceEntry {
	client, err := getClient("", token)
	if err != nil {
		log.Fatalf("Unable to get a consul client connection: %s\n", err)
	}

	serviceList, _, err := client.Catalog().Services(nil)
	if err != nil {
		log.Fatalf("Unable to get list of services from catalog: %s", err)
	}

	nodeServiceMap := make(map[string]map[string]*api.ServiceEntry)

	for svc := range serviceList {
		entries, _, err := client.Health().Service(svc, tag, false, nil)
		if err != nil {
			log.Fatalf("Unable to query for service \"%s\" health: %s", svc, err)
		}
		for _, entry := range entries {
			if _, ok := nodeServiceMap[entry.Node.Node]; !ok {
				nodeServiceMap[entry.Node.Node] = make(map[string]*api.ServiceEntry)
			}
			nodeServiceMap[entry.Node.Node][entry.Service.ID] = entry
		}
	}

	seOut := make([]*api.ServiceEntry, 0)
	if serviceString == "" {
		for _, services := range nodeServiceMap {
			for _, service := range services {
				seOut = append(seOut, service)
			}
		}
	} else {
		var serviceID string
		var services map[string]*api.ServiceEntry
		var service *api.ServiceEntry
		re := regexp.MustCompile(serviceString)
		for _, services = range nodeServiceMap {
			for serviceID, service = range services {
				if re.FindString(serviceID) != "" || re.FindString(service.Service.Service) != "" {
					seOut = append(seOut, service)
				}
			}
		}
	}

	return seOut
}

// A service entry is considered healty if all the eligible checks are passing.
// serfChecks are not eligible
func isHealthy(se *api.ServiceEntry) bool {
	if se == nil || se.Checks == nil || len(se.Checks) == 0 {
		// No checks = failing
		return false
	}

	healthy := true
	eligible := 0
	for _, c := range se.Checks {
		if c.Name == "serfHealth" {
			continue
		}
		// All found checks have to be passing
		healthy = healthy && (c.Status == "passing")
		eligible++
	}

	// No eligible checks were found
	if eligible == 0 {
		return false
	}

	return healthy
}
