Generate the code for a go 1.24 application that should run as a small daemon, exposing a RESTful API that is authenticated via one or multiple configured API keys or via a local socket. It should also provide an admin API which allows the configuration of keylightd accessible primarily over a unix socket. It should support discovery of keylights via mDNS with a configurable discovery interval, the registration of discovered lists as individual lights, the grouping of those lights together and the management of API keys, many of which should be a subcommand of a config command.  The service should be called keylightd, and the module called github.com/jmylchreest/keylightd. 

the git tag, commit id and build date should be stored as the version and goreleaser uses ldflags to set them in the application, which would be visible by calling the version command. A debug message should be printed by both keylightd and keylightctl on startup.

Log level should be configurable via a -v flag, which increases the log level the more times it's specified.

Please make sure suitable debug and info logging exists. Be concise and show any necessary variables. Make sure it prefers the socket connection which would be in the standard user directories. 

Start by making a well structured, well tested and robust package which would be reusable for other projects. This package should provide mdns discovery, discovered light caching, exposing commands for those lights. Once thats done, use it to create the initial keylightd service. We will make the keylightctl command afterwards.