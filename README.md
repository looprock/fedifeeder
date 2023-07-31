# fedifeeder
My attempt at 'relaying' traffic from a relay-less mastodon instance. fedifeeder scrapes the public timeline of an instance and follow users that show up there, with the intention of creating a more 'organic' federated timeline on a personal Activitypub server instance which doesn't currently support relays (in my case GoToSocial).

The concept around this can be found here: https://github.com/hachyderm/community/issues/32

# USAGE
fedifeeder is a service that requires the env vars defined in env.sh.example to be configured. Once those are set you should just need to execute the binary.

To use fedifeeder, both the source and target servers must allow API access.

Currently fedifeeder only accepts parameters via environment variables. See: `env.sh.example` for an example of what to set.

a simple status is available at: localhost:8080/healthz

Setting the DEBUG env var to any value will make the /debug endpoint available for information on the current uses and IDs fedifeeder knows about. To access that data hit: localhost:8080/debug when DEBUG mode is enabled.

# How I used fedifeeder

I chose to be up-front about what I was doing and set up a `fedifeeder` account on my local instance with description of the process with a link back to this repository. I then ran fedifeeder for about 30 minutes to populate that user's friends list with anyone who posted to the public feed of an instance I liked the "consistency" of. This allowed me to get federated data, while still not pushing a bunch of random people to my personal account on the instance. After that, my federated timeline started populating with posts from the users who posted over that time and normal federated data started showing up.

Please note:
1) a friend request was made through the regular channels, which could be, and sometimes was, rejected
2) the #nobot tag was honored).

# TODO
* figure out if I actually need a TOKEN to read the public timeline using go-mastodon
* support a config file
* Move to using streams
