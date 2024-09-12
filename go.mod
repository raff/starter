module github.com/raff/starter

go 1.23.1

require (
	github.com/BurntSushi/toml v1.4.0
	github.com/gobs/pretty v0.0.0-20180724170744-09732c25a95b
	github.com/kontera-technologies/go-supervisor v0.0.0-20200419122805-0ca2cb2a0d85
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d
)

require (
	github.com/mattn/go-colorable v0.1.13 // indirect
	github.com/mattn/go-isatty v0.0.16 // indirect
	golang.org/x/sys v0.0.0-20220811171246-fbc7d0a398ab // indirect
)

replace github.com/kontera-technologies/go-supervisor => github.com/raff/go-supervisor v0.0.0-20220614192355-5f8d38b6d27a
