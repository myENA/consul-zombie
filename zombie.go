// list and offer to kill dead consul services
package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/hashicorp/consul/api"
	"github.com/olekukonko/tablewriter"
)

type verbosityLevel uint8

func (v verbosityLevel) is(other verbosityLevel) bool {
	return v == other
}

func (v verbosityLevel) allows(other verbosityLevel) bool {
	return v >= other
}

const (
	V0 verbosityLevel = iota
	V1
	V2
	V3
)

func usage(code int) {
	fmt.Printf("usage: %s [options] (hunt|kill|search)\n", os.Args[0])
	fmt.Printf("Search (hunt) or deregister (kill) services: %s --help for options.\n", os.Args[0])
	os.Exit(code)
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ExitOnError)
	consulHost := fs.String("h", "127.0.0.1", "Consul host")
	consulPort := fs.Int("p", 8500, "Port for talking to remote consul agents")
	serviceString := fs.String("s", "", "Limit search by service address (regexp)")
	tag := fs.String("t", "", "Limit search by tag")
	force := fs.Bool("f", false, "Force killing of all matches, including healthy services")
	v1 := fs.Bool("v", false, "Verbose")
	v2 := fs.Bool("vv", false, "Increased Verbosity")
	v3 := fs.Bool("vvv", false, "Super Verbosity")
	fs.Parse(os.Args[1:])

	var verbosity verbosityLevel
	if *v3 {
		verbosity = V3
	} else if *v2 {
		verbosity = V2
	} else if *v1 {
		verbosity = V1
	}

	// show usage if there are not command line args
	args := fs.Args()
	if len(args) == 0 {
		usage(0)
	}

	cmd := args[0]
	switch cmd {
	// define a couple synonyms to "hunt" as well
	case "hunt", "find", "search":
		serviceList := getList(*serviceString, *tag, *consulHost, *consulPort)
		printList(serviceList, verbosity)

	case "kill":
		serviceList := getList(*serviceString, *tag, *consulHost, *consulPort)
		deregister(serviceList, *force, *consulPort)

	default:
		usage(1)
	}

}

// display a list of matching services
func printList(serviceList []*api.ServiceEntry, v verbosityLevel) {
	var header, footer []string
	var headerLen int

	table := tablewriter.NewWriter(os.Stdout)

	header = []string{"node", "id", "name", "address", "state"}
	headerLen = len(header)
	footer = make([]string, headerLen)

	table.SetHeader(header)

	healthy := 0
	unhealthy := 0

	for _, se := range serviceList {
		isHealthy := isHealthy(se)

		if isHealthy {
			healthy++
		} else {
			unhealthy++
		}

		switch true {
		case v.allows(V3), v.allows(V2), v.allows(V1):
			table.Append([]string{
				se.Node.Node,
				se.Service.ID,
				se.Service.Service,
				fmt.Sprintf("%s:%d", se.Service.Address, se.Service.Port),
				fmt.Sprintf("healthy=%t", isHealthy),
			})

		default:
			if !isHealthy {
				table.Append([]string{
					se.Node.Node,
					se.Service.ID,
					se.Service.Service,
					fmt.Sprintf("%s:%d", se.Service.Address, se.Service.Port),
					fmt.Sprintf("healthy=%t", isHealthy),
				})
			}
		}
	}

	footer[0] = "summary"
	footer[headerLen-1] = fmt.Sprintf("total: %d", healthy+unhealthy)
	footer[headerLen-2] = fmt.Sprintf("healthy: %d", healthy)
	footer[headerLen-3] = fmt.Sprintf("unhealthy: %d", unhealthy)
	table.SetFooter(footer)

	table.Render()
}

// kill those services that are failing in the passed list, or all if force is true
func deregister(serviceList []*api.ServiceEntry, force bool, consulPort int) {
	for _, se := range serviceList {
		if !isHealthy(se) || force {
			fullAddress := fmt.Sprintf("%s:%d", se.Node.Address, consulPort)
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
