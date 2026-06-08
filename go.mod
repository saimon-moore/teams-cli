module github.com/fossteams/teams-cli

go 1.26.1

require (
	github.com/dgrijalva/jwt-go v3.2.0+incompatible
	github.com/gdamore/tcell/v2 v2.5.1
	github.com/rivo/tview v0.0.0-20220307222120-9994674d60a8
	github.com/saimon-moore/teams-api v0.0.0
	github.com/sirupsen/logrus v1.8.1
	golang.org/x/net v0.38.0
)

replace github.com/saimon-moore/teams-api => ../teams-api

require (
	github.com/gdamore/encoding v1.0.0 // indirect
	github.com/lucasb-eyer/go-colorful v1.2.0 // indirect
	github.com/mattn/go-runewidth v0.0.13 // indirect
	github.com/rivo/uniseg v0.2.0 // indirect
	golang.org/x/sys v0.31.0 // indirect
	golang.org/x/term v0.30.0 // indirect
	golang.org/x/text v0.23.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
