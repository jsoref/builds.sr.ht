package main

type Manifest struct {
	Arch         *string
	Environment  map[string]interface{}
	Image        string
	Packages     []string
	Repositories map[string]string
	Secrets      []string
	Shell        bool
	Sources      []string
	Tasks        []map[string]string
	Triggers     []map[string]interface{}
}
