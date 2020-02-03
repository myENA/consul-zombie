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

// this is the default port for talking to remote consul agents
const defaultPort = 8500

type verbosityLevel uint8

func (v verbosityLevel) is(other verbosityLevel) bool {
	return v == other
}

func (v verbosityLevel) allows(other verbosityLevel) bool {
	return v >= other
}

const (
	verbosityLevel0 verbosityLevel = iota
	verbosityLevel1
	verbosityLevel2
	verbosityLevel3
)

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
	port := fs.Int("port", defaultPort, "Port to use when connecting to remote agents")
	token := fs.String("token", "", "Token to use when connecting to remote agents")
	v1 := fs.Bool("v", false, "Verbose")
	v2 := fs.Bool("vv", false, "Increased Verbosity")
	v3 := fs.Bool("vvv", false, "Super Verbosity")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Printf("Error parsing args: %s", err)
		os.Exit(1)
	}

	var verbosity verbosityLevel
	if *v3 {
		verbosity = verbosityLevel3
	} else if *v2 {
		verbosity = verbosityLevel2
	} else if *v1 {
		verbosity = verbosityLevel1
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
		serviceList := getList(*serviceString, *token, *tag)
		printList(serviceList, verbosity)

	case "kill":
		serviceList := getList(*serviceString, *token, *tag)
		deregister(serviceList, *port, *token, *force)

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
		case v.allows(verbosityLevel3), v.allows(verbosityLevel2), v.allows(verbosityLevel1):
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
func deregister(serviceList []*api.ServiceEntry, port int, token string, force bool) {
	for _, se := range serviceList {
		if !isHealthy(se) || force {
			fullAddress := fmt.Sprintf("%s:%d", se.Node.Address, port)
			fmt.Printf("Deregistering %s: %s (%s)\n", se.Service.Service, se.Service.ID, fullAddress)
			client, err := getClient(fullAddress, token)
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
