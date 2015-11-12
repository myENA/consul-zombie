// list and offer to kill dead consul services
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/consul/api"
)

// this is the default port for talking to remote consul agents
const CONSUL_PORT = 8500

func usage(code int) {
	fmt.Printf("usage: zombie (hunt|kill) [options]\n")
	fmt.Printf("Search (hunt) or deregister (kill) services: zombie -h for options.\n")
	os.Exit(code)
}

func main() {
	serviceString := flag.String("s", "", "Limit search by service address (regexp)")
	tag := flag.String("t", "", "Limit search by tag")
	flag.Parse()

	// show usage if there are not command line args
	args := flag.Args()
	if len(args) == 0 {
		usage(0)
	}

	cmd := args[0]
	switch cmd {
	// define a couple synonyms to "hunt" as well
	case "hunt", "find", "search":
		serviceList := getList(*serviceString, *tag)
		printList(serviceList)

	case "kill":
		serviceList := getList(*serviceString, *tag)
		deregister(serviceList)

	default:
		usage(1)
	}

}

// display a list of matching services
func printList(serviceList []*api.ServiceEntry) {
	translate := map[bool]string{
		false: "-",
		true:  "+",
	}

	for _, se := range serviceList {
		healthy := isHealthy(se)
		fmt.Printf("%s %s: %s - healthy=%t\n", translate[healthy],
			se.Service.Service, se.Service.ID, healthy)
	}
}

// kill those services that are failing in the passed list
func deregister(serviceList []*api.ServiceEntry) {
	for _, se := range serviceList {
		if !isHealthy(se) {
			fullAddress := fmt.Sprintf("%s:%d", se.Node.Address, CONSUL_PORT)
			fmt.Printf("Deregistering %s: %s (%s)\n", se.Service.Service, se.Service.ID, fullAddress)
			client, err := getClient(fullAddress)
			if err != nil {
				log.Fatalf("Unable to get consul client: %s\n", err)
			}
			agent := client.Agent()
			err = agent.ServiceDeregister(se.Service.ID)
			if err != nil {
				log.Printf("Unable to deregister: %s\n", err)
			}
		}

	}
}
