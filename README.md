# consul-zombie
Find and kill consul zombies.

If you fail to deregister consul services and health checks it can be nice to
have a tool to prune the dead from the living. This is especially true if you 
are deploying in a container environment where every service gets a fresh
and unique ID.

Install to your `$GOPATH` by cloning the repo and running `./build.sh`

There are three invocations of this tool:

`zombie` - get a little help

`zombie [options] hunt` - list services that match your search terms, dead or alive.

`zombie [options] kill` - repeat the search from the hunt above but kill those services that fail at least one health check.

The options are:
	
`-v` enable verbose mode

`-t <tag>` limit the search to services that match the tag exactly.

`-s <regexp>` limit the search to service names that match the specified regexp.