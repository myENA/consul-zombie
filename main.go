// list and offer to kill dead consul services
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"sync"
	"time"

	"github.com/hashicorp/consul/api"
	"github.com/olekukonko/tablewriter"
)

// this is the default port for talking to remote consul agents
const (
	defaultPort = 8500
)

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

var (
	clientMap = new(sync.Map)
)

func usage(code int) {
	fmt.Println("usage: zombie [options] (hunt|kill|search)")
	fmt.Println("Search (hunt) or deregister (kill) services: zombie -h for options.")
	os.Exit(code)
}

func main() {
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	serviceString := fs.String("s", "", "Limit search by service address (regexp)")
	tag := fs.String("t", "", "Limit search by tag")
	force := fs.Bool("f", false, "Force killing of all matches, including healthy services")
	localAddr := fs.String("local-addr", os.Getenv(api.HTTPAddrEnvName), "Address with port of \"local\" agent.  Used to list services.")
	remotePort := fs.Int("remote-port", defaultPort, "Port to use when connecting to remote agents")
	token := fs.String("token", os.Getenv(api.HTTPTokenEnvName), "ACL token.  Used in all api queries.")
	rate := fs.Int("rate", 0, "Per-minute rate of deregistration calls.  0 means no enforced limit, calls will be executed as fast as possible.")
	v1 := fs.Bool("v", false, "Verbose")
	v2 := fs.Bool("vv", false, "Really verbose")
	v3 := fs.Bool("vvv", false, "Extremely verbose")
	if err := fs.Parse(os.Args[1:]); err != nil {
		log.Printf("Error parsing args: %s", err)
		os.Exit(1)
	}

	if *rate < 0 {
		log.Printf("rate must be >= 0")
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
		serviceList := getList(*localAddr, *token, *serviceString, *tag)
		printList(serviceList, verbosity)

	case "kill":
		serviceList := getList(*localAddr, *token, *serviceString, *tag)
		deregister(*remotePort, *token, serviceList, *force, *rate)

	default:
		usage(1)
	}

}

// get a client handle for a specified address (or the local agent if "")
func getClient(address, token string) (*api.Client, error) {
	config := api.DefaultNonPooledConfig()
	if address != "" {
		config.Address = address
	}
	if token != "" {
		config.Token = token
	}
	key := fmt.Sprintf("%s%s", address, token)
	if client, ok := clientMap.Load(key); ok {
		return client.(*api.Client), nil
	} else if client, err := api.NewClient(config); err != nil {
		return nil, err
	} else {
		clientMap.Store(key, client)
		return client, nil
	}
}

// get a list of all services, limit to those matching the search criteria
func getList(localAddr, token, serviceString, tag string) []*api.ServiceEntry {
	client, err := getClient(localAddr, token)
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
		var (
			serviceID string
			services  map[string]*api.ServiceEntry
			service   *api.ServiceEntry
		)
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
		healthy = healthy && (c.Status == api.HealthPassing)
		eligible++
	}

	// No eligible checks were found
	if eligible == 0 {
		return false
	}

	return healthy
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
func deregister(remotePort int, token string, serviceList []*api.ServiceEntry, force bool, rate int) {
	var (
		delay time.Duration
		timer *time.Timer
	)
	if rate > 0 {
		delay = time.Duration((float64(60) / float64(rate)) * float64(time.Second))
		timer = time.NewTimer(delay)
	}
	for _, se := range serviceList {
		if !isHealthy(se) || force {
			fullAddress := fmt.Sprintf("%s:%d", se.Node.Address, remotePort)
			log.Printf("Deregistering %s: %s (%s)\n", se.Service.Service, se.Service.ID, fullAddress)
			client, err := getClient(fullAddress, token)
			if err != nil {
				log.Fatalf("Unable to get consul client: %s\n", err)
			}
			agent := client.Agent()
			err = agent.ServiceDeregister(se.Service.ID)
			if err != nil {
				log.Printf("Unable to deregister: %s\n", err)
			}
			if rate > 0 {
				<-timer.C
				timer.Reset(delay)
			}
		}
	}
}
