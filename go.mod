module github.com/psanford/hat

go 1.21

toolchain go1.22.4

require (
	github.com/Azure/go-ansiterm v0.0.0-20210617225240-d185dfc1b5a1
	github.com/google/go-cmp v0.4.0
	github.com/vito/midterm v0.1.5-0.20240307214207-d0271a7ca452
	golang.org/x/sys v0.7.0
)

require (
	github.com/aymanbagabas/go-osc52/v2 v2.0.1 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-isatty v0.0.18 // indirect
	github.com/mattn/go-runewidth v0.0.14 // indirect
	github.com/muesli/termenv v0.15.1 // indirect
	github.com/rivo/uniseg v0.4.4 // indirect
)

replace github.com/Azure/go-ansiterm => github.com/psanford/go-ansiterm v0.0.0-20240729051413-7b84f1edf5ba
