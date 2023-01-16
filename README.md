# fedifeeder
My attempt at 'relaying' traffic from a relay-less mastodon instance. fedifeeder scrapes the public timeline of an instance and follow users that show up there, with the intention of creating a more 'organic' federated timeline on a personal Activitypub server instance which doesn't currently support relays.

The concept around this can be found here: https://github.com/hachyderm/community/issues/32

# USAGE
fedifeeder is a service that requires the env vars defined in env.sh.example to be configured. Once those are set you should just need to execute the binary.

a simple status is available at: localhost:8080/healthz

It also supports any value under the DEBUG env var to get more output and make the /debug endpoint available for information on the current uses and IDs fedifeeder knows about.

to access that data hit: localhost:8080/debug when DEBUG mode is enabled.

# TODO
* figure out if I actually need a TOKEN to read the public timeline using go-mastodon
* support a config file
* Move to using streams
* Not blow up the internet and make everyone hate me whenever I turn this on
