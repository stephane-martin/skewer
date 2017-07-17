# Skewer, a Syslog to Kafka gateway

Skewer is a syslog server that forwards the messages to a Kafka cluster.


## Status


Pretty much experimental. Not much tested. Probably some bugs. Don't be afraid
to post bugs and ask questions.


## Features


-   Listens on TCP, UDP or RELP
-   Can fetch log messages from Journald on Linux
-   Can fetch audit logs from the kernel on Linux
-   Configuration can be provided as a configuration file, or fetched from Consul
-   Dynamic configuration with Consul
-   Can register the TCP and RELP listeners as services in Consul
-   Custom message parsers and filters can be defined through Javascript functions
-   Works on Linux and MacOS (not tested on *BSD), does not work on Windows


## How it works


-   skewer implements the standard syslog protocols for TCP and UDP.

    When receiving a message on TCP and UDP, there is no way to notify the
    emitter that the message has been correctly received. To avoid message
    loss, such messages are first stored in an embedded database (the Store),
    that's persisted on disk. The messages are removed from the Store only
    after Kafka has acknowledged them. When some message fails to be transfered
    to Kafka, skewer keeps it and retries later.

-   skewer implements the RELP (aka reliable syslog) protocol that was defined
    by rsyslog. 

    With RELP, we can inform the emitter when a message has been correctly
    received by Kafka. The emitter is responsible to keep the message as long
    as we don't notify him. So in this case, there is no Store mechanism
    involved.

-   skewer uses a Netlink connection to fetch audit logs from the Linux Kernel

-   skewer uses the C Journald API to fetch messages from Journald.


## Building


Nothing special. Dependencies are vendored. Uses `cgo` at least on Linux.

You can `go get github.com/stephane-martin/skewer` or `git clone ...` and `go build`.


## Configuration


The configuration file directory location is specified by the `--config`
command line flag (by default `/etc`). Inside that directory, the configuration
file must be called `skewer.toml`.

See the example file `skewer.example.toml` in source root directory for the
various options.

You can also specify a Consul server through the command line flags. In that case,
the configuration will be fetched from Consul. When the configuration changes in
Consul, the services will be restarted accordingly (only the Store configuration
is not dynamic).

The configuration must be given as a tree in Consul KV. For example:

> skewer/syslog/A/bind_addr 0.0.0.0
> skewer/syslog/A/port 2514
> 
> skewer/syslog/B/bind_addr 0.0.0.0
> skewer/syslog/A/port 3514
> 
> skewer/kafka/brokers ["kafka1","kafka2","kafka2"]


## Commands


Flags are documented in the command line help.

-   `skewer default-config`

    Prints a default configuration in TOML syntax.

-   `skewer print-config [--config dirname]`

    Print the current configuration

-   `skewer serve [--config dirname] [--test]`

    The main command. Creates the services specified in the configuration,
    listens for new messages on the network services, fetch audit logs and
    journald messages, forward everything to kafka.

    You can provide a `--test` command line flag to print the collected
    messages instead of sending them to Kafka.

-   `skewer make-secret`

    Generates a secret that you can use in the Store configuration, so that
    log messages are not written in clear text on disk.



