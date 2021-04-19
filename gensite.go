package main

import (
	"fmt"
	"io/fs"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var templateroot string = "./builders/"
var htmlroot string = "./html/"

type Str string

func (S Str) EndsWith(suffix string) bool {
	return strings.HasSuffix(string(S), suffix)
}

func (S Str) Join(slice []string) Str {
	return Str(strings.Join(slice, string(S)))
}

func (S Str) Split(by string) []string {
	return strings.Split(string(S), by)
}

func (S Str) Replace(before, after string) Str {
	return Str(strings.Replace(string(S), before, after, -1))
}

func (S Str) Str() string {
	return string(S)
}

var ConvFiles map[string]string = make(map[string]string)

func walkerfunc(fpath string, info fs.FileInfo, err error) error {

	fname := info.Name()
	if Str(fname).EndsWith(".b.html") && !info.IsDir() {
		cont, err := os.ReadFile(fpath)
		if err == nil {
			ConvFiles[fpath] = string(cont)
		}
	}
	return nil
}

func transform(filePath, fileData, temp string, tlates map[string]string, rch chan int) {
	dat := Str(fileData).Split("\n")
	if dat[0] == "0" {
		title := dat[1]
		content := Str("\n\t\t\t").Join(dat[2:]).Str()
		nextemp := Str(temp)
		// Add templatedata files
		for k, v := range tlates {
			ev := Str("\n\t\t\t").Join(Str(v).Split("\n")).Str()
			nextemp = nextemp.Replace("<TEMPLATE>["+k+"]</TEMPLATE>", ev)
		}
		// Add title and content
		nextemp = nextemp.Replace("<TEMPLATE>[HTML-TITLE]</TEMPLATE>", title).Replace("<TEMPLATE>[HTML-CONTENT]</TEMPLATE>", content)
		newfile, ferr := os.Create(filePath[:len(filePath)-6] + "html")
		defer newfile.Close()
		if ferr != nil {
			log.Fatal(ferr)
		}
		_, werr := newfile.Write([]byte(nextemp))
		if werr != nil {
			log.Fatal(werr)
		}
	}
	// Tell the main thread we are done
	rch <- 0
}

func main() {

	templates := make(map[string]string)
	tfiles, direrr := os.ReadDir(templateroot + "templatedata")

	if direrr != nil {
		log.Fatal(direrr)
	}

	for _, f := range tfiles {
		fname := f.Name()
		if Str(fname).EndsWith(".t.html") {
			cont, readerr := os.ReadFile(templateroot + "templatedata/" + fname)
			if readerr != nil {
				fmt.Println("Error reading file:", fname)
			} else {
				templates[fname[:len(fname)-7]] = string(cont)
			}
		}
	}

	walkerr := filepath.Walk(htmlroot, walkerfunc)

	if walkerr != nil {
		log.Fatal(walkerr)
	}

	templatefile, err := os.ReadFile(templateroot + "TEMPLATE.t.html")

	if err != nil {
		log.Fatal(err)
	}

	rawstart := string(templatefile)

	rch := make(chan int)

	for path, data := range ConvFiles {
		go transform(path, data, rawstart, templates, rch)
	}
	for range ConvFiles {
		<-rch
	}
}
