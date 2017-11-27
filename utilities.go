package main

import (
	"log"
	"regexp"

	"github.com/hashicorp/consul/api"
)

// get a client handle for a specified address (or the local agent if "")
func getClient(address string) (*api.Client, error) {
	config := api.DefaultConfig()
	if address != "" {
		config.Address = address
	}
	return api.NewClient(config)
}

// get a list of all services, limit to those matching the search criteria
func getList(serviceString string, tag string) []*api.ServiceEntry {
	client, err := getClient("")
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
			log.Fatalf("Unable to query for service \"%s\" from catalog: %s", svc, err)
		}
		for _, entry := range entries {
			if _, ok := nodeServiceMap[entry.Node.Node]; !ok {
				nodeServiceMap[entry.Node.Node] = make(map[string]*api.ServiceEntry)
			}
			nodeServiceMap[entry.Node.Node][entry.Service.ID] = entry
		}
	}

	// get a handle to the health endpoint and pre-calculate the regexp
	var re *regexp.Regexp
	if serviceString != "" {
		re = regexp.MustCompile(serviceString)
	}

	// prepare a slice to hold the result list
	seOut := make([]*api.ServiceEntry, 0)

	for _, services := range nodeServiceMap {
		for serviceID, service := range services {
			match := true
			if re != nil {
				idStr := re.FindString(serviceID)
				nameStr := re.FindString(service.Service.ID)
				match = idStr != "" || nameStr != ""
			}
			if match {
				seOut = append(seOut, service)
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

	var healthy bool = true
	var eligible int = 0
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
