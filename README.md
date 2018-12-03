# consul-zombie
Find and kill consul zombies.

If you fail to deregister consul services and health checks it can be nice to
have a tool to prune the dead from the living. This is especially true if you 
are deploying in a container environment where every service gets a fresh
and unique ID.

Install to your `$GOPATH` by cloning the repo and running `./build.sh`

There are three invocations of this tool:

Command                 | Description
------------------------|------------
`zombie`                | Get a little help
`zombie [opts] hunt` | List services that match your search terms, dead or alive
`zombie [opts] kill` | Repeat the search from the hunt above but kill those services that fail at least one health check

Available options:

Option      | Description
------------|------------
`-f`        | Force killing of all matches, including healthy services
`-s string` | Limit search by service address (regexp)
`-t string` | Limit search by tag
`-v`        | Verbose
`-vv`       | Increased verbosity
`-vvv`      | Super verbosity
