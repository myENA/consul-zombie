package main

import (
	"log"
	"regexp"

	"github.com/hashicorp/consul/api"
)

func getClient(address string) (*api.Client, error) {
	config := api.DefaultConfig()
	if address != "" {
		config.Address = address
	}
	return api.NewClient(config)
}

func getList(serviceString string, tag string) []*api.ServiceEntry {
	client, err := getClient("")
	if err != nil {
		log.Fatalf("Unable to get a consul client connection: %s\n", err)
	}
	catalog := client.Catalog()

	// services is a map[string] to slice of tags
	services, _, err := catalog.Services(nil)
	if err != nil {
		log.Fatalf("Error getting serives from catalog: %s\n", err)
	}

	health := client.Health()
	var re *regexp.Regexp
	if serviceString != "" {
		re, err = regexp.Compile(serviceString)
		if err != nil {
			log.Fatalf("Error compiling <%s> as regexp: %s\n", serviceString, err)
		}
	}

	seOut := make([]*api.ServiceEntry, 0)

	for service, _ := range services {
		match := true
		if re != nil {
			str := re.FindString(service)
			match = (str != "")
		}
		if match {
			seList, _, err := health.Service(service, tag, false, nil)
			if err != nil {
				log.Fatalf("Unable to query health status of: %s\n", err)
			}
			for _, se := range seList {
				seOut = append(seOut, se)
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
