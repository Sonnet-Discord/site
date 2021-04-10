package main

import (
	"fmt"
	"io/fs"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strings"
)

var templateroot string = "./builders/"
var htmlroot string = "./html/"

func endswith(data, end string) bool {
	if len(end) > len(data) {
		return false
	}
	return data[len(data)-len(end):] == end
}
var ConvFiles map[string]string = make(map[string]string)

func walkerfunc(fpath string, info fs.FileInfo, err error) error {

	fname := info.Name()
	if endswith(fname, ".b.html") && !info.IsDir() {
		cont, err := ioutil.ReadFile(fpath)
		if err == nil {
			ConvFiles[fpath] = string(cont)
		}
	}
	return nil
}

func transform(filePath, fileData, nextemp string, tlates map[string]string, rch chan int) {
	dat := strings.Split(fileData, "\n")
	if dat[0] == "0" {
		title := dat[1]
		content := strings.Join(dat[2:], "\n\t\t\t")
		// Add templatedata files
		for k, v := range tlates {
			ev := strings.Join(strings.Split(v, "\n"), "\n\t\t\t")
			nextemp = strings.Replace(nextemp, "<TEMPLATE>["+k+"]</TEMPLATE>", ev, 1)
		}
		// Add title and content
		nextemp = strings.Replace(nextemp, "<TEMPLATE>[HTML-TITLE]</TEMPLATE>", title, 1)
		nextemp = strings.Replace(nextemp, "<TEMPLATE>[HTML-CONTENT]</TEMPLATE>", content, 1)
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
	tfiles, direrr := ioutil.ReadDir(templateroot + "templatedata")

	if direrr != nil {
		log.Fatal(direrr)
	}

	for _, f := range tfiles {
		fname := f.Name()
		if endswith(fname, ".t.html") {
			cont, readerr := ioutil.ReadFile(templateroot + "templatedata/" + fname)
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

	templatefile, err := ioutil.ReadFile(templateroot + "TEMPLATE.t.html")

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
