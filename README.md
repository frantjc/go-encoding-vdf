# go-encoding-vdf [![CI](https://github.com/frantjc/go-encoding-vdf/actions/workflows/ci.yml/badge.svg?branch=main&event=push)](https://github.com/frantjc/go-encoding-vdf/actions) [![godoc](https://pkg.go.dev/badge/github.com/frantjc/go-encoding-vdf.svg)](https://pkg.go.dev/github.com/frantjc/go-encoding-vdf)

[Go](https://go.dev) module to encode and decode Valve Data Format with a similar API to the Go standard library's `encoding/json`.

## install

```sh
go get github.com/frantjc/go-encoding-vdf
```

## usage

```go
// appmanifest_10.acf
//
// "AppState"
// {
// 	"appid"		"10"
// 	"name"		"Counter-Strike"
// }
var manifest struct {
	AppState struct {
		AppID int    `vdf:"appid"`
		Name  string `vdf:"name"`
	}
}

if err := vdf.Unmarshal(b, &manifest); err != nil {
	return err
}

b, err := vdf.Marshal(&manifest)
```

Objects decode into structs, maps and `any`; strings decode into strings, numbers, bools and `encoding.TextUnmarshaler`s. Quoted and unquoted tokens, `\n` `\t` `\\` `\"` escapes and `//` comments are supported. Platform conditionals such as `[$WIN32]` are accepted but not evaluated.
