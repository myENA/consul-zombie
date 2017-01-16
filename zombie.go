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
	fmt.Println("usage: zombie [options] (hunt|kill|search)")
	fmt.Println("Search (hunt) or deregister (kill) services: zombie -h for options.")
	os.Exit(code)
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	serviceString := fs.String("s", "", "Limit search by service address (regexp)")
	tag := fs.String("t", "", "Limit search by tag")
	force := fs.Bool("f", false, "Force killing of all matches, including healthy services")
	fs.Parse(os.Args[1:])

	// show usage if there are not command line args
	args := fs.Args()
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
		deregister(serviceList, *force)

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

// kill those services that are failing in the passed list, or all if force is true
func deregister(serviceList []*api.ServiceEntry, force bool) {
	for _, se := range serviceList {
		if !isHealthy(se) || force {
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
